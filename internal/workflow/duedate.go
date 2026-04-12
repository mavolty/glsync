package workflow

import (
	"fmt"
	"strings"
	"time"
)

// DueDateStrategy controls how the due date is calculated for "In Progress" transitions.
const (
	StrategySprintEnd    = "sprint_end"
	StrategyStoryPoints  = "story_points"
)

// NextWeekday returns the next occurrence of the given weekday at or after today.
// If today is already that weekday, it returns today.
func NextWeekday(from time.Time, weekday time.Weekday) time.Time {
	from = from.Truncate(24 * time.Hour)
	daysUntil := int(weekday) - int(from.Weekday())
	if daysUntil < 0 {
		daysUntil += 7
	}
	return from.AddDate(0, 0, daysUntil)
}

// ParseWeekday converts a weekday name string (e.g. "tuesday") to time.Weekday.
func ParseWeekday(s string) (time.Weekday, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "sunday":
		return time.Sunday, nil
	case "monday":
		return time.Monday, nil
	case "tuesday":
		return time.Tuesday, nil
	case "wednesday":
		return time.Wednesday, nil
	case "thursday":
		return time.Thursday, nil
	case "friday":
		return time.Friday, nil
	case "saturday":
		return time.Saturday, nil
	default:
		return 0, fmt.Errorf("unknown weekday: %q", s)
	}
}

// CalculateDueDate returns the due date based on the configured strategy.
// storyPoints is only used when strategy is StrategyStoryPoints.
func CalculateDueDate(strategy string, sprintEndWeekday string, storyPoints float64, now time.Time) (time.Time, error) {
	switch strategy {
	case StrategySprintEnd:
		wd, err := ParseWeekday(sprintEndWeekday)
		if err != nil {
			return time.Time{}, err
		}
		return NextWeekday(now, wd), nil

	case StrategyStoryPoints:
		days := int(storyPoints)
		if days < 1 {
			days = 1
		}
		return now.AddDate(0, 0, days), nil

	default:
		return time.Time{}, fmt.Errorf("unknown due_date_strategy: %q", strategy)
	}
}
