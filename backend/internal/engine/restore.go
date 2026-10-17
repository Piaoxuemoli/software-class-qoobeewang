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

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/container"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/huffman"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/pack"
)

// checkTarget 确认还原目录不存在或为空目录。
func checkTarget(target string) error {
	st, err := os.Stat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("访问还原目录: %w", err)
	}
	if !st.IsDir() {
		return fmt.Errorf("%w: %s 不是目录", ErrTargetNotEmpty, target)
	}
	d, err := os.Open(target)
	if err != nil {
		return fmt.Errorf("读取还原目录: %w", err)
	}
	defer d.Close()
	if names, _ := d.Readdirnames(1); len(names) > 0 {
		return fmt.Errorf("%w: %s", ErrTargetNotEmpty, target)
	}
	return nil
}

// openBackup 打开备份文件，完整校验后返回负载明文流（已按头部解压）与容器信息。
// 校验通过前不写任何文件。
func (r *run) openBackup(f *os.File, password []byte) (io.Reader, container.Info, error) {
	info, err := container.ReadInfo(f)
	if err != nil {
		return nil, info, err
	}
	if info.Header.Encryption == container.EncryptionNone && password != nil {
		r.warn("备份未加密，已忽略口令")
		password = nil
	}
	cr, err := container.Open(ctxReadSeeker{ctx: r.ctx, rs: f}, password)
	if err != nil {
		return nil, info, err
	}
	if info.Header.Compression == container.CompressionHuffman {
		return huffman.NewReader(cr), info, nil // huffman.Reader 自带缓冲
	}
	return bufio.NewReaderSize(cr, ioBufSize), info, nil
}

// dirMeta 是延后到全部子项还原之后才设置的目录元数据。
type dirMeta struct {
	path string
	fi   fsmeta.FileInfo
}

// extractor 把 pack 流还原到 target。
type extractor struct {
	*run
	target   string
	root     bool // 以 root 运行：恢复属主
	uid, gid int
	warned   bool // 已发过"跳过属主恢复"的 warn
	parents  map[string]bool
	// seen 是归档中已经出现过的显式目录条目；与 parents 的区别在于
	// ensureParent 补建的父目录不在其中——补建后再到达的显式条目合法。
	seen     map[string]bool
	dirs     []dirMeta
	prog     *progressor
	buf      []byte
	files    int64
	dirCount int64
	bytes    int64
}

func (r *run) newExtractor(target string, total container.Summary) *extractor {
	uid, gid, root := currentOwnership()
	return &extractor{
		run:     r,
		target:  target,
		root:    root,
		uid:     uid,
		gid:     gid,
		parents: map[string]bool{},
		seen:    map[string]bool{},
		prog:    r.newProgress(PhaseRestore).withTotal(total.Files, total.Bytes),
		buf:     make([]byte, ioBufSize),
	}
}

// extract 依次还原 pr 中的全部条目，最后自底向上设置目录元数据。
func (x *extractor) extract(pr *pack.Reader) error {
	for {
		if err := x.cancelled(); err != nil {
			return err
		}
		fi, err := pr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取归档: %w", err)
		}
		x.prog.cur.Current = fi.Path
		if err := x.entry(pr, fi); err != nil {
			return err
		}
		x.prog.tick()
	}
	// 目录按先序收集，逆序即子目录先于父目录：设置子目录元数据不会改动父目录的 mtime，
	// 父目录的只读权限也不会妨碍访问子目录。
	for i := len(x.dirs) - 1; i >= 0; i-- {
		if err := x.applyMeta(x.dirs[i].path, x.dirs[i].fi); err != nil {
			return err
		}
	}
	x.prog.flush()
	return nil
}

