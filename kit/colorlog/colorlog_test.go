package colorlog

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

func newTestLogger(label string, buf *bytes.Buffer) *slog.Logger {
	return New(label, Options{Output: buf, UseColor: ptr(true)})
}

func TestNew(t *testing.T) {
	t.Run("with label only", func(t *testing.T) {
		logger := New("TEST")
		if logger == nil {
			t.Fatal("New should not return nil")
		}
		h, ok := logger.Handler().(*ColorLogHandler)
		if !ok {
			t.Fatal("handler should be *ColorLogHandler")
		}
		if h.label != "TEST" {
			t.Errorf("label = %q, want TEST", h.label)
		}
	})

	t.Run("with options", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New("TEST", Options{
			Output:   &buf,
			Level:    slog.LevelWarn,
			UseColor: ptr(false),
		})
		h := logger.Handler().(*ColorLogHandler)
		if h.opts.Level != slog.LevelWarn {
			t.Errorf("level = %v, want LevelWarn", h.opts.Level)
		}
		if h.color != false {
			t.Error("color should be false")
		}
	})
}

func TestEnabled(t *testing.T) {
	tests := []struct {
		handlerLevel slog.Level
		logLevel     slog.Level
		want         bool
	}{
		{slog.LevelDebug, slog.LevelDebug, true},
		{slog.LevelDebug, slog.LevelInfo, true},
		{slog.LevelInfo, slog.LevelDebug, false},
		{slog.LevelInfo, slog.LevelInfo, true},
		{slog.LevelWarn, slog.LevelInfo, false},
		{slog.LevelWarn, slog.LevelWarn, true},
		{slog.LevelError, slog.LevelWarn, false},
		{slog.LevelError, slog.LevelError, true},
	}

	for _, tt := range tests {
		var buf bytes.Buffer
		logger := New("TEST", Options{Output: &buf, Level: tt.handlerLevel})
		h := logger.Handler().(*ColorLogHandler)
		got := h.Enabled(context.Background(), tt.logLevel)
		if got != tt.want {
			t.Errorf("Enabled(handlerLevel=%v, logLevel=%v) = %v, want %v",
				tt.handlerLevel, tt.logLevel, got, tt.want)
		}
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := New("TEST", Options{Output: &buf, Level: slog.LevelWarn, UseColor: ptr(true)})

	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	got := buf.String()
	if strings.Contains(got, "debug") {
		t.Error("debug message should be filtered")
	}
	if strings.Contains(got, "info") {
		t.Error("info message should be filtered")
	}
	if !strings.Contains(got, "warn") {
		t.Error("warn message should appear")
	}
	if !strings.Contains(got, "error") {
		t.Error("error message should appear")
	}
}

func TestLevels(t *testing.T) {
	tests := []struct {
		name   string
		level  slog.Level
		prefix string
		color  string
	}{
		{"Debug", slog.LevelDebug, "DEBUG  ", colorGray},
		{"Info", slog.LevelInfo, "", colorCyan},
		{"Warn", slog.LevelWarn, "WARNING  ", colorYellow},
		{"Error", slog.LevelError, "ERROR  ", colorRed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := New("TEST", Options{Output: &buf, Level: slog.LevelDebug, UseColor: ptr(true)})
			logger.Log(context.Background(), tt.level, "test message")
			got := buf.String()

			if !strings.Contains(got, tt.color) {
				t.Errorf("missing color %q in output: %q", tt.color, got)
			}
			if !strings.Contains(got, tt.prefix+"test message") {
				t.Errorf("missing prefix+message %q in output: %q", tt.prefix+"test message", got)
			}
			if !strings.Contains(got, "TEST") {
				t.Errorf("missing label in output: %q", got)
			}
		})
	}
}

func TestWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger := New("TEST", Options{Output: &buf, UseColor: ptr(true)})

	// WithAttrs should return a new logger
	logger2 := logger.With("key1", "value1")
	logger2.Info("message")

	got := buf.String()
	if !strings.Contains(got, "key1") || !strings.Contains(got, "value1") {
		t.Errorf("WithAttrs not applied: %q", got)
	}

	// Original logger should not have the attr
	buf.Reset()
	logger.Info("original")
	got = buf.String()
	if strings.Contains(got, "key1") {
		t.Errorf("original logger should not have attr: %q", got)
	}
}

func TestWithAttrsChained(t *testing.T) {
	var buf bytes.Buffer
	logger := New("TEST", Options{Output: &buf, UseColor: ptr(true)})

	logger.With("a", 1).With("b", 2).Info("chained")

	got := buf.String()
	if !strings.Contains(got, "a") || !strings.Contains(got, "1") {
		t.Errorf("missing first attr: %q", got)
	}
	if !strings.Contains(got, "b") || !strings.Contains(got, "2") {
		t.Errorf("missing second attr: %q", got)
	}
}

func TestWithGroup(t *testing.T) {
	var buf bytes.Buffer
	logger := New("TEST", Options{Output: &buf, UseColor: ptr(true)})

	logger.WithGroup("grp").Info("message", "key", "value")

	got := buf.String()
	if !strings.Contains(got, "grp.key") {
		t.Errorf("group prefix not applied: %q", got)
	}
}

