package document

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const DefaultMaxPaperBytes int64 = 10 << 20

var (
	ErrRemotePaperUnsupported = errors.New("remote paper input is unsupported; use a local .txt or .md file")
	ErrUnsupportedExtension   = errors.New("paper must use a .txt or .md extension")
	ErrPathOutsideRoot        = errors.New("paper path is outside input root")
	ErrSymlinkRejected        = errors.New("paper symlinks are rejected")
	ErrPaperTooLarge          = errors.New("paper exceeds maximum size")
	ErrInvalidUTF8            = errors.New("paper is not valid UTF-8")
)

type LocalTextIngestor struct {
	DefaultMaxBytes int64
	ChunkBytes      int
}

func (i LocalTextIngestor) Ingest(ctx context.Context, input PaperInput) (DocumentBundle, error) {
	if err := ctx.Err(); err != nil {
		return DocumentBundle{}, err
	}
	if input.Path == "" {
		return DocumentBundle{}, errors.New("paper path is required")
	}
	lower := strings.ToLower(strings.TrimSpace(input.Path))
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return DocumentBundle{}, ErrRemotePaperUnsupported
	}
	ext := strings.ToLower(filepath.Ext(input.Path))
	if ext != ".txt" && ext != ".md" {
		return DocumentBundle{}, ErrUnsupportedExtension
	}
	root, candidate, err := resolveLocalPath(input.Path, input.InputRoot)
	if err != nil {
		return DocumentBundle{}, err
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return DocumentBundle{}, fmt.Errorf("resolve paper path: %w", err)
	}
	if filepath.Clean(resolved) != filepath.Clean(candidate) {
		return DocumentBundle{}, ErrSymlinkRejected
	}
	if root != "" && !withinRoot(root, resolved) {
		return DocumentBundle{}, ErrPathOutsideRoot
	}
	if info, err := os.Lstat(candidate); err != nil {
		return DocumentBundle{}, err
	} else if info.Mode()&os.ModeSymlink != 0 {
		return DocumentBundle{}, ErrSymlinkRejected
	}
	f, err := os.Open(candidate)
	if err != nil {
		return DocumentBundle{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return DocumentBundle{}, err
	}
	if info.IsDir() {
		return DocumentBundle{}, errors.New("paper path is a directory")
	}
	if !info.Mode().IsRegular() {
		return DocumentBundle{}, errors.New("paper path is not a regular file")
	}
	max := input.MaxBytes
	if max <= 0 {
		max = i.DefaultMaxBytes
	}
	if max <= 0 {
		max = DefaultMaxPaperBytes
	}
	if info.Size() > max {
		return DocumentBundle{}, ErrPaperTooLarge
	}
	raw, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return DocumentBundle{}, err
	}
	if int64(len(raw)) > max {
		return DocumentBundle{}, ErrPaperTooLarge
	}
	if !utf8.Valid(raw) {
		return DocumentBundle{}, ErrInvalidUTF8
	}
	if err := ctx.Err(); err != nil {
		return DocumentBundle{}, err
	}
	chunkBytes := i.ChunkBytes
	if chunkBytes <= 0 {
		chunkBytes = 6000
	}
	media := "text/plain"
	if ext == ".md" {
		media = "text/markdown"
	}
	inputPath := input.Path
	if filepath.IsAbs(inputPath) && !input.IncludeResolvedPath {
		inputPath = filepath.Base(inputPath)
	}
	source := SourceMetadata{Kind: "local_text", InputPath: inputPath, MediaType: media, OriginalHash: hashBytes(raw), SizeBytes: int64(len(raw))}
	if input.IncludeResolvedPath {
		source.ResolvedPath = resolved
	}
	return buildDocument(filepath.Base(candidate), source, raw, chunkBytes), nil
}

func resolveLocalPath(path, inputRoot string) (string, string, error) {
	root := ""
	var err error
	if inputRoot != "" {
		root, err = filepath.Abs(inputRoot)
		if err != nil {
			return "", "", err
		}
		root, err = filepath.EvalSymlinks(root)
		if err != nil {
			return "", "", fmt.Errorf("resolve input root: %w", err)
		}
		st, e := os.Stat(root)
		if e != nil {
			return "", "", e
		}
		if !st.IsDir() {
			return "", "", errors.New("input root is not a directory")
		}
	}
	candidate := path
	if root != "" && !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return "", "", err
	}
	candidate = filepath.Clean(candidate)
	if root != "" && !withinRoot(root, candidate) {
		return "", "", ErrPathOutsideRoot
	}
	return root, candidate, nil
}
func withinRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
