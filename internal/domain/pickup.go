package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/changerr/dishflow-grok/internal/apperr"
)

type StoreHours struct {
	Timezone              string
	BusinessHours         string
	ScheduledEnabled      bool
	AdvanceDays           int
	SlotMinutes           int
	SlotCapacity          int
	MinLeadMinutes        int
	PickupMinutes         int
}

type PickupSlot struct {
	StartsAt  time.Time `json:"starts_at"`
	Label     string    `json:"label"`
	Capacity  int       `json:"capacity"`
	Remaining int       `json:"remaining"`
	Available bool      `json:"available"`
}

func ParseHours(spec string) (startMin, endMin int, err error) {
	parts := strings.Split(strings.TrimSpace(spec), "-")
	if len(parts) != 2 {
		return 0, 0, apperr.Validation("营业时间必须为 HH:mm-HH:mm")
	}
	parse := func(s string) (int, error) {
		hm := strings.Split(s, ":")
		if len(hm) != 2 {
			return 0, apperr.Validation("营业时间格式无效")
		}
		h, err1 := strconv.Atoi(hm[0])
		m, err2 := strconv.Atoi(hm[1])
		if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
			return 0, apperr.Validation("营业时间格式无效")
		}
		return h*60 + m, nil
	}
	startMin, err = parse(parts[0])
	if err != nil {
		return 0, 0, err
	}
	endMin, err = parse(parts[1])
	if err != nil {
		return 0, 0, err
	}
	if endMin <= startMin {
		return 0, 0, apperr.Validation("营业结束时间必须晚于开始时间，且不支持跨午夜")
	}
	return startMin, endMin, nil
}

func LoadLocation(tz string) (*time.Location, error) {
	if tz == "" {
		tz = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, apperr.Validation("时区无效")
	}
	return loc, nil
}

func BusinessDate(now time.Time, tz string) (string, error) {
	loc, err := LoadLocation(tz)
	if err != nil {
		return "", err
	}
	return now.In(loc).Format("2006-01-02"), nil
}

func GenerateSlots(cfg StoreHours, date string, nowUTC time.Time, reserved map[string]int) ([]PickupSlot, error) {
	if !cfg.ScheduledEnabled {
		return []PickupSlot{}, nil
	}
	if cfg.AdvanceDays < 1 || cfg.AdvanceDays > 30 {
		return nil, apperr.Validation("可预约天数必须为 1～30")
	}
	if cfg.SlotMinutes < 5 || cfg.SlotMinutes > 120 {
		return nil, apperr.Validation("时段间隔必须为 5～120 分钟")
	}
	if cfg.SlotCapacity < 1 {
		return nil, apperr.Validation("每时段容量至少为 1")
	}
	if cfg.MinLeadMinutes < 0 || cfg.MinLeadMinutes > 1440 {
		return nil, apperr.Validation("最少提前分钟数必须为 0～1440")
	}
	loc, err := LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, err
	}
	day, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return nil, apperr.Validation("日期格式无效")
	}
	today := nowUTC.In(loc)
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)
	last := todayDate.AddDate(0, 0, cfg.AdvanceDays-1)
	if day.Before(todayDate) || day.After(last) {
		return []PickupSlot{}, nil
	}
	startMin, endMin, err := ParseHours(cfg.BusinessHours)
	if err != nil {
		return nil, err
	}
	cutoff := nowUTC.In(loc).Add(time.Duration(cfg.MinLeadMinutes) * time.Minute)
	out := []PickupSlot{}
	for m := startMin; m < endMin; m += cfg.SlotMinutes {
		h := m / 60
		mm := m % 60
		start := time.Date(day.Year(), day.Month(), day.Day(), h, mm, 0, 0, loc)
		key := start.UTC().Format(time.RFC3339)
		used := reserved[key]
		remaining := cfg.SlotCapacity - used
		if remaining < 0 {
			remaining = 0
		}
		available := remaining > 0 && start.After(cutoff)
		out = append(out, PickupSlot{
			StartsAt:  start,
			Label:     start.Format("15:04"),
			Capacity:  cfg.SlotCapacity,
			Remaining: remaining,
			Available: available,
		})
	}
	return out, nil
}

func ValidateScheduledFor(cfg StoreHours, scheduled *time.Time, nowUTC time.Time, reserved int) error {
	if scheduled == nil {
		return apperr.PickupTimeInvalid
	}
	slots, err := GenerateSlots(cfg, scheduled.In(mustLoc(cfg.Timezone)).Format("2006-01-02"), nowUTC, map[string]int{
		scheduled.UTC().Format(time.RFC3339): reserved,
	})
	if err != nil {
		return err
	}
	want := scheduled.UTC().Unix()
	for _, s := range slots {
		if s.StartsAt.UTC().Unix() == want {
			if !s.Available {
				if s.Remaining <= 0 {
					return apperr.PickupSlotFull
				}
				return apperr.PickupTimeInvalid
			}
			return nil
		}
	}
	return apperr.PickupTimeInvalid
}

func mustLoc(tz string) *time.Location {
	loc, err := LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

func AlignRFC3339(t time.Time) string {
	return t.Format(time.RFC3339)
}

func SlotKey(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func FormatPickupLabel(t time.Time, loc *time.Location) string {
	local := t.In(loc)
	return fmt.Sprintf("%s %s", local.Format("1月2日"), local.Format("15:04"))
}
