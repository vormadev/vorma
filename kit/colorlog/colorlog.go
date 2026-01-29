package colorlog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

const (
	colorReset  = "\033[0m"
	colorGray   = "\033[37m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
	colorBlue   = "\033[34m"
)

type Options struct {
	Output   io.Writer
	Level    slog.Level
	UseColor *bool // nil = auto-detect
}

type ColorLogHandler struct {
	label  string
	opts   Options
	mu     *sync.Mutex // shared across WithAttrs/WithGroup clones
	attrs  []slog.Attr
	groups []string
	color  bool
}

func New(label string, opts ...Options) *slog.Logger {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	if o.Output == nil {
		o.Output = os.Stdout
	}

	h := &ColorLogHandler{
		label: label,
		opts:  o,
		mu:    &sync.Mutex{},
		color: detectColor(o.Output, o.UseColor),
	}
	return slog.New(h)
}

func detectColor(w io.Writer, override *bool) bool {
	if override != nil {
		return *override
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func (h *ColorLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level
}

func (h *ColorLogHandler) Handle(_ context.Context, r slog.Record) error {
	timeStr := r.Time.Format("2006/01/02 15:04:05")

	// Collect all attrs: handler's stored attrs + record's attrs
	allAttrs := make([]slog.Attr, 0, len(h.attrs)+r.NumAttrs())
	allAttrs = append(allAttrs, h.attrs...)
	r.Attrs(func(a slog.Attr) bool {
		allAttrs = append(allAttrs, h.prefixAttr(a))
		return true
	})

	// Build attrs string
	var attrsStr string
	for i, a := range allAttrs {
		if i > 0 {
			attrsStr += " "
		}
		attrsStr += fmt.Sprintf("%s %s %s %v %s",
			h.wrap(colorGray, "["),
			h.wrap(colorGray, a.Key),
			h.wrap(colorGray, "="),
			a.Value.Any(),
			h.wrap(colorGray, "]"),
		)
	}

	// Build message
	levelColor := h.levelToColor(r.Level)
	prefix := h.levelToPrefix(r.Level)

	var msg string
	if len(allAttrs) == 0 {
		msg = fmt.Sprintf("%s  (%s)  %s\n",
			h.wrap(colorGray, timeStr),
			h.wrap(colorBlue, h.label),
			h.wrap(levelColor, prefix+r.Message),
		)
	} else {
		msg = fmt.Sprintf("%s  (%s)  %s  %s\n",
			h.wrap(colorGray, timeStr),
			h.wrap(colorBlue, h.label),
			h.wrap(levelColor, prefix+r.Message),
			attrsStr,
		)
	}

	h.mu.Lock()
	_, err := io.WriteString(h.opts.Output, msg)
	h.mu.Unlock()
	return err
}

func (h *ColorLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	newAttrs := make([]slog.Attr, len(h.attrs), len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	for _, a := range attrs {
		newAttrs = append(newAttrs, h.prefixAttr(a))
	}
	return &ColorLogHandler{
		label:  h.label,
		opts:   h.opts,
		mu:     h.mu,
		attrs:  newAttrs,
		groups: h.groups,
		color:  h.color,
	}
}

func (h *ColorLogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name
	return &ColorLogHandler{
		label:  h.label,
		opts:   h.opts,
		mu:     h.mu,
		attrs:  h.attrs,
		groups: newGroups,
		color:  h.color,
	}
}

func (h *ColorLogHandler) prefixAttr(a slog.Attr) slog.Attr {
	if len(h.groups) == 0 {
		return a
	}
	key := ""
	for _, g := range h.groups {
		key += g + "."
	}
	key += a.Key
	return slog.Attr{Key: key, Value: a.Value}
}

func (h *ColorLogHandler) wrap(color string, v any) string {
	if !h.color {
		return fmt.Sprintf("%v", v)
	}
	return fmt.Sprintf("%s%v%s", color, v, colorReset)
}

func (h *ColorLogHandler) levelToColor(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return colorRed
	case level >= slog.LevelWarn:
		return colorYellow
	case level >= slog.LevelInfo:
		return colorCyan
	default:
		return colorGray
	}
}

func (h *ColorLogHandler) levelToPrefix(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "ERROR  "
	case level >= slog.LevelWarn:
		return "WARNING  "
	case level >= slog.LevelInfo:
		return ""
	default:
		return "DEBUG  "
	}
}
