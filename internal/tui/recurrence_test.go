package tui

import (
	"testing"
)

func TestBuildRecurrenceRRule(t *testing.T) {
	tests := []struct {
		name      string
		inFreq    recurrenceFreq
		inCount   int
		wantRRule string
	}{
		{"None@0", recurNone, 0, ""},
		{"daily@5", recurDaily, 5, "RRULE:FREQ=DAILY;COUNT=5"},
		{"week@forever", recurWeekly, 0, "RRULE:FREQ=WEEKLY"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotRRule := buildRecurrenceRRule(tc.inFreq, tc.inCount)
			if gotRRule != tc.wantRRule {
				t.Errorf(
					"buildRecurrenceRRule(%v, %d) = %q expected %q",
					tc.inFreq,
					tc.inCount,
					gotRRule,
					tc.wantRRule,
				)
			}
		})
	}

}

func TestParseRecurrenceRRule(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantFreq  recurrenceFreq
		wantCount int
	}{
		{"daily@5", "RRULE:FREQ=DAILY;COUNT=5", recurDaily, 5},
		{"week@forever", "RRULE:FREQ=WEEKLY", recurWeekly, 0},
		{"trash", "RRULE:FREQ=ABOBA", recurNone, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotFreq, gotCount := parseRecurrenceRRule(tc.in)
			if gotFreq != tc.wantFreq || gotCount != tc.wantCount {
				t.Errorf("parseRecurrenceRRule(%q) = (%v, %d), want (%v, %d)",
					tc.in, gotFreq, gotCount, tc.wantFreq, tc.wantCount)
			}
		})
	}
}
