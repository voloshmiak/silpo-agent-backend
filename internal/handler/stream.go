package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/voloshmiak/silpo-agent-backend/internal/middleware"
	"github.com/voloshmiak/silpo-agent-backend/internal/silpo"
	"github.com/voloshmiak/silpo-agent-backend/internal/storage"
)

type StreamHandler struct {
	plans        *storage.PlanRepo
	tokens       *storage.TokenRepo
	users        *storage.UserRepo
	settings     *storage.SettingsRepo
	feedbacks    *storage.FeedbackRepo
	progress     *storage.ProgressRepo
	silpoSvc     *silpo.Service
	coreAgentURL string
	serviceToken string
	httpClient   *http.Client
}

func NewStreamHandler(
	plans *storage.PlanRepo,
	tokens *storage.TokenRepo,
	users *storage.UserRepo,
	settings *storage.SettingsRepo,
	feedbacks *storage.FeedbackRepo,
	progress *storage.ProgressRepo,
	silpoSvc *silpo.Service,
	coreAgentURL string,
	serviceToken string,
) *StreamHandler {
	return &StreamHandler{
		plans:        plans,
		tokens:       tokens,
		users:        users,
		settings:     settings,
		feedbacks:    feedbacks,
		progress:     progress,
		silpoSvc:     silpoSvc,
		coreAgentURL: coreAgentURL,
		serviceToken: serviceToken,
		httpClient:   newStreamClient(),
	}
}

// newStreamClient builds the client that talks to the core agent's SSE endpoint.
//
// It deliberately has no http.Client.Timeout: that deadline covers reading the
// response body, so on a streaming response it kills the stream mid-run — a
// plan that takes longer than the deadline was cut off at exactly that mark,
// and the browser saw the connection close with no final event. A plan run
// legitimately takes many minutes, so the phases that must not hang are
// bounded individually instead, and the body read is left unbounded.
func newStreamClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
			ExpectContinueTimeout: time.Second,
			IdleConnTimeout:       90 * time.Second,
		},
	}
}

