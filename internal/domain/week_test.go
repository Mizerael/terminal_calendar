package domain

import (
	"testing"
	"time"
)

func TestMondayOf(t *testing.T) {
	cases := []struct {
		day  time.Time
		want time.Time
	}{
		{time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)}, // Monday -> itself
		{time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)},  // Wednesday -> Monday
		{time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)},  // Sunday -> Monday
	}
	for _, c := range cases {
		if got := MondayOf(c.day); !got.Equal(c.want) {
			t.Errorf("MondayOf(%v) = %v, want %v", c.day, got, c.want)
		}
	}
}

func TestDayIndex(t *testing.T) {
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		e    Event
		want int
	}{
		{"monday", Event{Start: time.Date(2026, 8, 31, 9, 0, 0, 0, time.UTC)}, 0},
		{"tuesday", Event{Start: time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)}, 1},
		{"sunday", Event{Start: time.Date(2026, 9, 6, 9, 0, 0, 0, time.UTC)}, 6},
		{"before week", Event{Start: time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)}, -1},
		{"after week", Event{Start: time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)}, -1},
		{"zero start", Event{}, -1},
	}
	for _, c := range cases {
		if got := DayIndex(&c.e, mon); got != c.want {
			t.Errorf("%s: DayIndex = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestHourRange(t *testing.T) {
	// timed event 09:00-11:00
	e := Event{Start: time.Date(2026, 8, 31, 9, 0, 0, 0, time.UTC), End: time.Date(2026, 8, 31, 11, 0, 0, 0, time.UTC)}
	s, en, ok := HourRange(&e)
	if !ok || s != 9 || en != 11 {
		t.Errorf("hourRange = %d,%d,%v, want 9,11,true", s, en, ok)
	}

	// all-day event -> not ok
	ad := Event{AllDay: true, Start: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)}
	if _, _, ok := HourRange(&ad); ok {
		t.Error("all-day event should report ok=false")
	}

	// zero-length event -> at least one hour
	zl := Event{Start: time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC), End: time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC)}
	s, en, ok = HourRange(&zl)
	if !ok || s != 10 || en != 11 {
		t.Errorf("zero-length hourRange = %d,%d,%v, want 10,11,true", s, en, ok)
	}

	// short event ending mid-hour still fills its start hour
	sh := Event{Start: time.Date(2026, 8, 31, 9, 0, 0, 0, time.UTC), End: time.Date(2026, 8, 31, 9, 30, 0, 0, time.UTC)}
	s, en, ok = HourRange(&sh)
	if !ok || s != 9 || en != 10 {
		t.Errorf("short hourRange = %d,%d,%v, want 9,10,true", s, en, ok)
	}

	// event spanning midnight crosses into hour 24
	loc := time.Local
	cross := Event{Start: time.Date(2026, 9, 1, 23, 30, 0, 0, loc), End: time.Date(2026, 9, 2, 0, 30, 0, 0, loc)}
	s, en, ok = HourRange(&cross)
	if !ok || s != 23 || en != 24 {
		t.Errorf("crossing hourRange = %d,%d,%v, want 23,24,true", s, en, ok)
	}
}

func TestClampHour(t *testing.T) {
	low := -3
	ClampHour(&low)
	if low != 0 {
		t.Errorf("ClampHour(-3) = %d, want 0", low)
	}
	high := 30
	ClampHour(&high)
	if high != 23 {
		t.Errorf("ClampHour(30) = %d, want 23", high)
	}
	mid := 12
	ClampHour(&mid)
	if mid != 12 {
		t.Errorf("ClampHour(12) = %d, want 12", mid)
	}
}
