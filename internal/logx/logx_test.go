package logx

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// 级别过滤：debug 默认不输出
func TestLevelFilter(t *testing.T) {
	SetLevel(InfoLevel)
	if IsEnabled(DebugLevel) {
		t.Error("debug should be disabled at info level")
	}
	if !IsEnabled(WarnLevel) {
		t.Error("warn should be enabled at info level")
	}

	SetLevel(DebugLevel)
	if !IsEnabled(DebugLevel) {
		t.Error("debug should be enabled at debug level")
	}
}

// 输出格式：时间 + 级别 + tag + 消息
func TestOutputFormat(t *testing.T) {
	var buf bytes.Buffer
	mu.Lock()
	out = &buf
	mu.Unlock()
	defer func() {
		mu.Lock()
		out = os.Stdout
		mu.Unlock()
	}()

	SetLevel(DebugLevel)
	Infof("sync", "hello %s", "world")
	Debugf("audit", "detail %d", 42)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %q", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], "INFO") || !strings.Contains(lines[0], "[sync]") || !strings.Contains(lines[0], "hello world") {
		t.Errorf("bad info line: %q", lines[0])
	}
	if !strings.Contains(lines[1], "DEBUG") || !strings.Contains(lines[1], "[audit]") || !strings.Contains(lines[1], "detail 42") {
		t.Errorf("bad debug line: %q", lines[1])
	}
}

// 级别之下不输出
func TestLevelBelowSuppressed(t *testing.T) {
	var buf bytes.Buffer
	mu.Lock()
	out = &buf
	mu.Unlock()
	defer func() {
		mu.Lock()
		out = os.Stdout
		mu.Unlock()
	}()

	SetLevel(ErrorLevel)
	Infof("x", "should not appear")
	if buf.Len() != 0 {
		t.Errorf("info should be suppressed at error level: %q", buf.String())
	}
	Errorf("x", "fatal thing")
	if !strings.Contains(buf.String(), "fatal thing") {
		t.Error("error should be output at error level")
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]Level{
		"debug": DebugLevel, "DEBUG": DebugLevel,
		"info": InfoLevel, "": InfoLevel,
		"warn": WarnLevel, "error": ErrorLevel,
	}
	for in, want := range cases {
		got, ok := ParseLevel(in)
		if !ok || got != want {
			t.Errorf("ParseLevel(%q): want %v ok=true, got %v ok=%v", in, want, got, ok)
		}
	}
	if _, ok := ParseLevel("verbose"); ok {
		t.Error("invalid level should return ok=false")
	}
}
