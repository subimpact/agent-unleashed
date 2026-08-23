package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// cronFieldBounds are the inclusive ranges of the five crontab(5) fields:
// minute, hour, day-of-month, month, day-of-week.
var cronFieldBounds = [5][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 6}}

// cronFieldNames maps the three-letter month and weekday aliases onto their
// numeric values, so "0 9 * * mon-fri" behaves the way crontab(5) says it does.
var cronFieldNames = [5]map[string]int{
	nil, nil, nil,
	{"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
		"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12},
	{"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6},
}

// isValidCron reports whether every field of a 5-field expression parses. The
// previous version only counted fields, so "0-30 abc * * *" was accepted and
// then silently never fired.
func isValidCron(expr string) bool {
	return validateCron(expr) == nil
}

// validateCron returns the reason an expression is unusable, so callers can
// tell the user which field is wrong instead of just refusing.
func validateCron(expr string) error {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return fmt.Errorf("expected 5 fields (minute hour day-of-month month day-of-week), got %d", len(fields))
	}
	for i, f := range fields {
		if _, err := parseCronField(f, cronFieldBounds[i][0], cronFieldBounds[i][1], cronFieldNames[i]); err != nil {
			return fmt.Errorf("field %d (%q): %w", i+1, f, err)
		}
	}
	return nil
}

func matchesCron(expr string, t time.Time) bool {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return false
	}

	vals := [5]int{t.Minute(), t.Hour(), t.Day(), int(t.Month()), int(t.Weekday())}
	for i, f := range fields {
		set, err := parseCronField(f, cronFieldBounds[i][0], cronFieldBounds[i][1], cronFieldNames[i])
		if err != nil || !set[vals[i]] {
			return false
		}
	}
	return true
}

// parseCronField expands one field into the set of values it matches. It
// supports "*", "a", "a-b", "a,b,c", "*/n" and "a-b/n", plus month and weekday
// name aliases. Only "*", "*/n" and bare integers worked before.
func parseCronField(field string, min, max int, names map[string]int) (map[int]bool, error) {
	if strings.TrimSpace(field) == "" {
		return nil, fmt.Errorf("empty field")
	}

	out := make(map[int]bool)
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty list item")
		}

		step := 1
		if slash := strings.Index(part, "/"); slash >= 0 {
			parsed, err := strconv.Atoi(strings.TrimSpace(part[slash+1:]))
			if err != nil || parsed <= 0 {
				return nil, fmt.Errorf("invalid step %q", part[slash+1:])
			}
			step = parsed
			part = strings.TrimSpace(part[:slash])
		}

		lo, hi := min, max
		switch {
		case part == "*":
			// the whole range
		case strings.Contains(part, "-"):
			bits := strings.SplitN(part, "-", 2)
			var err error
			if lo, err = cronValue(bits[0], names); err != nil {
				return nil, err
			}
			if hi, err = cronValue(bits[1], names); err != nil {
				return nil, err
			}
		default:
			v, err := cronValue(part, names)
			if err != nil {
				return nil, err
			}
			lo, hi = v, v
		}

		if lo < min || hi > max || lo > hi {
			return nil, fmt.Errorf("value out of range, allowed %d-%d", min, max)
		}
		// Steps count from the start of the range, as crontab(5) specifies:
		// "*/2" on day-of-month is the 1st, 3rd, 5th - not the even days, which
		// is what the old val%step check produced.
		for v := lo; v <= hi; v += step {
			out[v] = true
		}
	}
	return out, nil
}

func cronValue(tok string, names map[string]int) (int, error) {
	tok = strings.TrimSpace(tok)
	if names != nil {
		if v, ok := names[strings.ToLower(tok)]; ok {
			return v, nil
		}
	}
	v, err := strconv.Atoi(tok)
	if err != nil {
		return 0, fmt.Errorf("invalid value %q", tok)
	}
	return v, nil
}
