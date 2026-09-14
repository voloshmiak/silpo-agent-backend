package handler

import (
	"errors"
	"strings"
	"time"

	"github.com/voloshmiak/silpo-agent-backend/internal/storage"
)

// parseWeekStart resolves ?week_start= into the Monday a plan is filed under.
//
// The week travels as an absolute date rather than "current"/"next": the page
// works the week out when it renders, and a tab opened on Sunday evening and
// submitted on Monday morning would ask for "next" relative to the wrong day
// and file the plan a week too late, as week 1. A date names the same week
// whenever it arrives, so all that is left to check is that the week can still
// be planned — this one or the next. Anything else is a stale page or a
// malformed request, and is refused rather than silently moved to another week.
//
// An empty value means the current week. The messages reach the user as is.
func parseWeekStart(raw string, now time.Time) (time.Time, error) {
	thisMonday := storage.WeekStart(now)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return thisMonday, nil
	}

	date, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, errors.New("week_start має бути датою у форматі YYYY-MM-DD")
	}
	if date.Weekday() != time.Monday {
		return time.Time{}, errors.New("week_start має бути понеділком")
	}
	if !date.Equal(thisMonday) && !date.Equal(thisMonday.AddDate(0, 0, 7)) {
		return time.Time{}, errors.New("Цей тиждень уже не можна спланувати — оновіть сторінку й спробуйте ще раз")
	}
	return date, nil
}
