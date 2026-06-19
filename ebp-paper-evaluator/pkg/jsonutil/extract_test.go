package jsonutil

import "testing"

func TestExtractJSON_FirstObjectOnly(t *testing.T) {
	b, e := ExtractFirstJSONObject(`p {"a":1} {"b":2}`)
	if e != nil || string(b) != `{"a":1}` {
		t.Fatalf("%s %v", b, e)
	}
}
func TestExtractJSON_NestedObject(t *testing.T) {
	b, e := ExtractFirstJSONObject(`{"a":{"b":[{"c":1}]}}`)
	if e != nil || len(b) == 0 {
		t.Fatal(e)
	}
}
func TestExtractJSON_BracesInsideString(t *testing.T) {
	b, e := ExtractFirstJSONObject(`x {"a":"}"} y`)
	if e != nil || string(b) != `{"a":"}"}` {
		t.Fatalf("%s %v", b, e)
	}
}
func TestExtractJSON_EscapedQuotes(t *testing.T) {
	if _, e := ExtractFirstJSONObject(`{"a":"x\"}y"}`); e != nil {
		t.Fatal(e)
	}
}
func TestExtractJSON_TrailingTextIgnored(t *testing.T) {
	if _, e := ExtractFirstJSONObject(`before {"a":1} after`); e != nil {
		t.Fatal(e)
	}
}
func TestExtractJSON_MalformedFailsClosed(t *testing.T) {
	if _, e := ExtractFirstJSONObject(`{"a":}`); e == nil {
		t.Fatal("accepted malformed JSON")
	}
}
func TestExtractJSON_UnterminatedFailsClosed(t *testing.T) {
	if _, e := ExtractFirstJSONObject(`prefix {"a":1`); e == nil {
		t.Fatal("accepted unterminated JSON")
	}
}
