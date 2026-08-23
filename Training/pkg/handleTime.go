package pkg

import (
	"fmt"
	"strings"
	"time"
)

func HandleTime(t string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02-15:04:05", t)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse time %q: %w", t, err)
	}

	if parsed.After(time.Now()) {
		return time.Time{}, fmt.Errorf("time %q exceeds current time", t)
	}

	return parsed, nil
}

func ActiveTime() []string {
	now := time.Now()
	res := make([]string, 5)
	for i := 0; i < 5; i++ {
		t := now.AddDate(0, -i, 0)
		res[i] = t.Format("2006-01")
	}
	return res
}

func GetTime(s string) time.Time {
	dateString := strings.TrimSuffix(s, ".db")
	date, _ := time.Parse("2006-01-02 15:04:05", dateString)
	return date
}