// GET /plan/stream — SSE proxy to the core agent.
// Requires: Authorization: Bearer <our-jwt>
//
// The profile, the budget, the training regime and the food restrictions are
// read from user_settings and cannot be overridden per request: a screen with a
// stale copy of the profile used to be able to plan a week against numbers the
// user never saved, and half the profile came from the DB anyway, so the two
// halves could disagree.
//
// Query params are only what no row holds: note (the user's free text), fridge
// (comma-separated), plan_id (previous plan to adapt), apply (write to cart).
func (h *StreamHandler) Stream(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	// Ensure user exists.
	_, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user not found"})
	}

	// Load user settings: the profile, budget, training regime and restrictions
	// all come from here, so a failure to read them is a failure of the run.
	// Planning a week on fallback numbers would produce a plausible plan for
	// somebody else.
	userSettings, err := h.settings.GetByUserID(c.Context(), userID)
	if err != nil || userSettings == nil {
		log.Printf("[ERROR] failed to load settings for user %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load user settings",
		})
	}

	log.Printf("[INFO] user %s settings loaded: budget=%.0f UAH, focus=%q, weight=%.1f→%.1f, workouts=%d, updated_at=%s",
		userID, userSettings.WeeklyBudget, userSettings.Focus,
		userSettings.Weight, userSettings.TargetWeight, userSettings.WorkoutsPerWeek,
		userSettings.UpdatedAt.Format(time.RFC3339))

	silpoToken, err := h.silpoSvc.EnsureValid(c.Context(), h.tokens, userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("silpo token unavailable: %v", err),
		})
	}

	// Free text from the user, and nothing else: the restrictions used to be
	// glued in here as Ukrainian prose, and now travel as diet_type, allergens
	// and excluded_products, which the core can actually act on.
	note := strings.TrimSpace(c.Query("note", ""))

	// Cart writes are on by default: the agent fills the user's Silpo cart with
	// what it picked, while checkout and payment stay with the user in Silpo.
	// ?apply=false keeps a run read-only.
	apply := c.QueryBool("apply", true)

	fridgeItems := []string{}
	if fridge := strings.TrimSpace(c.Query("fridge", "")); fridge != "" {
		for _, item := range strings.Split(fridge, ",") {
			if t := strings.TrimSpace(item); t != "" {
				fridgeItems = append(fridgeItems, t)
			}
		}
	}

	// Load previous plan if plan_id provided.
	var previousPlan *interface{}
	if prevID := strings.TrimSpace(c.Query("plan_id", "")); prevID != "" {
		if pid, parseErr := uuid.Parse(prevID); parseErr == nil {
			if prev, loadErr := h.plans.GetByID(c.Context(), pid, userID); loadErr == nil {
				var stored map[string]interface{}
				if json.Unmarshal([]byte(prev.Content), &stored) == nil {
					// What we persist is a wrapper — answer, plan_data, raw_text —
					// and raw_text is the whole SSE transcript of that run. Sending
					// the wrapper upstream shipped tens of thousands of tokens the
					// core cannot read and left it with no usable history at all.
					// Only plan_data is a Plan.
					planObj, ok := stored["plan_data"]
					if !ok {
						planObj = interface{}(stored)
					}
					previousPlan = &planObj
					log.Printf("[INFO] previous plan %s loaded for user %s (unwrapped=%t)", pid, userID, ok)
				}
			}
		}
	}

	// Load latest user feedback if available (to adapt next plan based on dish ratings & tags).
	latestFeedback, _ := h.feedbacks.GetLatestByUserID(c.Context(), userID)

	// Send request to core agent.
	bodyBytes, err := json.Marshal(buildCoreRequest(userSettings, silpoToken, note, fridgeItems, previousPlan, apply, latestFeedback))
	if err != nil {
		log.Printf("[ERROR] failed to build upstream body for user %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to build upstream request"})
	}

	upstream := h.coreAgentURL + "/plan/stream"
	// Cancelling this context aborts the upstream run. It is cancelled when the
	// browser goes away, so a closed tab stops the agent instead of leaving it
	// planning (and billing) into a connection nobody is reading.
	streamCtx, cancelStream := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(streamCtx, http.MethodPost, upstream, bytes.NewReader(bodyBytes))
	if err != nil {
		cancelStream()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to build upstream request"})
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+h.serviceToken)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		cancelStream()
		log.Printf("[ERROR] upstream request failed for user %s: %v", userID, err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": fmt.Sprintf("upstream failed: %v", err)})
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancelStream()
		log.Printf("[ERROR] upstream returned %d for user %s: %s", resp.StatusCode, userID, string(body))
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   fmt.Sprintf("upstream returned %d", resp.StatusCode),
			"details": string(body),
		})
	}

	// Hijack the TCP connection — write SSE directly, bypassing fasthttp buffering.
	// HijackSetNoResponse(true) tells fasthttp NOT to send any response on its own.
	c.Context().HijackSetNoResponse(true)
	c.Context().Hijack(func(conn net.Conn) {
		defer conn.Close()
		defer resp.Body.Close()
		defer cancelStream()

		// Write HTTP response headers directly.
		w := bufio.NewWriterSize(conn, 4096)
		fmt.Fprintf(w, "HTTP/1.1 200 OK\r\n")
		fmt.Fprintf(w, "Content-Type: text/event-stream\r\n")
		fmt.Fprintf(w, "Cache-Control: no-cache\r\n")
		fmt.Fprintf(w, "Connection: keep-alive\r\n")
		fmt.Fprintf(w, "X-Accel-Buffering: no\r\n")
		fmt.Fprintf(w, "Access-Control-Allow-Origin: *\r\n")
		fmt.Fprintf(w, "\r\n")
		w.Flush()

		// Stream SSE lines from core agent to client.
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 512*1024), 512*1024)

		var (
			currentEvent string
			planData     string // structured plan JSON from "plan" event
			planTitle    string
			tokenBuf     strings.Builder // full agent text response from "token" events
			allEvents    strings.Builder // raw log of all SSE events
		)

		planSaved := false
		savePlanFunc := func() {
			if planSaved {
				return
			}

			// Build plan title from the first ~80 chars of streamed text.
			if tokenBuf.Len() > 0 {
				planTitle = tokenBuf.String()
				if len(planTitle) > 80 {
					planTitle = planTitle[:80] + "…"
				}
			}
			if planTitle == "" {
				planTitle = "Plan " + time.Now().Format("2006-01-02 15:04")
			}

			var contentObj struct {
				Answer  string      `json:"answer"`
				Plan    interface{} `json:"plan_data,omitempty"`
				RawText string      `json:"raw_text,omitempty"`
			}
			ans := strings.TrimSpace(tokenBuf.String())
			if ans == "" {
				ans = strings.TrimSpace(allEvents.String())
			}
			contentObj.Answer = ans
			contentObj.RawText = allEvents.String()
			if planData != "" {
				var pd map[string]interface{}
				if json.Unmarshal([]byte(planData), &pd) == nil {
					if innerPlan, ok := pd["plan"]; ok {
						contentObj.Plan = innerPlan
					} else {
						contentObj.Plan = pd
					}
				}
			}

			content, err := json.Marshal(contentObj)
			if err != nil || len(content) < 5 {
				content = []byte(allEvents.String())
			}

			planSaved = true
			saveCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			log.Printf("[INFO] saving plan for user %s: title=%q, bytes=%d", userID, planTitle, len(content))
			p, err := h.plans.Create(saveCtx, userID, planTitle, string(content))
			if err != nil {
				log.Printf("[ERROR] failed to create plan in DB for user %s: %v", userID, err)
			} else {
				log.Printf("[INFO] plan created successfully: id=%s for user %s", p.ID, userID)

				if h.progress != nil {
					cost := calculatePlanCost(contentObj.Plan, userSettings.WeeklyBudget)
					count, _ := h.progress.CountExpenses(saveCtx, userID)
					weekNum := count + 1
					weekLabel := fmt.Sprintf("Т%d", weekNum)
					_, expErr := h.progress.RecordExpense(saveCtx, &storage.ExpenseRecord{
						UserID:      userID,
						PlanID:      &p.ID,
						WeekNumber:  weekNum,
						WeekLabel:   weekLabel,
						TotalCost:   cost,
						BudgetLimit: userSettings.WeeklyBudget,
						RecordedAt:  time.Now(),
					})
					if expErr != nil {
						log.Printf("[WARN] failed to record weekly expense for user %s: %v", userID, expErr)
					} else {
						log.Printf("[INFO] recorded weekly expense for user %s: week=%s cost=%.2f limit=%.2f",
							userID, weekLabel, cost, userSettings.WeeklyBudget)
					}
				}
			}
		}

		// The browser must always learn how the run ended. Anything that leaves
		// this loop without a plan or an error event reaches the client as a
		// connection that simply stopped, which it can only report as "the
		// stream ended without a plan" — the least useful message possible.
		sawError := false
		writeError := func(message string) {
			payload, err := json.Marshal(map[string]string{"type": "error", "message": message})
			if err != nil {
				return
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
			_ = w.Flush()
		}

		for scanner.Scan() {
			line := scanner.Text()

			// A failed write means the client is gone: stop reading upstream.
			if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
				log.Printf("[WARN] client went away mid-stream for user %s: %v", userID, err)
				return
			}
			if err := w.Flush(); err != nil {
				log.Printf("[WARN] client went away mid-stream for user %s: %v", userID, err)
				return
			}

			// Log all events for full persistence.
			allEvents.WriteString(line)
			allEvents.WriteString("\n")

			// Track SSE event type if explicitly sent.
			if strings.HasPrefix(line, "event:") {
				currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				continue
			}

			// Capture data.
			if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if data == "" {
					continue
				}

				var ev map[string]interface{}
				if err := json.Unmarshal([]byte(data), &ev); err == nil {
					eventType := currentEvent
					if t, ok := ev["type"].(string); ok && t != "" {
						eventType = t
					}

					switch eventType {
					case "plan":
						planData = data
						if ansStr, ok := ev["answer"].(string); ok && ansStr != "" {
							tokenBuf.Reset()
							tokenBuf.WriteString(ansStr)
						}
						// Save immediately as soon as plan event arrives!
						savePlanFunc()
					case "token":
						if txt, ok := ev["text"].(string); ok {
							tokenBuf.WriteString(txt)
						} else if d, ok := ev["data"].(string); ok {
							tokenBuf.WriteString(d)
						}
					case "error":
						sawError = true
						tokenBuf.WriteString("[ERROR] ")
						if msg, ok := ev["message"].(string); ok {
							log.Printf("[ERROR] agent reported a failure for user %s (run %v): %s",
								userID, ev["run_id"], msg)
							tokenBuf.WriteString(msg)
						} else {
							tokenBuf.WriteString(data)
						}
						tokenBuf.WriteString("\n")
					}
				} else {
					tokenBuf.WriteString(data)
				}
			}
		}

		switch {
		case scanner.Err() != nil:
			// A read error here is the upstream connection breaking under us:
			// a cut stream, a dropped connection, a cancelled context.
			log.Printf("[ERROR] upstream stream broke for user %s: %v", userID, scanner.Err())
			writeError("Зв'язок з агентом обірвався. Спробуйте згенерувати план ще раз.")
		case planData == "" && !sawError:
			log.Printf("[ERROR] upstream closed with no plan and no error for user %s", userID)
			writeError("Агент завершив роботу без плану. Спробуйте ще раз.")
		case planData != "":
			savePlanFunc()
		}
	})

	return nil
}

func calculatePlanCost(planObj interface{}, fallbackBudget float64) float64 {
	if planObj == nil {
		return fallbackBudget
	}
	m, ok := planObj.(map[string]interface{})
	if !ok {
		return fallbackBudget
	}
	if items, ok := m["cart_items"].([]interface{}); ok && len(items) > 0 {
		var sum float64
		for _, it := range items {
			if itemMap, ok := it.(map[string]interface{}); ok {
				var price float64
				switch p := itemMap["price"].(type) {
				case float64:
					price = p
				case int:
					price = float64(p)
				}
				qty := 1.0
				switch q := itemMap["quantity"].(type) {
				case float64:
					qty = q
				case int:
					qty = float64(q)
				}
				sum += price * qty
			}
		}
		if sum > 0 {
			return math.Round(sum*100) / 100
		}
	}
	if b, ok := m["budget_uah"].(float64); ok && b > 0 {
		return b
	}
	return fallbackBudget
}
