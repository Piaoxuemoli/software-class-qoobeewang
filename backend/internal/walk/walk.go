// Package walk 先序遍历源目录，产出经过过滤的 fsmeta.FileInfo 序列。
//
// 约定：不跟随软链接（lstat）；同一目录内按名字字节序排序，输出顺序稳定；
// 源目录本身不输出；按 Dev+Ino 识别硬链接（只对被保留的条目），第二次及以后出现的
// 路径填写 FileInfo.HardlinkTo。进度节流由 engine 负责，本包不做。
package walk

import (
	"context"
	"errors"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/filter"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
)

var errNotImplemented = errors.New("walk: 尚未实现")

// Options 配置一次遍历。
type Options struct {
	// Filter 为 nil 时全部保留。
	Filter filter.Filter
	// OnError 处理单个条目的读取错误（如权限不足）：返回 nil 表示跳过该条目继续遍历，
	// 返回非 nil 则中止遍历并由 Walk 返回该错误。为 nil 时任何错误都中止。
	OnError func(rel string, err error) error
}

// Stats 是遍历的统计，口径与契约 §3.1 一致。
type Stats struct {
	// Files 是保留的非目录条目数（含链接与特殊文件）。
	Files int64
	// Dirs 是保留的目录数。
	Dirs int64
	// Bytes 是保留的普通文件数据总字节（硬链接条目不计）。
	Bytes int64
	// Skipped 是被过滤掉的条目数（SkipDir 的目录计 1，不计其内部）。
	Skipped int64
	// Errors 是经 OnError 跳过的条目数。
	Errors int64
}

// Walk 先序遍历 root，对每个保留的条目调用 fn。fn 返回错误时中止遍历并返回该错误。
// ctx 取消时尽快返回 ctx.Err()。
func Walk(ctx context.Context, root string, opts Options, fn func(fsmeta.FileInfo) error) (Stats, error) {
	return Stats{}, errNotImplemented
}
