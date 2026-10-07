package core

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestNoOpLogger(t *testing.T) {
	l := NewNoOpLogger()
	// Should not panic
	l.Printf("hello %s", "world")
	l.Println("testing")
}

func TestStdLogger(t *testing.T) {
	var buf bytes.Buffer
	stdLog := log.New(&buf, "PREFIX: ", 0)
	l := NewStdLogger(stdLog)

	l.Printf("item %d", 42)
	l.Println("done")

	out := buf.String()
	if !strings.Contains(out, "PREFIX: item 42") {
		t.Fatalf("buf.String() = %q, want to contain %q", out, "PREFIX: item 42")
	}
	if !strings.Contains(out, "done") {
		t.Fatalf("buf.String() = %q, want to contain %q", out, "done")
	}
}
