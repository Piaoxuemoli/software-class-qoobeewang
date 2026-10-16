package engine

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/container"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/filter"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/huffman"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/pack"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/walk"
)

// source 是校验过的备份源：Abs 写入容器头，Root 是解析软链接后的真实路径（遍历根）。
type source struct {
	Abs, Root string
	Filter    filter.Filter
}

// prepare 校验源目录、把排除目录换算为相对路径、自动排除位于源目录内的仓库，
// 并编译过滤器。Backup 与 DryRun 共用，不写任何文件。
func prepare(opts BackupOptions) (source, error) {
	abs, err := filepath.Abs(opts.Source)
	if err != nil {
		return source{}, fmt.Errorf("解析源目录 %s: %w", opts.Source, err)
	}
	st, err := os.Stat(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return source{}, fmt.Errorf("%w: %s", ErrSourceNotFound, opts.Source)
	case err != nil:
		return source{}, fmt.Errorf("访问源目录: %w", err)
	case !st.IsDir():
		return source{}, fmt.Errorf("%w: %s", ErrSourceNotDir, opts.Source)
	}
	// 源目录本身可以是指向目录的软链接；遍历从真实路径开始（walk 不跟随软链接）。
	root, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return source{}, fmt.Errorf("解析源目录 %s: %w", opts.Source, err)
	}

	c := opts.Criteria
	c.ExcludeDirs = nil
	for _, d := range opts.Criteria.ExcludeDirs {
		rel, err := excludeRel(abs, root, d)
		if err != nil {
			return source{}, err
		}
		c.ExcludeDirs = append(c.ExcludeDirs, rel)
	}
	if opts.Repo != "" {
		repoAbs, err := filepath.Abs(opts.Repo)
		if err != nil {
			return source{}, fmt.Errorf("解析仓库目录 %s: %w", opts.Repo, err)
		}
		if rel, ok := within(root, realPath(repoAbs)); ok {
			if rel == "." {
				return source{}, fmt.Errorf("仓库目录不能是源目录本身: %w", ErrInvalidOptions)
			}
			c.ExcludeDirs = append(c.ExcludeDirs, rel) // 契约 §2.1 过滤语义 5
		}
	}
	f, err := filter.New(c)
	if err != nil {
		return source{}, err
	}
	return source{Abs: abs, Root: root, Filter: f}, nil
}

// excludeRel 把 --exclude-dir 换算为相对源目录、以 "/" 分隔的路径：相对路径按源目录解析；
// 绝对路径必须位于源目录内（按用户给出的路径或解析软链接后的真实路径判断均可）。
func excludeRel(abs, root, dir string) (string, error) {
	if filepath.IsAbs(dir) {
		p := filepath.Clean(dir)
		rel, ok := within(abs, p)
		if !ok {
			rel, ok = within(root, realPath(p))
		}
		if !ok || rel == "." {
			return "", fmt.Errorf("排除目录 %s 不在源目录内: %w", dir, ErrInvalidOptions)
		}
		return rel, nil
	}
	rel := path.Clean(filepath.ToSlash(dir))
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("排除目录 %s 不在源目录内: %w", dir, ErrInvalidOptions)
	}
	return rel, nil
}

// within 报告 p 是否位于 base 之内（或就是 base），并返回以 "/" 分隔的相对路径。
func within(base, p string) (string, bool) {
	rel, err := filepath.Rel(base, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// realPath 解析 p 中的软链接；p 尚不存在时解析其最深的已存在祖先，再接上剩余部分。
func realPath(p string) string {
	var rest []string
	for cur := p; ; cur = filepath.Dir(cur) {
		if real, err := filepath.EvalSymlinks(cur); err == nil {
			for i := len(rest) - 1; i >= 0; i-- {
				real = filepath.Join(real, rest[i])
			}
			return real
		}
		if parent := filepath.Dir(cur); parent == cur {
			return p
		}
		rest = append(rest, filepath.Base(cur))
	}
}

// scan 是 scan 阶段：遍历并过滤，emit 非 nil 时收到每个保留的条目。无法读取的项发 warn 后跳过。
func (r *run) scan(src source, emit func(fsmeta.FileInfo)) (walk.Stats, error) {
	prog := r.newProgress(PhaseScan)
	stats, err := walk.Walk(r.ctx, src.Root, walk.Options{
		Filter: src.Filter,
		OnError: func(rel string, err error) error {
			r.warn("跳过无法读取的项 %s：%v", rel, err)
			return nil
		},
	}, func(fi fsmeta.FileInfo) error {
		if emit != nil {
			emit(fi)
		}
		if fi.Type != fsmeta.TypeDir {
			prog.cur.Files++
			prog.cur.Bytes += fi.Size
		}
		prog.cur.Current = fi.Path
		prog.tick()
		return nil
	})
	if err != nil {
		return stats, err
	}
	prog.flush()
	return stats, nil
}

// writeArchive 是 pack 阶段：把 entries 依次写成 path 处的备份文件
// （pack → [huffman] → container[→AES+HMAC] → 文件），成功时已 fsync 并关闭。
// 失败时文件保留，由调用方删除。
func (r *run) writeArchive(path string, src source, h container.Header, password []byte,
	entries []fsmeta.FileInfo, total walk.Stats) (sum container.Summary, err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return sum, fmt.Errorf("创建备份文件: %w", err)
	}
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("关闭备份文件: %w", cerr)
		}
	}()
	bw := bufio.NewWriterSize(f, ioBufSize)
	cw, err := container.NewWriter(bw, h, password)
	if err != nil {
		return sum, err
	}
	var payload io.Writer = cw
	var hw *huffman.Writer
	if h.Compression == container.CompressionHuffman {
		hw = huffman.NewWriter(cw)
		payload = hw
	}
	pw := pack.NewWriter(payload)
	if sum, err = r.packEntries(pw, src.Root, entries, total); err != nil {
		return sum, err
	}
	if err := pw.Close(); err != nil {
		return sum, err
	}
	if hw != nil {
		if err := hw.Close(); err != nil {
			return sum, err
		}
	}
	if err := cw.Finish(sum); err != nil {
		return sum, err
	}
	if err := bw.Flush(); err != nil {
		return sum, fmt.Errorf("写入备份文件: %w", err)
	}
	if err := f.Sync(); err != nil {
		return sum, fmt.Errorf("同步备份文件: %w", err)
	}
	return sum, nil
}

