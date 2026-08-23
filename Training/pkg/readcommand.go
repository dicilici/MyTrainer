package pkg

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func ReadCommand() (string, []string) {
	fmt.Print("\n>> ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", nil
	}

	line = strings.TrimSpace(line)
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return "", nil
	}

	cmd := parts[0]
	args := make([]string, 0, len(parts)-1)
	for _, p := range parts[1:] {
		p = strings.TrimPrefix(p, "--")
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 {
			args = append(args, kv[1])
		} else {
			args = append(args, p)
		}
	}

	return cmd, args
}
