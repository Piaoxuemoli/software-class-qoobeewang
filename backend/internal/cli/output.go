package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/config"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/engine"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/filter"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/jsonl"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/repo"
)

// message 返回面向用户的错误文本：剥掉包装链末尾的哨兵错误后缀
// （如 "…: 用法错误"），保留具体原因。
func message(err error) string {
	msg := err.Error()
	for _, s := range []error{
		ErrUsage, filter.ErrInvalidCriteria, repo.ErrInvalidName,
		config.ErrInvalid, engine.ErrInvalidOptions,
	} {
		if suffix := ": " + s.Error(); strings.HasSuffix(msg, suffix) {
			return strings.TrimSuffix(msg, suffix)
		}
	}
	return msg
}

// reporter 按输出模式返回 engine.Reporter：--json 发 progress/log 事件，
// 否则把日志与人类可读进度写 stderr。
func (c *cliCtx) reporter() engine.Reporter {
	if c.jsonMode {
		return jsonlReporter{emit: c.emit, stderr: c.stderr}
	}
	return humanReporter{w: c.stderr}
}

// jsonlReporter 把 engine 的进度/日志适配为 JSONL 事件。
type jsonlReporter struct {
	emit   *jsonl.Emitter
	stderr io.Writer
}

func (r jsonlReporter) Progress(p engine.Progress) {
	ev := jsonl.Progress{
		Type:    jsonl.TypeProgress,
		Phase:   string(p.Phase),
		Files:   p.Files,
		Bytes:   p.Bytes,
		Current: p.Current,
	}
	if p.HasTotal {
		ev.TotalFiles = &p.TotalFiles
		ev.TotalBytes = &p.TotalBytes
	}
	if err := r.emit.Emit(ev); err != nil {
		fmt.Fprintf(r.stderr, "databackup: %v\n", err)
	}
}

func (r jsonlReporter) Log(level engine.Level, msg string) {
	l := jsonl.LevelInfo
	switch level {
	case engine.LevelWarn:
		l = jsonl.LevelWarn
	case engine.LevelError:
		l = jsonl.LevelError
	}
	if err := r.emit.Emit(jsonl.Log{Type: jsonl.TypeLog, Level: l, Message: msg}); err != nil {
		fmt.Fprintf(r.stderr, "databackup: %v\n", err)
	}
}

// humanReporter 把进度与日志写成人类可读文本（走 stderr，stdout 只放结果）。
type humanReporter struct {
	w io.Writer
}

func (h humanReporter) Progress(p engine.Progress) {
	if p.HasTotal {
		fmt.Fprintf(h.w, "%s: %d/%d 项, %d/%d 字节\r",
			p.Phase, p.Files, p.TotalFiles, p.Bytes, p.TotalBytes)
		return
	}
	fmt.Fprintf(h.w, "%s: %d 项, %d 字节\r", p.Phase, p.Files, p.Bytes)
}

func (h humanReporter) Log(level engine.Level, msg string) {
	prefix := "databackup"
	if level == engine.LevelWarn {
		prefix = "databackup: warn"
	} else if level == engine.LevelError {
		prefix = "databackup: error"
	}
	fmt.Fprintf(h.w, "%s: %s\n", prefix, msg)
}
