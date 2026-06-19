package document

import (
	"crypto/sha256"
	"encoding/hex"
)

func hashBytes(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func buildDocument(title string, source SourceMetadata, raw []byte, chunkBytes int) DocumentBundle {
	text := normalize(string(raw))
	sections := detectSections(text)
	chunks := buildChunks(sections, source.OriginalHash, chunkBytes)
	summary := ""
	if len(sections) > 0 {
		summary = sections[0].Text
		if len(summary) > 1200 {
			summary = summary[:1200]
		}
	}
	return DocumentBundle{ID: source.OriginalHash[:16], Title: title, Abstract: summary, Sections: sections, Chunks: chunks, Source: source, Hash: source.OriginalHash, Summary: summary}
}
