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
	c.human = &humanReporter{w: c.stderr}
	return c.human
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
// 同一阶段的进度用 "\r" 原地刷新（计数单调不减，新行不会比旧行短）；换阶段、
// 输出日志或命令结束前先换行，避免与进度行粘在一起。
type humanReporter struct {
	w       io.Writer
	pending bool // 已输出一行未换行的进度
	phase   engine.Phase
}

func (h *humanReporter) Progress(p engine.Progress) {
	if h.pending && p.Phase != h.phase {
		h.endLine()
	}
	h.phase, h.pending = p.Phase, true
	if p.HasTotal {
		fmt.Fprintf(h.w, "\r%s: %d/%d 项, %d/%d 字节",
			p.Phase, p.Files, p.TotalFiles, p.Bytes, p.TotalBytes)
		return
	}
	fmt.Fprintf(h.w, "\r%s: %d 项, %d 字节", p.Phase, p.Files, p.Bytes)
}

// endLine 结束正在刷新的进度行。
func (h *humanReporter) endLine() {
	if h.pending {
		fmt.Fprintln(h.w)
		h.pending = false
	}
}

func (h *humanReporter) Log(level engine.Level, msg string) {
	h.endLine()
	prefix := "databackup"
	if level == engine.LevelWarn {
		prefix = "databackup: warn"
	} else if level == engine.LevelError {
		prefix = "databackup: error"
	}
	fmt.Fprintf(h.w, "%s: %s\n", prefix, msg)
}
