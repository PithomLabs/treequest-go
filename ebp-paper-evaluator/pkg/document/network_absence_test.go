package document

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDocumentPackage_NoHTTPClient(t *testing.T) {
	files, e := filepath.Glob("*.go")
	if e != nil {
		t.Fatal(e)
	}
	for _, path := range files {
		if filepath.Ext(path) != ".go" || filepath.Base(path) == "network_absence_test.go" {
			continue
		}
		f, e := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if e != nil {
			t.Fatal(e)
		}
		for _, imp := range f.Imports {
			name, _ := strconv.Unquote(imp.Path.Value)
			if name == "net/http" || name == "os/exec" {
				t.Fatalf("document package imports forbidden %s in %s", name, path)
			}
		}
	}
}
