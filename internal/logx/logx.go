// logx：轻量结构化日志（级别过滤 + tag 前缀，零依赖）
// Q39 需求：明细请求日志仅 debug 级；级别由 config log_level 控制
package logx

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Level 日志级别（数字越大越详细）
type Level int

const (
	ErrorLevel Level = iota
	WarnLevel
	InfoLevel
	DebugLevel
)

var levelNames = map[Level]string{
	ErrorLevel: "ERROR",
	WarnLevel:  "WARN",
	InfoLevel:  "INFO",
	DebugLevel: "DEBUG",
}

var (
	mu    sync.Mutex
	level Level = InfoLevel
	out   io.Writer = os.Stdout
)

// ParseLevel 解析配置字符串（debug/info/warn/error，大小写不敏感）
func ParseLevel(s string) (Level, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return DebugLevel, true
	case "info", "":
		return InfoLevel, true
	case "warn":
		return WarnLevel, true
	case "error":
		return ErrorLevel, true
	}
	return InfoLevel, false
}

// SetLevel 设置全局级别（低于该级别的不输出）
func SetLevel(l Level) {
	mu.Lock()
	defer mu.Unlock()
	level = l
}

// IsEnabled 当前级别是否允许输出 l
func IsEnabled(l Level) bool {
	mu.Lock()
	defer mu.Unlock()
	return l <= level
}

func logf(l Level, tag, format string, args ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	if l > level {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if tag != "" {
		msg = "[" + tag + "] " + msg
	}
	fmt.Fprintf(out, "%s %-5s %s\n", time.Now().Format("2006-01-02T15:04:05.000"), levelNames[l], msg)
}

// Errorf 错误级
func Errorf(tag, format string, args ...interface{}) { logf(ErrorLevel, tag, format, args...) }

// Warnf 警告级
func Warnf(tag, format string, args ...interface{}) { logf(WarnLevel, tag, format, args...) }

// Infof 信息级
func Infof(tag, format string, args ...interface{}) { logf(InfoLevel, tag, format, args...) }

// Debugf 调试级（Q39：明细请求日志仅 debug 级，默认不输出）
func Debugf(tag, format string, args ...interface{}) { logf(DebugLevel, tag, format, args...) }
