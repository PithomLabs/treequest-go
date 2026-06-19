package document

import "strings"

func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for n := range lines {
		lines[n] = strings.TrimSpace(lines[n])
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
