package engine

import (
	"context"
	"fmt"
	"io"
	"time"
)

// 流水线缓冲区大小：读写用户文件与备份文件时每次系统调用的数据量。
const ioBufSize = 256 << 10

// progressInterval 是同一阶段两次进度报告的最小间隔（ADR-013：每阶段 ≤10 次/秒）。
// 测试可以调整它。
var progressInterval = 100 * time.Millisecond

// run 是一次 Backup/DryRun/Restore 的执行上下文：取消信号、报告器与警告计数。
type run struct {
	ctx      context.Context
	rep      Reporter
	warnings int64
}

func newRun(ctx context.Context, rep Reporter) *run {
	if rep == nil {
		rep = nopReporter{}
	}
	return &run{ctx: ctx, rep: rep}
}

// warn 输出一条 warn 日志并计入 warnings。
func (r *run) warn(format string, args ...any) {
	r.warnings++
	r.rep.Log(LevelWarn, fmt.Sprintf(format, args...))
}

func (r *run) info(format string, args ...any) {
	r.rep.Log(LevelInfo, fmt.Sprintf(format, args...))
}

// cancelled 在 ctx 已取消时返回 cancelError，否则返回 nil。
func (r *run) cancelled() error {
	if err := r.ctx.Err(); err != nil {
		return cancelError{err}
	}
	return nil
}

// finish 归一化流水线返回的错误：ctx 已取消时一律报告为取消
// （下层因取消而产生的读写错误不应以 E_IO 等形式暴露给用户）。
func (r *run) finish(err error) error {
	if err == nil {
		return nil
	}
	if c := r.cancelled(); c != nil {
		return c
	}
	return err
}

// cancelError 包装 ctx.Err()：对用户显示中文说明，errors.Is(err, context.Canceled) 仍成立，
// 由 cli 映射为 E_CANCELLED。
type cancelError struct{ err error }

func (e cancelError) Error() string { return "操作已取消" }
func (e cancelError) Unwrap() error { return e.err }

type nopReporter struct{}

func (nopReporter) Progress(Progress) {}
func (nopReporter) Log(Level, string) {}

func (r *run) newProgress(ph Phase) *progressor {
	return &progressor{rep: r.rep, cur: Progress{Phase: ph}}
}

// progressor 节流一个阶段的进度报告：调用方直接修改 cur 后调用 tick；
// 距上次报告不足 progressInterval 时不发。阶段结束调用 flush 必发一次。
type progressor struct {
	rep  Reporter
	cur  Progress
	last time.Time
}

func (p *progressor) withTotal(files, bytes int64) *progressor {
	p.cur.HasTotal, p.cur.TotalFiles, p.cur.TotalBytes = true, files, bytes
	return p
}

func (p *progressor) tick() {
	now := time.Now()
	if !p.last.IsZero() && now.Sub(p.last) < progressInterval {
		return
	}
	p.last = now
	p.rep.Progress(p.cur)
}

func (p *progressor) flush() {
	p.last = time.Now()
	p.rep.Progress(p.cur)
}

// ctxReadSeeker 在每次 Read 前检查 ctx，使 container.Open 的整文件校验可以被取消。
type ctxReadSeeker struct {
	ctx context.Context
	rs  io.ReadSeeker
}

func (c ctxReadSeeker) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.rs.Read(p)
}

func (c ctxReadSeeker) Seek(off int64, whence int) (int64, error) {
	return c.rs.Seek(off, whence)
}
