package pkg

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

func Log(file *os.File, err error, id string, output bool) {
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	if output == true {
		log.Println("\n" + nowStr + ":" + "number:" + id + ":" + err.Error() + "\n")
	}
	if file != nil {
		file.WriteString("\n" + nowStr + ":" + "number:" + id + ":" + err.Error() + "\n")
	}
}

func LogWithString(file *os.File, str string, id string, output bool) {
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	if output == true {
		log.Println("\n" + nowStr + ":" + "number:" + id + ":" + str + "\n")
	}
	if file != nil {
		file.WriteString("\n" + nowStr + ":" + "number:" + id + ":" + str + "\n")
	}
}

func ReadLog(file *os.File, s time.Time, e time.Time) error {
	now := time.Now()
	if s.After(now) || e.After(now) {
		return errors.New("time range exceeds current time")
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":number:", 2)
		if len(parts) != 2 {
			continue
		}

		ts, err := time.Parse("2006-01-02 15:04:05", parts[0])
		if err != nil {
			continue
		}

		if (ts.Equal(s) || ts.After(s)) && (ts.Equal(e) || ts.Before(e)) {
			log.Println(line)
		}
	}

	return nil
}

func MuxLog(file *os.File, err error, id string, output bool, mux *sync.RWMutex) {
	mux.Lock()
	defer mux.Unlock()
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	if output == true {
		log.Println("\n" + nowStr + ":" + "number:" + id + ":" + err.Error() + "\n")
	}
	if file != nil {
		file.WriteString("\n" + nowStr + ":" + "number:" + id + ":" + err.Error() + "\n")
	}
}

func MuxLogWithString(file *os.File, str string, id string, output bool, mux *sync.RWMutex) {
	mux.Lock()
	defer mux.Unlock()
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	if output == true {
		log.Println("\n" + nowStr + ":" + "number:" + id + ":" + str + "\n")
	}
	if file != nil {
		file.WriteString("\n" + nowStr + ":" + "number:" + id + ":" + str + "\n")
	}
}

func MuxReadLog(file *os.File, s time.Time, e time.Time, mux *sync.RWMutex) error {
	mux.RLock()
	defer mux.RUnlock()
	now := time.Now()
	if s.After(now) || e.After(now) {
		return errors.New("time range exceeds current time")
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":number:", 2)
		if len(parts) != 2 {
			continue
		}

		ts, err := time.Parse("2006-01-02 15:04:05", parts[0])
		if err != nil {
			continue
		}

		if (ts.Equal(s) || ts.After(s)) && (ts.Equal(e) || ts.Before(e)) {
			log.Println(line)
		}
	}

	return nil
}
