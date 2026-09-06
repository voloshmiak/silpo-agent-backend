package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	silpoSvc     *silpo.Service
	coreAgentURL string
	serviceToken string
	httpClient   *http.Client
}

func NewStreamHandler(
	plans *storage.PlanRepo,
	tokens *storage.TokenRepo,
	users *storage.UserRepo,
	silpoSvc *silpo.Service,
	coreAgentURL string,
	serviceToken string,
) *StreamHandler {
	return &StreamHandler{
		plans:        plans,
		tokens:       tokens,
		users:        users,
		silpoSvc:     silpoSvc,
		coreAgentURL: coreAgentURL,
		serviceToken: serviceToken,
		httpClient:   &http.Client{Timeout: 5 * time.Minute},
	}
}

// coreRequest mirrors the PlanRequest schema of the core agent.
type coreRequest struct {
	SilpoAccessToken string       `json:"silpo_access_token"`
	Profile          coreProfile  `json:"profile"`
	BudgetUAH        float64      `json:"budget_uah"`
	WorkoutsPerWeek  int          `json:"workouts_per_week"`
	FridgeItems      []string     `json:"fridge_items"`
	Note             string       `json:"note"`
	PreviousPlan     *interface{} `json:"previous_plan"`
	Apply            bool         `json:"apply"`
}

type coreProfile struct {
	WeightKg       float64  `json:"weight_kg"`
	TargetWeightKg float64  `json:"target_weight_kg"`
	HeightCm       *float64 `json:"height_cm"`
	Age            *int     `json:"age"`
	Sex            *string  `json:"sex"`
}

// GET /plan/stream — SSE proxy to the core agent.
// Requires: Authorization: Bearer <our-jwt>
// Query params: budget_uah (required), workouts, note, fridge, target_weight,
//
//	age, sex, silpo_token (override for testing), plan_id (previous plan)
func (h *StreamHandler) Stream(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	// Load user profile.
	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user not found"})
	}

	// Silpo token: query param overrides DB value (useful for testing).
	var silpoToken string
	if override := c.Query("silpo_token", ""); override != "" {
		silpoToken = override
	} else {
		silpoToken, err = h.silpoSvc.EnsureValid(c.Context(), h.tokens, userID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("silpo token unavailable: %v", err),
			})
		}
	}

	// Parse query params.
	budgetUAH := c.QueryFloat("budget_uah", 0)
	if budgetUAH <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "budget_uah is required and must be > 0"})
	}
	workouts := c.QueryInt("workouts", 0)
	note := c.Query("note", "")
	apply := c.QueryBool("apply", false)

	fridgeItems := []string{}
	if fridge := c.Query("fridge", ""); fridge != "" {
		for _, item := range strings.Split(fridge, ",") {
			if t := strings.TrimSpace(item); t != "" {
				fridgeItems = append(fridgeItems, t)
			}
		}
	}

	// Load previous plan if plan_id provided.
	var previousPlan *interface{}
	if prevID := c.Query("plan_id", ""); prevID != "" {
		if pid, parseErr := uuid.Parse(prevID); parseErr == nil {
			if prev, loadErr := h.plans.GetByID(c.Context(), pid, userID); loadErr == nil {
				var planObj interface{}
				if json.Unmarshal([]byte(prev.Content), &planObj) == nil {
					previousPlan = &planObj
				}
			}
		}
	}

	// Build profile.
	targetWeight := c.QueryFloat("target_weight", user.Weight)
	profile := coreProfile{WeightKg: user.Weight, TargetWeightKg: targetWeight}
	if user.Height > 0 {
		ht := user.Height
		profile.HeightCm = &ht
	}
	if age := c.QueryInt("age", 0); age > 0 {
		profile.Age = &age
	}
	if sex := c.Query("sex", ""); sex == "male" || sex == "female" {
		profile.Sex = &sex
	}

	// Send request to core agent.
	bodyBytes, _ := json.Marshal(coreRequest{
		SilpoAccessToken: silpoToken,
		Profile:          profile,
		BudgetUAH:        budgetUAH,
		WorkoutsPerWeek:  workouts,
		FridgeItems:      fridgeItems,
		Note:             note,
		PreviousPlan:     previousPlan,
		Apply:            apply,
	})

	upstream := h.coreAgentURL + "/plan/stream"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, upstream, bytes.NewReader(bodyBytes))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to build upstream request"})
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+h.serviceToken)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": fmt.Sprintf("upstream failed: %v", err)})
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
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
			planData     string
			planTitle    string
			tokenBuf     strings.Builder
		)

		for scanner.Scan() {
			line := scanner.Text()

			// Send line to frontend.
			fmt.Fprintf(w, "%s\n", line)
			w.Flush()

			// Track SSE event type.
			if strings.HasPrefix(line, "event:") {
				currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				continue
			}

			// Capture data by event type.
			if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				switch currentEvent {
				case "plan":
					planData = data
				case "token":
					var ev struct {
						Data string `json:"data"`
					}
					if json.Unmarshal([]byte(data), &ev) == nil {
						tokenBuf.WriteString(ev.Data)
					}
				}
			}
		}

		// Build plan title.
		if tokenBuf.Len() > 0 {
			planTitle = tokenBuf.String()
			if len(planTitle) > 80 {
				planTitle = planTitle[:80] + "…"
			}
		}
		if planTitle == "" {
			planTitle = "Plan " + time.Now().Format("2006-01-02 15:04")
		}

		content := planData
		if content == "" {
			content = "(no plan data captured)"
		}

		// Save plan to DB.
		_, _ = h.plans.Create(context.Background(), userID, planTitle, content)
	})

	return nil
}
