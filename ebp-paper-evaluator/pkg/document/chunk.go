package document

import (
	"fmt"
	"regexp"
	"strings"
)

func detectSections(text string) []Section {
	parts := regexp.MustCompile(`\n{2,}`).Split(text, -1)
	var out []Section
	var b strings.Builder
	ordinal := 0
	flush := func() {
		if b.Len() == 0 {
			return
		}
		t := strings.TrimSpace(b.String())
		out = append(out, Section{ID: fmt.Sprintf("section-%03d", ordinal+1), Title: fmt.Sprintf("Section %d", ordinal+1), Ordinal: ordinal, Text: t})
		ordinal++
		b.Reset()
	}
	for _, part := range parts {
		if b.Len()+len(part) > 20000 {
			flush()
		}
		b.WriteString(part)
		b.WriteString("\n\n")
	}
	flush()
	if len(out) == 0 && text != "" {
		out = []Section{{ID: "section-001", Title: "Document", Text: text}}
	}
	return out
}
func buildChunks(sections []Section, sourceHash string, size int) []Chunk {
	var out []Chunk
	for _, section := range sections {
		for start := 0; start < len(section.Text); {
			end := start + size
			if end > len(section.Text) {
				end = len(section.Text)
			}
			if end < len(section.Text) {
				if n := strings.LastIndex(section.Text[start:end], "\n"); n > size/2 {
					end = start + n
				}
			}
			id := hashBytes([]byte(fmt.Sprintf("%s:%s:%d:%d", sourceHash, section.ID, start, end)))
			out = append(out, Chunk{ID: id[:16], SectionID: section.ID, Text: section.Text[start:end], Start: start, End: end, TokenCount: (end - start) / 4, SourceHash: sourceHash})
			start = end
		}
	}
	return out
}
