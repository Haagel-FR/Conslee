package proxy

import (
	"testing"
	"time"

	"conslee/internal/config"
)

func TestParseHHMM(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"00:00", 0},
		{"23:59", 1439},
		{"08:00", 480},
		{"12:30", 750},
		{"invalid", 0},
		{"25:00", 0},
		{"12:60", 0},
		{"-1:30", 0},
		{"12", 0},
	}
	for _, tt := range tests {
		got := parseHHMM(tt.input)
		if got != tt.want {
			t.Errorf("parseHHMM(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseSchedule(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := ParseSchedule(nil, "on_demand"); got != nil {
			t.Errorf("ParseSchedule(nil) = %v, want nil", got)
		}
	})

	t.Run("mode mapping", func(t *testing.T) {
		sc := &config.ScheduleConfig{}
		cases := map[string]ScheduleMode{
			"on_demand":     ModeOnDemand,
			"schedule_only": ModeScheduleOnly,
			"both":          ModeBoth,
			"unknown":       ModeOnDemand,
			"":              ModeOnDemand,
		}
		for mode, want := range cases {
			got := ParseSchedule(sc, mode)
			if got.Mode != want {
				t.Errorf("ParseSchedule mode %q = %v, want %v", mode, got.Mode, want)
			}
		}
	})

	t.Run("day parsing", func(t *testing.T) {
		sc := &config.ScheduleConfig{Days: []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}}
		s := ParseSchedule(sc, "on_demand")
		if len(s.Days) != 7 {
			t.Errorf("expected 7 days, got %d", len(s.Days))
		}
		if !s.Days[time.Monday] || !s.Days[time.Sunday] {
			t.Errorf("monday or sunday not set")
		}
	})

	t.Run("start/stop parsing", func(t *testing.T) {
		sc := &config.ScheduleConfig{Start: "08:00", Stop: "17:00"}
		s := ParseSchedule(sc, "on_demand")
		if s.StartMinutes != 480 {
			t.Errorf("StartMinutes = %d, want 480", s.StartMinutes)
		}
		if s.StopMinutes != 1020 {
			t.Errorf("StopMinutes = %d, want 1020", s.StopMinutes)
		}
	})
}

func TestShouldBeUp(t *testing.T) {
	t.Run("no schedule returns false", func(t *testing.T) {
		s := &ServiceState{}
		if s.ShouldBeUp(time.Now()) {
			t.Error("ShouldBeUp with nil schedule should return false")
		}
	})

	t.Run("day filter", func(t *testing.T) {
		s := &ServiceState{
			Schedule: &ServiceSchedule{
				Mode:         ModeScheduleOnly,
				Days:         map[time.Weekday]bool{time.Monday: true},
				StartMinutes: 0, StopMinutes: 0,
			},
		}
		mon := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
		if !s.ShouldBeUp(mon) {
			t.Error("ShouldBeUp on Monday should be true")
		}
		tue := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
		if s.ShouldBeUp(tue) {
			t.Error("ShouldBeUp on Tuesday should be false")
		}
	})

	t.Run("time window normal", func(t *testing.T) {
		s := &ServiceState{
			Schedule: &ServiceSchedule{
				Mode:         ModeScheduleOnly,
				Days:         map[time.Weekday]bool{},
				StartMinutes: 480, StopMinutes: 1020,
			},
		}
		at0800 := time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)
		at1200 := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
		at1700 := time.Date(2026, 9, 21, 17, 0, 0, 0, time.UTC)
		at1800 := time.Date(2026, 9, 21, 18, 0, 0, 0, time.UTC)

		if !s.ShouldBeUp(at0800) {
			t.Error("08:00 should be up")
		}
		if !s.ShouldBeUp(at1200) {
			t.Error("12:00 should be up")
		}
		if s.ShouldBeUp(at1700) {
			t.Error("17:00 should be down (exclusive stop)")
		}
		if s.ShouldBeUp(at1800) {
			t.Error("18:00 should be down")
		}
	})

	t.Run("time window wrap-around", func(t *testing.T) {
		s := &ServiceState{
			Schedule: &ServiceSchedule{
				Mode:         ModeScheduleOnly,
				Days:         map[time.Weekday]bool{},
				StartMinutes: 1320, StopMinutes: 120,
			},
		}
		at2300 := time.Date(2026, 9, 21, 23, 0, 0, 0, time.UTC)
		at0100 := time.Date(2026, 9, 22, 1, 0, 0, 0, time.UTC)
		at0300 := time.Date(2026, 9, 22, 3, 0, 0, 0, time.UTC)

		if !s.ShouldBeUp(at2300) {
			t.Error("23:00 should be up")
		}
		if !s.ShouldBeUp(at0100) {
			t.Error("01:00 should be up")
		}
		if s.ShouldBeUp(at0300) {
			t.Error("03:00 should be down")
		}
	})

	t.Run("start==stop means always up", func(t *testing.T) {
		s := &ServiceState{
			Schedule: &ServiceSchedule{
				Mode:         ModeScheduleOnly,
				Days:         map[time.Weekday]bool{},
				StartMinutes: 0, StopMinutes: 0,
			},
		}
		if !s.ShouldBeUp(time.Now()) {
			t.Error("start==stop should always be up")
		}
	})
}

func TestModeString(t *testing.T) {
	tests := []struct {
		mode ScheduleMode
		want string
	}{
		{ModeOnDemand, "on_demand"},
		{ModeScheduleOnly, "schedule_only"},
		{ModeBoth, "both"},
		{ScheduleMode("unknown"), "on_demand"},
	}
	for _, tt := range tests {
		s := &ServiceSchedule{Mode: tt.mode}
		if got := s.ModeString(); got != tt.want {
			t.Errorf("ModeString(%q) = %q, want %q", tt.mode, got, tt.want)
		}
	}
}
