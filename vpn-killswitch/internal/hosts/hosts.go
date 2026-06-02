package hosts

import (
	"os"
	"strings"
)

const (
	DefaultBegin = "# vpn-killswitch managed block: begin"
	DefaultEnd   = "# vpn-killswitch managed block: end"
)

type Manager struct {
	Path  string
	Begin string
	End   string
}

func (m Manager) WithDefaults() Manager {
	if m.Begin == "" {
		m.Begin = DefaultBegin
	}
	if m.End == "" {
		m.End = DefaultEnd
	}
	return m
}

func (m Manager) HasBlock() (bool, error) {
	m = m.WithDefaults()
	data, err := os.ReadFile(m.Path)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), m.Begin), nil
}

func (m Manager) Enable(domains []string) error {
	m = m.WithDefaults()
	data, err := os.ReadFile(m.Path)
	if err != nil {
		return err
	}
	next := AddBlock(string(data), domains, m.Begin, m.End)
	return os.WriteFile(m.Path, []byte(next), 0644)
}

func (m Manager) Disable() error {
	m = m.WithDefaults()
	data, err := os.ReadFile(m.Path)
	if err != nil {
		return err
	}
	next := StripBlock(string(data), m.Begin, m.End)
	return os.WriteFile(m.Path, []byte(next), 0644)
}

func AddBlock(content string, domains []string, begin, end string) string {
	base := StripBlock(content, begin, end)
	var block strings.Builder
	block.WriteString(begin + "\n")
	for _, domain := range domains {
		block.WriteString("0.0.0.0 " + domain + "\n")
		block.WriteString(":: " + domain + "\n")
	}
	block.WriteString(end + "\n")
	return strings.TrimRight(base, "\n") + "\n\n" + block.String()
}

func StripBlock(content, begin, end string) string {
	lines := strings.Split(content, "\n")
	var out []string
	inBlock := false
	for _, line := range lines {
		switch strings.TrimSpace(line) {
		case begin:
			inBlock = true
			continue
		case end:
			inBlock = false
			continue
		}
		if !inBlock {
			out = append(out, line)
		}
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
}
