package main

import (
	"os"
	"strings"
	"testing"
)

func TestCLI_NoURLPaperKind(t *testing.T) {
	b, e := os.ReadFile("main.go")
	if e != nil {
		t.Fatal(e)
	}
	s := string(b)
	if strings.Contains(s, "paper-kind") || strings.Contains(s, "URLIngestor") || strings.Contains(s, "document.Ingest(") {
		t.Fatal("remote ingestion CLI surface present")
	}
}
func TestRun_DoesNotInstantiateURLIngestor(t *testing.T) {
	b, e := os.ReadFile("main.go")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(b), "document.LocalTextIngestor") {
		t.Fatal("run does not use local text ingestor")
	}
}
