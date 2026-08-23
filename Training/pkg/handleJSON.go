package pkg

import (
	"fmt"
	"os"
	"strings"
	"train/back/selector"
)

func Analysis(filePath string) ([]selector.Criteria, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	result := make([]selector.Criteria, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			return nil, fmt.Errorf("invalid line format: %q", line)
		}
		result = append(result, selector.Criteria{
			Field:    parts[0],
			Operator: parts[1],
			Value:    strings.Join(parts[2:], " "),
		})
	}

	return result, nil
}