// packEntries 写出全部条目并返回实际写入的统计（打包时才发现无法读取而跳过的文件不计入）。
func (r *run) packEntries(pw *pack.Writer, root string, entries []fsmeta.FileInfo,
	total walk.Stats) (container.Summary, error) {
	var sum container.Summary
	prog := r.newProgress(PhasePack).withTotal(total.Files, total.Bytes)
	buf := make([]byte, ioBufSize)
	// skipped 记录打包时无法读取而跳过的普通文件；指向它们的硬链接也只能跳过。
	skipped := map[string]bool{}
	for _, fi := range entries {
		if err := r.cancelled(); err != nil {
			return sum, err
		}
		prog.cur.Current = fi.Path
		switch {
		case fi.Type == fsmeta.TypeDir:
			if err := pw.WriteHeader(fi); err != nil {
				return sum, err
			}
			sum.Dirs++
			prog.tick()
			continue
		case fi.IsHardlink() && skipped[fi.HardlinkTo]:
			r.warn("跳过硬链接 %s：其目标 %s 未能备份", fi.Path, fi.HardlinkTo)
			continue
		case fi.Type == fsmeta.TypeRegular && !fi.IsHardlink():
			ok, err := r.packFile(pw, filepath.Join(root, filepath.FromSlash(fi.Path)), fi, buf, prog)
			if err != nil {
				return sum, err
			}
			if !ok {
				skipped[fi.Path] = true
				continue
			}
			sum.Bytes += fi.Size
		default: // 硬链接、软链接与特殊文件：只有头记录
			if err := pw.WriteHeader(fi); err != nil {
				return sum, err
			}
		}
		sum.Files++
		prog.cur.Files++
		prog.tick()
	}
	prog.flush()
	return sum, nil
}

// packFile 写出一个普通文件的头记录与数据。打开失败（权限不足、扫描后被删除或换了类型）
// 时发 warn 并返回 ok=false，不写任何内容。数据按扫描时的大小写出：文件在此期间变短时
// 以零补齐并发 warn，变长时只写到原大小。
func (r *run) packFile(pw *pack.Writer, abs string, fi fsmeta.FileInfo, buf []byte, prog *progressor) (ok bool, err error) {
	f, err := openSource(abs)
	if err != nil {
		r.warn("跳过无法读取的文件 %s：%v", fi.Path, err)
		return false, nil
	}
	defer f.Close()
	if err := pw.WriteHeader(fi); err != nil {
		return false, err
	}
	var n int64
	for n < fi.Size {
		if err := r.cancelled(); err != nil {
			return false, err
		}
		k, rerr := f.Read(buf[:min(int64(len(buf)), fi.Size-n)])
		if k > 0 {
			if _, err := pw.Write(buf[:k]); err != nil {
				return false, err
			}
			n += int64(k)
			prog.cur.Bytes += int64(k)
			prog.tick()
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return false, fmt.Errorf("读取 %s: %w", fi.Path, rerr)
		}
	}
	if short := fi.Size - n; short > 0 {
		clear(buf)
		for short > 0 {
			k := min(short, int64(len(buf)))
			if _, err := pw.Write(buf[:k]); err != nil {
				return false, err
			}
			short -= k
		}
		prog.cur.Bytes += fi.Size - n
		r.warn("文件 %s 在备份过程中变短，缺少的 %d 字节已用零补齐", fi.Path, fi.Size-n)
	}
	return true, nil
}
