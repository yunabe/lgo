package gojupyterscaffold

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

func TestLoggerInfo(t *testing.T) {
	defer log.SetOutput(os.Stderr)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	logger.Info("a", "b", 10, 20)
	suffix := "ab10 20\n"
	got := buf.String()
	if !strings.HasSuffix(got, suffix) {
		t.Errorf("want %q suffix, got %q", suffix, got)
	}
}

func TestLoggerInfof(t *testing.T) {
	defer log.SetOutput(os.Stderr)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	logger.Infof("%05d", 123)
	suffix := "00123\n"
	got := buf.String()
	if !strings.HasSuffix(got, suffix) {
		t.Errorf("want %q suffix, got %q", suffix, got)
	}
}
