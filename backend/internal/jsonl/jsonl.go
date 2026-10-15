// Package jsonl emits JSON Lines records (one JSON object per line) for
// the backend's machine-readable stdout protocol consumed by the client.
package jsonl

import (
	"encoding/json"
	"fmt"
	"io"
)

// Emitter writes one JSON object per line to an underlying writer.
// Struct fields marshal in declaration order, so record shapes are stable.
type Emitter struct {
	w io.Writer
}

// NewEmitter returns an Emitter writing to w.
func NewEmitter(w io.Writer) *Emitter {
	return &Emitter{w: w}
}

// Emit marshals v as a single JSON object followed by a newline.
func (e *Emitter) Emit(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("jsonl: marshal: %w", err)
	}
	if _, err := e.w.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("jsonl: write: %w", err)
	}
	return nil
}
