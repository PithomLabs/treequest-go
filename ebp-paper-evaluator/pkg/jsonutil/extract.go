package jsonutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var ErrJSONObjectNotFound = errors.New("complete JSON object not found")

func ExtractFirstJSONObject(s string) ([]byte, error) {
	start := -1
	depth := 0
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if start < 0 {
			if ch == '{' {
				start = i
				depth = 1
			}
			continue
		}
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				candidate := []byte(s[start : i+1])
				var raw json.RawMessage
				dec := json.NewDecoder(bytes.NewReader(candidate))
				if err := dec.Decode(&raw); err != nil {
					return nil, fmt.Errorf("malformed JSON object: %w", err)
				}
				if err := requireEOF(dec); err != nil {
					return nil, err
				}
				return candidate, nil
			}
		}
	}
	return nil, ErrJSONObjectNotFound
}

func DecodeFirstJSONObject[T any](s string) (T, error) {
	var out T
	b, err := ExtractFirstJSONObject(s)
	if err != nil {
		return out, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&out); err != nil {
		return out, err
	}
	if err = requireEOF(dec); err != nil {
		return out, err
	}
	return out, nil
}
func requireEOF(dec *json.Decoder) error {
	var extra any
	err := dec.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("multiple JSON values")
	}
	return err
}
