package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	feedback     *storage.FeedbackRepo
	settings     *storage.SettingsRepo
	silpoSvc     *silpo.Service
	coreAgentURL string
	serviceToken string
	httpClient   *http.Client
}

func NewStreamHandler(
	plans *storage.PlanRepo,
	tokens *storage.TokenRepo,
	users *storage.UserRepo,
	feedback *storage.FeedbackRepo,
	settings *storage.SettingsRepo,
	silpoSvc *silpo.Service,
	coreAgentURL string,
	serviceToken string,
) *StreamHandler {
	return &StreamHandler{
		plans:        plans,
		tokens:       tokens,
		users:        users,
		feedback:     feedback,
		settings:     settings,
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

func (h *StreamHandler) Stream(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	// Ensure user exists.
	_, err := h.users.GetByID(c.Context(), userID)
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

	// Load user settings
	allSettings, _ := h.settings.GetAll(c.Context(), userID)

	var (
		weeklyBudget     float64
		workoutsPerWeek  int = -1
		allergens        []string
		excludedProducts []string
		dietType         string
		focus            string
		userWeight       float64 = 70.0
		targetWeight     float64 = 70.0
		userHeight       float64 = 175.0
		userAge          int     = 25
		userSex          string  = "male"
	)

	// Парсим JSON payload для каждой категории
	for _, s := range allSettings {
		if s.IsDraft {
			continue // Используем только активные (примененные) настройки для генерации
		}
		switch s.Category {
		case storage.CategoryBudget:
			var p struct {
				WeeklyBudget float64 `json:"weekly_budget"`
			}
			if err := json.Unmarshal(s.Payload, &p); err == nil && p.WeeklyBudget > 0 {
				weeklyBudget = p.WeeklyBudget
			}
		case storage.CategorySport:
			var p struct {
				WorkoutsPerWeek int `json:"workouts_per_week"`
			}
			if err := json.Unmarshal(s.Payload, &p); err == nil {
				workoutsPerWeek = p.WorkoutsPerWeek
			}
		case storage.CategoryDiet:
			var p struct {
				Allergens        []string `json:"allergens"`
				ExcludedProducts []string `json:"excluded_products"`
				DietType         string   `json:"diet_type"`
			}
			if err := json.Unmarshal(s.Payload, &p); err == nil {
				allergens = p.Allergens
				excludedProducts = p.ExcludedProducts
				dietType = p.DietType
			}
		case storage.CategoryPhysical:
			var p struct {
				Weight       float64 `json:"weight"`
				TargetWeight float64 `json:"target_weight"`
				Height       float64 `json:"height"`
				Age          int     `json:"age"`
				Sex          string  `json:"sex"`
				Focus        string  `json:"focus"`
			}
			if err := json.Unmarshal(s.Payload, &p); err == nil {
				if p.Weight > 0 {
					userWeight = p.Weight
				}
				if p.TargetWeight > 0 {
					targetWeight = p.TargetWeight
				}
				if p.Height > 0 {
					userHeight = p.Height
				}
				if p.Age > 0 {
					userAge = p.Age
				}
				if p.Sex != "" {
					userSex = p.Sex
				}
				if p.Focus != "" {
					focus = p.Focus
				}
			}
		}
	}

	// Budget: query param or stored weekly_budget.
	budgetUAH := c.QueryFloat("budget_uah", 0)
	if budgetUAH <= 0 && weeklyBudget > 0 {
		budgetUAH = weeklyBudget
	}
	if budgetUAH <= 0 {
		budgetUAH = 1500.0 // sane default
	}

	// Workouts: query param or stored workouts_per_week.
	workouts := c.QueryInt("workouts", -1)
	if workouts < 0 {
		if workoutsPerWeek >= 0 {
			workouts = workoutsPerWeek
		} else {
			workouts = 3
		}
	}
	if workouts > 14 {
		workouts = 14
	}

	note := strings.TrimSpace(c.Query("note", ""))

	// Append allergens and excluded products from settings into note if present
	restrictions := []string{}
	if len(allergens) > 0 {
		restrictions = append(restrictions, "Алергени: "+strings.Join(allergens, ", "))
	}
	if len(excludedProducts) > 0 {
		restrictions = append(restrictions, "Виключити: "+strings.Join(excludedProducts, ", "))
	}
	if dietType != "" && dietType != "БЕЗ ОБМЕЖЕНЬ" {
		restrictions = append(restrictions, "Дієта: "+dietType)
	}
	if focus != "" {
		restrictions = append(restrictions, "Ціль: "+focus)
	}
	if len(restrictions) > 0 {
		combinedRestrictions := strings.Join(restrictions, "; ")
		if note != "" {
			note = note + " (" + combinedRestrictions + ")"
		} else {
			note = combinedRestrictions
		}
	}

	apply := c.QueryBool("apply", false)

	if note == "" {
		if prevID := strings.TrimSpace(c.Query("plan_id", "")); prevID != "" {
			if pid, parseErr := uuid.Parse(prevID); parseErr == nil {
				note = h.buildFeedbackNote(c.Context(), userID, pid)
			}
		}
	}

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
				var planObj interface{}
				if json.Unmarshal([]byte(prev.Content), &planObj) == nil {
					previousPlan = &planObj
				}
			}
		}
	}

	// Profile normalize
	s := strings.ToLower(userSex)
	if s == "male" || s == "чол." || s == "чоловіча" {
		userSex = "male"
	} else if s == "female" || s == "жін." || s == "жіноча" {
		userSex = "female"
	}
	if targetWeight == 0 {
		targetWeight = userWeight
	}

	// Query overrides
	if qWeight := c.QueryFloat("target_weight", 0); qWeight > 0 {
		targetWeight = qWeight
	}
	if qAge := c.QueryInt("age", 0); qAge >= 10 && qAge <= 120 {
		userAge = qAge
	}
	if qSex := strings.ToLower(strings.TrimSpace(c.Query("sex", ""))); qSex == "male" || qSex == "female" {
		userSex = qSex
	}

	profile := coreProfile{
		WeightKg:       userWeight,
		TargetWeightKg: targetWeight,
		HeightCm:       &userHeight,
		Age:            &userAge,
		Sex:            &userSex,
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

	c.Context().HijackSetNoResponse(true)
	c.Context().Hijack(func(conn net.Conn) {
		defer conn.Close()
		defer resp.Body.Close()

		w := bufio.NewWriterSize(conn, 4096)
		fmt.Fprintf(w, "HTTP/1.1 200 OK\r\n")
		fmt.Fprintf(w, "Content-Type: text/event-stream\r\n")
		fmt.Fprintf(w, "Cache-Control: no-cache\r\n")
		fmt.Fprintf(w, "Connection: keep-alive\r\n")
		fmt.Fprintf(w, "X-Accel-Buffering: no\r\n")
		fmt.Fprintf(w, "Access-Control-Allow-Origin: *\r\n")
		fmt.Fprintf(w, "\r\n")
		w.Flush()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 512*1024), 512*1024)

		var (
			currentEvent string
			planData     string
			planTitle    string
			tokenBuf     strings.Builder
			allEvents    strings.Builder
		)

		planSaved := false
		savePlanFunc := func() {
			if planSaved {
				return
			}

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
			}
		}

		for scanner.Scan() {
			line := scanner.Text()
			_, _ = fmt.Fprintf(w, "%s\n", line)
			_ = w.Flush()
			allEvents.WriteString(line)
			allEvents.WriteString("\n")

			if strings.HasPrefix(line, "event:") {
				currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				continue
			}

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
						savePlanFunc()
					case "token":
						if txt, ok := ev["text"].(string); ok {
							tokenBuf.WriteString(txt)
						} else if d, ok := ev["data"].(string); ok {
							tokenBuf.WriteString(d)
						}
					case "error":
						tokenBuf.WriteString("[ERROR] ")
						if msg, ok := ev["message"].(string); ok {
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
		savePlanFunc()
	})

	return nil
}

func (h *StreamHandler) buildFeedbackNote(ctx context.Context, userID, prevPlanID uuid.UUID) string {
	if h.feedback == nil {
		return ""
	}

	var parts []string
	tags, err := h.feedback.ListTags(ctx, userID, prevPlanID)
	if err == nil && len(tags) > 0 {
		names := make([]string, 0, len(tags))
		for _, t := range tags {
			names = append(names, t.Tag)
		}
		parts = append(parts, "Теги фідбека: "+strings.Join(names, ","))
	}

	ratings, err := h.feedback.ListDishRatings(ctx, userID, prevPlanID)
	if err != nil {
		var disliked []string
		for _, r := range ratings {
			if r.Rating == -1 {
				disliked = append(disliked, r.DishName)
			}
		}
		if len(disliked) > 0 {
			parts = append(parts, "Не сподобались страви:"+strings.Join(disliked, ","))
		}
	}

	return strings.Join(parts, " ")
}
