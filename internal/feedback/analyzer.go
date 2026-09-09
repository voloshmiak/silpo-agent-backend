package feedback

import (
	"fmt"
	"strings"

	"github.com/voloshmiak/silpo-agent-backend/internal/storage"
)

// Analyze generates decisions and a summary based on dish ratings, tags, and user settings.
func Analyze(
	dishRatings []storage.DishRating,
	tags []string,
	settings *storage.UserSettings,
) (summary string, decisions []storage.DecisionItem) {
	decisions = make([]storage.DecisionItem, 0)

	// 1. Пошук страв з оцінкою "bad" -> "ВИЛУЧЕНО"
	for _, dish := range dishRatings {
		if dish.Rating == "bad" {
			dishName := strings.TrimSpace(dish.Title)
			if dishName == "" {
				dishName = "Ця страва"
			}
			decisions = append(decisions, storage.DecisionItem{
				Badge:      "ВИЛУЧЕНО",
				BadgeColor: "bg-[#FF5C00] text-white",
				Text:       fmt.Sprintf("%s прибрано з раціону на наступні 3 тижні", dishName),
			})
		}
	}

	// 2. Обробка тегів зауважень
	hasTooHard := false
	hasWashDishes := false
	hasLowProtein := false
	hasTiredChicken := false
	hasNotEnoughTime := false
	hasWantSnacks := false

	for _, tag := range tags {
		t := strings.ToLower(strings.TrimSpace(tag))
		switch {
		case strings.Contains(t, "складно готувати"):
			hasTooHard = true
		case strings.Contains(t, "мити посуду"):
			hasWashDishes = true
		case strings.Contains(t, "мало білка"):
			hasLowProtein = true
		case strings.Contains(t, "курка"):
			hasTiredChicken = true
		case strings.Contains(t, "не встигав"):
			hasNotEnoughTime = true
		case strings.Contains(t, "перекус"):
			hasWantSnacks = true
		}
	}

	if hasTooHard || hasNotEnoughTime || hasWashDishes {
		decisions = append(decisions, storage.DecisionItem{
			Badge:      "СПРОЩЕНО",
			BadgeColor: "bg-[#D2F832] text-black border border-black/20",
			Text:       "Середній час приготування обідів зменшено з 35 хв до 20 хв",
		})
		decisions = append(decisions, storage.DecisionItem{
			Badge:      "СІЛЬПО",
			BadgeColor: "bg-[#DFDACB] text-zinc-800",
			Text:       "Автозаміна на напівфабрикати власного виробництва «Сільпо» (котлети з індички)",
		})
	}

	if hasLowProtein {
		decisions = append(decisions, storage.DecisionItem{
			Badge:      "БІЛОК",
			BadgeColor: "bg-[#D2F832] text-black border border-black/20",
			Text:       "Частку білка збільшено на +15% за рахунок кисломолочного сиру та тунця",
		})
	}

	if hasTiredChicken {
		decisions = append(decisions, storage.DecisionItem{
			Badge:      "ВИЛУЧЕНО",
			BadgeColor: "bg-[#FF5C00] text-white",
			Text:       "Куряче філе тимчасово замінено на філе індички, яловичину та рибу",
		})
	}

	if hasWantSnacks {
		decisions = append(decisions, storage.DecisionItem{
			Badge:      "СНЕКИ",
			BadgeColor: "bg-[#DFDACB] text-zinc-800",
			Text:       "Додано 2 легких щоденних перекуси (горіхово-фруктові мікси та смузі)",
		})
	}

	// 3. Категорія "КАЛОРІЇ" відповідно до фокусу/ваги
	caloriesText := "Планка збережена: 1 780 ккал/день (дефіцит підтверджено)"
	if settings != nil {
		if strings.Contains(strings.ToLower(settings.Focus), "набір") {
			caloriesText = "Калорійність скоригована під набір маси: +300 ккал/день"
		} else if strings.Contains(strings.ToLower(settings.Focus), "підтрим") {
			caloriesText = "Калорійність збалансована для підтримання поточної ваги"
		}
	}

	decisions = append(decisions, storage.DecisionItem{
		Badge:      "КАЛОРІЇ",
		BadgeColor: "bg-[#DFDACB] text-zinc-800",
		Text:       caloriesText,
	})

	// Формування підсумкового рядка summary
	if len(tags) > 0 {
		highlightedTag := tags[0]
		summary = fmt.Sprintf("На основі ваших оцінок та тегу «%s» алгоритм оновив параметри генерації:", highlightedTag)
	} else {
		summary = "На основі ваших щотижневих оцінок алгоритм оновив параметри генерації наступного тижня:"
	}

	return summary, decisions
}
