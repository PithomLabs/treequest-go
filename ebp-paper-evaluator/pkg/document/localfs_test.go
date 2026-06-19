package document

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writePaper(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestLocalTextIngest_ValidTxt(t *testing.T) {
	p := writePaper(t, "paper.txt", []byte("Title\n\nA supported claim."))
	d, e := (LocalTextIngestor{}).Ingest(context.Background(), PaperInput{Path: p})
	if e != nil || d.Hash == "" || len(d.Chunks) == 0 {
		t.Fatalf("%+v %v", d, e)
	}
}
func TestLocalTextIngest_ValidMarkdown(t *testing.T) {
	p := writePaper(t, "paper.md", []byte("# Title\n\nClaim."))
	d, e := (LocalTextIngestor{}).Ingest(context.Background(), PaperInput{Path: p})
	if e != nil || d.Source.MediaType != "text/markdown" {
		t.Fatalf("%+v %v", d, e)
	}
}
func TestLocalTextIngest_RejectsDirectory(t *testing.T) {
	p := filepath.Join(t.TempDir(), "paper.txt")
	if e := os.Mkdir(p, 0700); e != nil {
		t.Fatal(e)
	}
	_, e := (LocalTextIngestor{}).Ingest(context.Background(), PaperInput{Path: p})
	if e == nil {
		t.Fatal("directory accepted")
	}
}
func TestLocalTextIngest_RejectsUnsupportedExtension(t *testing.T) {
	p := writePaper(t, "paper.pdf", []byte("x"))
	_, e := (LocalTextIngestor{}).Ingest(context.Background(), PaperInput{Path: p})
	if !errors.Is(e, ErrUnsupportedExtension) {
		t.Fatal(e)
	}
}
func TestLocalTextIngest_RejectsNonUTF8(t *testing.T) {
	p := writePaper(t, "paper.txt", []byte{0xff})
	_, e := (LocalTextIngestor{}).Ingest(context.Background(), PaperInput{Path: p})
	if !errors.Is(e, ErrInvalidUTF8) {
		t.Fatal(e)
	}
}
func TestLocalTextIngest_RejectsTooLargeFile(t *testing.T) {
	p := writePaper(t, "paper.txt", []byte("12345"))
	_, e := (LocalTextIngestor{}).Ingest(context.Background(), PaperInput{Path: p, MaxBytes: 4})
	if !errors.Is(e, ErrPaperTooLarge) {
		t.Fatal(e)
	}
}
func TestLocalTextIngest_StableSourceHash(t *testing.T) {
	p := writePaper(t, "paper.txt", []byte("a\r\n b"))
	a, e := (LocalTextIngestor{}).Ingest(context.Background(), PaperInput{Path: p})
	if e != nil {
		t.Fatal(e)
	}
	b, e := (LocalTextIngestor{}).Ingest(context.Background(), PaperInput{Path: p})
	if e != nil || a.Hash != b.Hash {
		t.Fatal("unstable hash")
	}
}
func TestLocalTextIngest_RejectsOutsideInputRoot(t *testing.T) {
	root := t.TempDir()
	p := writePaper(t, "paper.txt", []byte("x"))
	_, e := (LocalTextIngestor{}).Ingest(context.Background(), PaperInput{Path: p, InputRoot: root})
	if !errors.Is(e, ErrPathOutsideRoot) {
		t.Fatal(e)
	}
}
func TestLocalTextIngest_RejectsSymlinkOutsideRoot(t *testing.T) {
	root := t.TempDir()
	target := writePaper(t, "paper.txt", []byte("x"))
	link := filepath.Join(root, "paper.txt")
	if e := os.Symlink(target, link); e != nil {
		t.Skip(e)
	}
	_, e := (LocalTextIngestor{}).Ingest(context.Background(), PaperInput{Path: "paper.txt", InputRoot: root})
	if !errors.Is(e, ErrSymlinkRejected) {
		t.Fatal(e)
	}
}
func TestLocalTextIngest_SectionAndChunkIDsStable(t *testing.T) {
	p := writePaper(t, "paper.txt", []byte("A\n\nB\n\nC"))
	a, _ := (LocalTextIngestor{}).Ingest(context.Background(), PaperInput{Path: p})
	b, _ := (LocalTextIngestor{}).Ingest(context.Background(), PaperInput{Path: p})
	if a.Sections[0].ID != b.Sections[0].ID || a.Chunks[0].ID != b.Chunks[0].ID {
		t.Fatal("unstable IDs")
	}
}
func TestCLI_RejectsHTTPURLAsPaper(t *testing.T) {
	_, e := (LocalTextIngestor{}).Ingest(context.Background(), PaperInput{Path: "https://example.com/paper.txt"})
	if !errors.Is(e, ErrRemotePaperUnsupported) {
		t.Fatal(e)
	}
}
