package cron

import (
	"testing"
	"time"
)

func TestIsValidCron(t *testing.T) {
	valid := []string{
		"* * * * *",
		"0 9 * * *",
		"*/15 * * * *",
		"0,30 * * * *",
		"0 9-17 * * *",
		"0 9 * * mon-fri",
		"0 9 * * MON,WED,FRI",
		"0 0 1 jan *",
		"0 9-17/2 * * *",
		"59 23 31 12 6",
	}
	for _, e := range valid {
		if !isValidCron(e) {
			t.Errorf("isValidCron(%q) = false, want true (%v)", e, validateCron(e))
		}
	}

	// Every one of these was accepted by the old field-counting check and then
	// silently never fired.
	invalid := []string{
		"",
		"* * * *",
		"* * * * * *",
		"0-30 abc * * *",
		"60 * * * *",
		"* 24 * * *",
		"* * 0 * *",
		"* * 32 * *",
		"* * * 13 *",
		"* * * * 7",
		"*/0 * * * *",
		"*/-1 * * * *",
		"30-10 * * * *",
		"0 9 * * funday",
		"0,, * * * *",
	}
	for _, e := range invalid {
		if isValidCron(e) {
			t.Errorf("isValidCron(%q) = true, want false", e)
		}
	}
}

func TestMatchesCron(t *testing.T) {
	// 2026-08-24 09:30 is a Monday.
	monday0930 := time.Date(2026, 8, 24, 9, 30, 0, 0, time.UTC)

	cases := []struct {
		expr string
		want bool
	}{
		{"* * * * *", true},
		{"30 9 * * *", true},
		{"30 9 24 8 1", true},
		{"29 9 * * *", false},
		{"30 10 * * *", false},
		{"*/15 * * * *", true},
		{"*/7 * * * *", false},
		{"0,30 * * * *", true},
		{"0,15,45 * * * *", false},
		{"30 9-17 * * *", true},
		{"30 10-17 * * *", false},
		{"30 9 * * mon-fri", true},
		{"30 9 * * sat,sun", false},
		{"30 9 * aug *", true},
		{"30 9 * sep *", false},
	}
	for _, c := range cases {
		if got := matchesCron(c.expr, monday0930); got != c.want {
			t.Errorf("matchesCron(%q, Mon 09:30 24 Aug) = %v, want %v", c.expr, got, c.want)
		}
	}
}

// crontab(5) counts steps from the start of the range, so "*/2" on a 1-based
// field is the 1st, 3rd, 5th. The old val%step check produced the even days.
func TestStepCountsFromRangeStart(t *testing.T) {
	set, err := parseCronField("*/2", 1, 31, nil)
	if err != nil {
		t.Fatalf("parseCronField: %v", err)
	}
	for _, day := range []int{1, 3, 5, 31} {
		if !set[day] {
			t.Errorf("day %d should match */2 on a 1-based field", day)
		}
	}
	for _, day := range []int{2, 4, 30} {
		if set[day] {
			t.Errorf("day %d should not match */2 on a 1-based field", day)
		}
	}
}

func TestParseCronFieldRanges(t *testing.T) {
	set, err := parseCronField("9-17/4", 0, 23, nil)
	if err != nil {
		t.Fatalf("parseCronField: %v", err)
	}
	want := map[int]bool{9: true, 13: true, 17: true}
	for h := 0; h <= 23; h++ {
		if set[h] != want[h] {
			t.Errorf("hour %d: got %v, want %v", h, set[h], want[h])
		}
	}
}