func TestWithGroupNested(t *testing.T) {
	var buf bytes.Buffer
	logger := New("TEST", Options{Output: &buf, UseColor: ptr(true)})

	logger.WithGroup("a").WithGroup("b").Info("message", "key", "value")

	got := buf.String()
	if !strings.Contains(got, "a.b.key") {
		t.Errorf("nested group prefix not applied: %q", got)
	}
}

func TestWithGroupAndAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger := New("TEST", Options{Output: &buf, UseColor: ptr(true)})

	logger.With("pre", "val").WithGroup("grp").Info("msg", "key", "value")

	got := buf.String()
	if !strings.Contains(got, "pre") {
		t.Errorf("pre-group attr missing: %q", got)
	}
	if !strings.Contains(got, "grp.key") {
		t.Errorf("grouped attr missing: %q", got)
	}
}

func TestColorDisabled(t *testing.T) {
	var buf bytes.Buffer
	logger := New("TEST", Options{Output: &buf, UseColor: ptr(false)})

	logger.Info("test message", "key", "value")

	got := buf.String()
	if strings.Contains(got, "\033[") {
		t.Errorf("color codes should not appear when disabled: %q", got)
	}
	if !strings.Contains(got, "test message") {
		t.Errorf("message missing: %q", got)
	}
}

func TestTimeFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger("TEST", &buf)

	logger.Info("test")

	got := buf.String()
	today := time.Now().Format("2006/01/02")
	if !strings.Contains(got, today) {
		t.Errorf("expected date %q in output: %q", today, got)
	}
}

func TestAttributes(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger("TEST", &buf)

	logger.Info("msg",
		"string", "value",
		"int", 42,
		"float", 3.14,
		"bool", true,
	)

	got := buf.String()
	for _, exp := range []string{"string", "value", "int", "42", "float", "3.14", "bool", "true"} {
		if !strings.Contains(got, exp) {
			t.Errorf("missing %q in output: %q", exp, got)
		}
	}
}

func TestThreadSafety(t *testing.T) {
	var buf bytes.Buffer
	logger := New("TEST", Options{Output: &buf, UseColor: ptr(false)})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			logger.Info("message", "n", n)
		}(i)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 100 {
		t.Errorf("expected 100 lines, got %d", len(lines))
	}

	// Each line should be complete (not interleaved)
	for i, line := range lines {
		if !strings.Contains(line, "message") {
			t.Errorf("line %d appears corrupted: %q", i, line)
		}
	}
}

func TestSharedMutex(t *testing.T) {
	var buf bytes.Buffer
	logger := New("TEST", Options{Output: &buf, UseColor: ptr(false)})

	h1 := logger.Handler().(*ColorLogHandler)
	h2 := logger.With("key", "value").Handler().(*ColorLogHandler)
	h3 := logger.WithGroup("grp").Handler().(*ColorLogHandler)

	if h1.mu != h2.mu || h1.mu != h3.mu {
		t.Error("WithAttrs/WithGroup clones should share the same mutex")
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, errors.New("write error")
}

func TestHandleError(t *testing.T) {
	logger := New("TEST", Options{Output: errorWriter{}, UseColor: ptr(false)})
	h := logger.Handler().(*ColorLogHandler)

	err := h.Handle(context.Background(), slog.Record{
		Time:    time.Now(),
		Message: "test",
		Level:   slog.LevelInfo,
	})

	if err == nil || err.Error() != "write error" {
		t.Errorf("expected write error, got %v", err)
	}
}

func TestWithAttrsEmpty(t *testing.T) {
	var buf bytes.Buffer
	logger := New("TEST", Options{Output: &buf, UseColor: ptr(true)})
	h := logger.Handler().(*ColorLogHandler)

	h2 := h.WithAttrs(nil)
	if h2 != h {
		t.Error("WithAttrs(nil) should return same handler")
	}

	h3 := h.WithAttrs([]slog.Attr{})
	if h3 != h {
		t.Error("WithAttrs([]) should return same handler")
	}
}

func TestWithGroupEmpty(t *testing.T) {
	var buf bytes.Buffer
	logger := New("TEST", Options{Output: &buf, UseColor: ptr(true)})
	h := logger.Handler().(*ColorLogHandler)

	h2 := h.WithGroup("")
	if h2 != h {
		t.Error("WithGroup(\"\") should return same handler")
	}
}

func TestOutputFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := New("TEST", Options{Output: &buf, UseColor: ptr(true)})

	logger.Info("hello", "k", "v")
	got := buf.String()

	// Verify structure: timestamp  (label)  message  [attrs]
	if !strings.Contains(got, "("+colorBlue+"TEST"+colorReset+")") {
		t.Errorf("label format incorrect: %q", got)
	}
	if !strings.Contains(got, colorCyan+"hello"+colorReset) {
		t.Errorf("message format incorrect: %q", got)
	}
	if !strings.HasSuffix(got, "]\033[0m\n") {
		t.Errorf("should end with attr bracket and newline: %q", got)
	}
}
