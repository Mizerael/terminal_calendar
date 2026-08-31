package usecase

import (
	"testing"

	"github.com/Mizerael/terminal_calendar/internal/domain"
)

func TestReconcileDefaultsToAllCalendars(t *testing.T) {
	cals := []domain.Calendar{
		{ID: "work", Primary: true},
		{ID: "personal"},
	}
	s := &Selection{Enabled: map[string]bool{}, Target: ""}
	s.Reconcile(cals)
	if len(s.Enabled) != 2 || !s.Enabled["work"] || !s.Enabled["personal"] {
		t.Errorf("Enabled = %v, want both calendars on", s.Enabled)
	}
	if s.Target != "work" {
		t.Errorf("Target = %q, want work (primary)", s.Target)
	}
}

func TestReconcileDropsStaleAndKeepsTarget(t *testing.T) {
	cals := []domain.Calendar{{ID: "work", Primary: true}, {ID: "personal"}}
	s := &Selection{
		Enabled: map[string]bool{"work": true, "personal": false, "gone": true},
		Target:  "personal",
	}
	s.Reconcile(cals)
	if s.Enabled["gone"] {
		t.Error("stale calendar 'gone' should be dropped")
	}
	if !s.Enabled["work"] {
		t.Error("work should remain enabled")
	}
	if s.Target != "personal" {
		t.Errorf("Target = %q, want personal preserved", s.Target)
	}
}

func TestReconcileFallsBackInvalidTarget(t *testing.T) {
	cals := []domain.Calendar{{ID: "work", Primary: true}}
	s := &Selection{
		Enabled: map[string]bool{"work": true},
		Target:  "deleted-cal",
	}
	s.Reconcile(cals)
	if s.Target != "work" {
		t.Errorf("Target = %q, want work after invalid target", s.Target)
	}
}

func TestReconcileNilEnabled(t *testing.T) {
	s := &Selection{}
	s.Reconcile(nil)
	if s.Enabled == nil || len(s.Enabled) != 0 {
		t.Errorf("Reconcile(nil) = %v, want empty non-nil map", s.Enabled)
	}
	if s.Target != "" {
		t.Errorf("Target = %q, want empty when no calendars", s.Target)
	}
}

func TestPrimaryCalID(t *testing.T) {
	cals := []domain.Calendar{{ID: "personal"}, {ID: "work", Primary: true}}
	if got := PrimaryCalID(cals); got != "work" {
		t.Errorf("PrimaryCalID = %q, want work", got)
	}
	if got := PrimaryCalID([]domain.Calendar{{ID: "only"}}); got != "only" {
		t.Errorf("PrimaryCalID no-primary = %q, want only", got)
	}
	if got := PrimaryCalID(nil); got != "" {
		t.Errorf("PrimaryCalID(nil) = %q, want empty", got)
	}
}
