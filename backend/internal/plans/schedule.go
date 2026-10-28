package plans

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"app/internal/errs"
)

// Weekday is a day of the week as the API and plans.schedules spell it.
type Weekday string

const (
	Monday    Weekday = "mon"
	Tuesday   Weekday = "tue"
	Wednesday Weekday = "wed"
	Thursday  Weekday = "thu"
	Friday    Weekday = "fri"
	Saturday  Weekday = "sat"
	Sunday    Weekday = "sun"
)

// Weekdays is every day, in the order a schedule lists them.
var Weekdays = []Weekday{Monday, Tuesday, Wednesday, Thursday, Friday, Saturday, Sunday}

var weekdayOf = map[time.Weekday]Weekday{
	time.Monday: Monday, time.Tuesday: Tuesday, time.Wednesday: Wednesday, time.Thursday: Thursday,
	time.Friday: Friday, time.Saturday: Saturday, time.Sunday: Sunday,
}

// TimeOfDay is minutes after midnight UTC, 0 to 1439. It is "HH:MM" in JSON.
type TimeOfDay int

func ParseTimeOfDay(s string) (TimeOfDay, bool) {
	t, err := time.Parse("15:04", s)
	if err != nil || len(s) != len("15:04") {
		return 0, false
	}
	return TimeOfDay(t.Hour()*60 + t.Minute()), true
}

func (t TimeOfDay) String() string { return fmt.Sprintf("%02d:%02d", int(t)/60, int(t)%60) }

func (t TimeOfDay) MarshalJSON() ([]byte, error) { return json.Marshal(t.String()) }

func (t *TimeOfDay) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, ok := ParseTimeOfDay(s)
	if !ok {
		return fmt.Errorf("invalid time of day %q", s)
	}
	*t = parsed
	return nil
}

// Schedule is one line of a plan's timetable: a time of day in UTC on some weekdays. No days
// means every day. Days are stored in Weekdays order, and all seven are stored as none, so two
// schedules for the same slots are equal.
type Schedule struct {
	Time TimeOfDay `json:"time" swaggertype:"string"`
	Days []Weekday `json:"days" enums:"mon,tue,wed,thu,fri,sat,sun"`
}

func (s Schedule) runsOn(day time.Weekday) bool {
	return len(s.Days) == 0 || slices.Contains(s.Days, weekdayOf[day])
}

// NextRun is the first time after `after` that one of the schedules is due, in UTC. False when
// there are no schedules: the plan only runs when started by hand.
func NextRun(schedules []Schedule, after time.Time) (time.Time, bool) {
	after = after.UTC()
	midnight := time.Date(after.Year(), after.Month(), after.Day(), 0, 0, 0, 0, time.UTC)
	var next time.Time
	// Eight days: today's slot may have passed, and a schedule on one weekday comes back a week later.
	for offset := 0; offset <= 7; offset++ {
		day := midnight.AddDate(0, 0, offset)
		for _, s := range schedules {
			at := day.Add(time.Duration(s.Time) * time.Minute)
			if s.runsOn(day.Weekday()) && at.After(after) && (next.IsZero() || at.Before(next)) {
				next = at
			}
		}
		if !next.IsZero() {
			return next, true
		}
	}
	return time.Time{}, false
}

// PrevRun is the latest time at or before `at` that one of the schedules was due, in UTC. False
// when there are no schedules. The scheduler runs this slot when it has not run it yet.
func PrevRun(schedules []Schedule, at time.Time) (time.Time, bool) {
	at = at.UTC()
	midnight := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
	var prev time.Time
	for offset := 0; offset <= 7; offset++ {
		day := midnight.AddDate(0, 0, -offset)
		for _, s := range schedules {
			slot := day.Add(time.Duration(s.Time) * time.Minute)
			if s.runsOn(day.Weekday()) && !slot.After(at) && slot.After(prev) {
				prev = slot
			}
		}
		if !prev.IsZero() {
			return prev, true
		}
	}
	return time.Time{}, false
}

// parseSchedules checks a request's schedules and puts them in their stored form. Failures are
// keyed by position: "schedules.1.time".
func parseSchedules(inputs []ScheduleInput) ([]Schedule, error) {
	fields := errs.Fields{}
	schedules := make([]Schedule, 0, len(inputs))
	seen := map[string]bool{}
	for i, in := range inputs {
		path := fmt.Sprintf("schedules.%d", i)
		at, ok := ParseTimeOfDay(strings.TrimSpace(in.Time))
		if !ok {
			fields.Add(path+".time", "invalid")
		}
		days, ok := parseDays(in.Days)
		if !ok {
			fields.Add(path+".days", "invalid")
		}
		if len(fields) > 0 {
			continue
		}

		schedule := Schedule{Time: at, Days: days}
		key := fmt.Sprint(schedule)
		if seen[key] {
			fields[path] = append(fields[path], errs.ErrKeyPlansDuplicateSchedule)
		}
		seen[key] = true
		schedules = append(schedules, schedule)
	}
	if len(fields) > 0 {
		return nil, fields
	}
	return schedules, nil
}

func parseDays(input []string) ([]Weekday, bool) {
	chosen := map[Weekday]bool{}
	for _, day := range input {
		if !slices.Contains(Weekdays, Weekday(day)) {
			return nil, false
		}
		chosen[Weekday(day)] = true
	}
	days := []Weekday{}
	if len(chosen) == len(Weekdays) {
		return days, true
	}
	for _, day := range Weekdays {
		if chosen[day] {
			days = append(days, day)
		}
	}
	return days, true
}
