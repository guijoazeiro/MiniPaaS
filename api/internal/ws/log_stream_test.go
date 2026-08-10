package ws

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
)

func TestLineWriterSplitsAndBuffers(t *testing.T) {
	var got []string
	w := &lineWriter{stream: "stdout", onLine: func(_, line string) { got = append(got, line) }}

	_, _ = w.Write([]byte("hello wo"))
	_, _ = w.Write([]byte("rld\nfoo\r\nbar"))
	if want := []string{"hello world", "foo"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("after splits: got %v, want %v", got, want)
	}

	w.flush()
	if want := []string{"hello world", "foo", "bar"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("after flush: got %v, want %v", got, want)
	}
}

func TestIsClosedErr(t *testing.T) {
	if !isClosedErr(io.EOF) {
		t.Fatal("io.EOF should be treated as a closed stream")
	}
	if !isClosedErr(context.Canceled) {
		t.Fatal("context cancellation should be treated as a closed stream")
	}
	if isClosedErr(errors.New("EOF in application log")) {
		t.Fatal("unrelated error text must not be treated as a closed stream")
	}
}

func TestLineWriterFlushEmpty(t *testing.T) {
	called := false
	w := &lineWriter{stream: "stderr", onLine: func(_, _ string) { called = true }}
	w.flush()
	if called {
		t.Fatal("flush of empty buffer should not emit")
	}
}
