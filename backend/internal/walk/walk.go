// Package walk 先序遍历源目录，产出经过过滤的 fsmeta.FileInfo 序列。
//
// 约定：不跟随软链接（lstat）；同一目录内按名字字节序排序，输出顺序稳定；
// 源目录本身不输出；按 Dev+Ino 识别硬链接（只对被保留的条目），第二次及以后出现的
// 路径填写 FileInfo.HardlinkTo。进度节流由 engine 负责，本包不做。
package walk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/filter"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
)

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
	info, err := fsmeta.Lstat(root, ".")
	if err != nil {
		return Stats{}, err
	}
	if info.Type != fsmeta.TypeDir {
		return Stats{}, fmt.Errorf("walk: %s 不是目录: %w", root, syscall.ENOTDIR)
	}
	w := &walker{
		ctx:  ctx,
		root: root,
		opts: opts,
		fn:   fn,
		seen: make(map[inodeKey]string),
	}
	if err := w.dir("."); err != nil {
		return w.stats, err
	}
	return w.stats, nil
}

// inodeKey 用 (Dev, Ino) 唯一标识源文件系统上的一个文件，用于硬链接识别。
type inodeKey struct {
	dev, ino uint64
}

type walker struct {
	ctx   context.Context
	root  string
	opts  Options
	fn    func(fsmeta.FileInfo) error
	stats Stats
	// seen 记录"被保留的、非目录且 Nlink>1"的条目第一次出现的路径。
	seen map[inodeKey]string
}

func (w *walker) abs(dirRel string) string {
	return filepath.Join(w.root, filepath.FromSlash(dirRel))
}

// dir 遍历 root/dirRel 的直接子项（先序）；dirRel 为 "." 时表示 root 本身，
// root 的错误不走 OnError，直接中止。
func (w *walker) dir(dirRel string) error {
	// os.ReadDir 返回的条目已按文件名字节序排序，保证输出顺序稳定。
	entries, err := os.ReadDir(w.abs(dirRel))
	if err != nil {
		if dirRel == "." {
			return fmt.Errorf("walk: 读取源目录 %s: %w", w.root, err)
		}
		// 该目录本身已按过滤结论输出过，这里失败的只是它的内容。
		return w.onError(dirRel, fmt.Errorf("读取目录 %s: %w", dirRel, err))
	}
	for _, e := range entries {
		if err := w.ctx.Err(); err != nil {
			return err
		}
		rel := e.Name()
		if dirRel != "." {
			rel = dirRel + "/" + rel
		}
		fi, err := fsmeta.Lstat(w.root, rel)
		if err != nil {
			if err := w.onError(rel, err); err != nil {
				return err
			}
			continue
		}
		decision := filter.Include
		if w.opts.Filter != nil {
			decision = w.opts.Filter.Decide(fi)
		}
		switch decision {
		case filter.SkipDir:
			// 不记录也不进入，内部条目不计入 Skipped。
			w.stats.Skipped++
			continue
		case filter.Exclude:
			// 不记录；目录仍然进入，子项照常判定。
			w.stats.Skipped++
		case filter.Include:
			w.emit(&fi)
			if err := w.fn(fi); err != nil {
				return err
			}
		}
		if fi.Type == fsmeta.TypeDir {
			if err := w.dir(rel); err != nil {
				return err
			}
		}
	}
	return nil
}

// emit 统计被保留的条目并做硬链接识别；可能改写 fi（HardlinkTo、Size）。
func (w *walker) emit(fi *fsmeta.FileInfo) {
	if fi.Type == fsmeta.TypeDir {
		w.stats.Dirs++
		return
	}
	if fi.Nlink > 1 {
		key := inodeKey{dev: fi.Dev, ino: fi.Ino}
		if first, ok := w.seen[key]; ok {
			// 同一 inode 第二次及以后出现：记为硬链接，不携带数据。
			fi.HardlinkTo = first
			fi.Size = 0
		} else {
			w.seen[key] = fi.Path
		}
	}
	w.stats.Files++
	w.stats.Bytes += fi.Size
}

// onError 把单个条目的错误交给 OnError：返回 nil 表示跳过（Errors+1），
// 返回非 nil 中止遍历；OnError 为 nil 时错误直接中止。
func (w *walker) onError(rel string, err error) error {
	if w.opts.OnError == nil {
		return err
	}
	if rerr := w.opts.OnError(rel, err); rerr != nil {
		return rerr
	}
	w.stats.Errors++
	return nil
}