// entry 还原一个条目（目录的元数据延后设置）。
func (x *extractor) entry(pr *pack.Reader, fi fsmeta.FileInfo) error {
	p := x.abs(fi.Path)
	if err := x.ensureParent(fi.Path); err != nil {
		return err
	}
	var err error
	var skip bool // 平台不支持的条目：Linux 上仅非 root 设备节点（发 warn）；Windows 上含全部特殊文件（静默）
	switch {
	case fi.Type == fsmeta.TypeDir:
		if x.seen[fi.Path] {
			return fmt.Errorf("归档中路径 %s 重复: %w", fi.Path, pack.ErrCorrupt)
		}
		// 先以 0700 创建保证子项可写；最终权限最后设置。
		if err = os.Mkdir(p, 0o700); err != nil && isDir(p) && os.IsExist(err) {
			err = nil // 已作为缺失的父目录自动创建过
		}
		if err == nil {
			x.seen[fi.Path] = true
			x.parents[fi.Path] = true
			x.dirs = append(x.dirs, dirMeta{path: p, fi: fi})
			x.dirCount++
			return nil
		}
	case fi.IsHardlink():
		skip, err = x.linkEntry(fi)
	case fi.Type == fsmeta.TypeRegular:
		err = x.writeFile(pr, p, fi)
	case fi.Type == fsmeta.TypeSymlink:
		skip, err = x.symlinkEntry(p, fi)
	default: // FIFO、套接字、字符/块设备
		skip, err = x.createSpecial(p, fi)
	}
	if skip {
		return nil
	}
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("归档中路径 %s 重复: %w", fi.Path, pack.ErrCorrupt)
		}
		return fmt.Errorf("还原 %s: %w", fi.Path, err)
	}
	x.files++
	x.prog.cur.Files++
	if fi.IsHardlink() {
		return nil // 与目标共享 inode，元数据已随目标设置
	}
	return x.applyMeta(p, fi)
}

func (x *extractor) abs(rel string) string {
	return filepath.Join(x.target, filepath.FromSlash(rel))
}

// ensureParent 创建归档中没有记录的父目录（如 --types 不含 d 时）：权限 0755、属当前用户。
// pack.Reader 已拒绝位于软链接之下的路径，因此父路径上不会有软链接。
func (x *extractor) ensureParent(rel string) error {
	dir := path.Dir(rel)
	if dir == "." || x.parents[dir] {
		return nil
	}
	if err := os.MkdirAll(x.abs(dir), 0o755); err != nil {
		return fmt.Errorf("创建父目录 %s: %w", dir, err)
	}
	for d := dir; d != "." && !x.parents[d]; d = path.Dir(d) {
		x.parents[d] = true
	}
	return nil
}

// link 还原硬链接。目标必须是此前还原出的普通文件：拒绝指向软链接的硬链接，
// 否则构造的归档可借"硬链接到软链接"在还原目录外写文件。
func (x *extractor) link(fi fsmeta.FileInfo) error {
	old := x.abs(fi.HardlinkTo)
	st, err := os.Lstat(old)
	if err != nil {
		return fmt.Errorf("硬链接 %s 的目标 %s 不存在: %w", fi.Path, fi.HardlinkTo, pack.ErrCorrupt)
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("硬链接 %s 的目标 %s 不是普通文件: %w", fi.Path, fi.HardlinkTo, pack.ErrUnsafePath)
	}
	return os.Link(old, x.abs(fi.Path))
}

// writeFile 新建（O_EXCL，不会写穿已存在的软链接）并写入一个普通文件的数据。
func (x *extractor) writeFile(pr *pack.Reader, p string, fi fsmeta.FileInfo) (err error) {
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	for {
		if err := x.cancelled(); err != nil {
			return err
		}
		k, rerr := pr.Read(x.buf)
		if k > 0 {
			if _, err := f.Write(x.buf[:k]); err != nil {
				return err
			}
			x.bytes += int64(k)
			x.prog.cur.Bytes += int64(k)
			x.prog.tick()
		}
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}

// applyMeta 恢复元数据；非 root 时跳过属主，遇到属主与当前用户不同的条目时发一次 warn。
func (x *extractor) applyMeta(p string, fi fsmeta.FileInfo) error {
	if ownershipSupported && !x.root && !x.warned && (int(fi.UID) != x.uid || int(fi.GID) != x.gid) {
		x.warned = true
		x.warn("非 root 用户，已跳过属主恢复（chown）")
	}
	if err := fsmeta.ApplyMetadata(p, fi, fsmeta.ApplyOptions{Chown: ownershipSupported && x.root}); err != nil {
		return fmt.Errorf("恢复 %s 的元数据: %w", fi.Path, err)
	}
	return nil
}

func isDir(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.IsDir()
}

func isDevice(t fsmeta.FileType) bool {
	return t == fsmeta.TypeCharDevice || t == fsmeta.TypeBlockDevice
}
