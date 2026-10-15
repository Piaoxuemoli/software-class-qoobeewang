package jsonl

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type rec struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestEmitSingleLine(t *testing.T) {
	var buf bytes.Buffer
	e := NewEmitter(&buf)
	if err := e.Emit(rec{Type: "result", Name: "a", Count: 1}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if err := e.Emit(rec{Type: "result", Name: "b", Count: 2}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	got := buf.String()
	want := "{\"type\":\"result\",\"name\":\"a\",\"count\":1}\n" +
		"{\"type\":\"result\",\"name\":\"b\",\"count\":2}\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// Every emitted line must be valid JSON.
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	for i, ln := range lines {
		var v map[string]any
		if err := json.Unmarshal([]byte(ln), &v); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}

func TestEmitEscapesSpecialChars(t *testing.T) {
	var buf bytes.Buffer
	e := NewEmitter(&buf)
	if err := e.Emit(rec{Type: "x\"y\nz"}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	out := buf.String()
	if strings.Count(out, "\n") != 1 {
		t.Errorf("record must be exactly one line, got %q", out)
	}
	if !strings.Contains(out, `\"`) || !strings.Contains(out, `\n`) {
		t.Errorf("special chars not escaped: %q", out)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("boom") }

func TestEmitWriteError(t *testing.T) {
	e := NewEmitter(failWriter{})
	if err := e.Emit(rec{Type: "x"}); err == nil {
		t.Error("expected write error, got nil")
	}
}

func TestEmitMarshalError(t *testing.T) {
	var buf bytes.Buffer
	e := NewEmitter(&buf)
	if err := e.Emit(func() {}); err == nil {
		t.Error("expected marshal error, got nil")
	}
}
