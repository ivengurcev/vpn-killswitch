package traylog

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Logger struct {
	Path string
}

func (l Logger) Printf(format string, args ...any) {
	if l.Path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(l.Path), 0755); err != nil {
		return
	}
	f, err := os.OpenFile(l.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	line := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), line)
}
