package main

import "testing"

func TestNewLogger(t *testing.T) {
	logger := newLogger()
	if logger == nil {
		t.Fatal("expected logger instance")
	}
}
