package pkg

import (
	"strings"
)

func GetTimeString(s string) string {
	return s[:19]
}

func ReTime(data []byte) []string {
	s := strings.TrimSpace(string(data))
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
