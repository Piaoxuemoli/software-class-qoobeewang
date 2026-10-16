//go:build linux

package testutil

import (
	"fmt"
	"hash/fnv"
	"io"
	"math/rand/v2"
	"os"
	"path"
	"path/filepath"
	"sort"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// pendingMeta 是创建完全部节点后才设置的元数据（目录的时间会被子项创建改写）。
type pendingMeta struct {
	rel          string
	symlink      bool
	mode         uint32
	owner        *Owner
	mtime, atime time.Time
}

type pendingLink struct {
	rel, target string
}

// CreateTree 在已存在的目录 root（通常是 t.TempDir()）下生成 tree；root 本身的元数据不变。
//
// 顺序：先创建除硬链接外的全部条目，再创建硬链接，最后自底向上设置元数据
// （lchown → chmod → atime/mtime，软链接不 chmod、时间设置在链接本身），
// 因此目录时间不会被其子项的创建改写。
// 树中含 Device，或 Owner 与当前进程不同，且当前不是 root 时调用 t.Skip。
// CreateTree 会注册清理函数恢复树中目录的写权限，保证 t.TempDir 能删除只读目录。
func CreateTree(t testing.TB, root string, tree Dir) {
	t.Helper()
	if os.Geteuid() != 0 {
		if why := needsRoot(tree, ""); why != "" {
			t.Skipf("需要 root：%s", why)
		}
	}
	t.Cleanup(func() { makeRemovable(root) })

	var links []pendingLink
	var metas []pendingMeta
	if err := createDir(root, "", tree, &links, &metas); err != nil {
		t.Fatalf("CreateTree: %v", err)
	}
	for _, l := range links {
		if err := os.Link(abs(root, l.target), abs(root, l.rel)); err != nil {
			t.Fatalf("CreateTree: 硬链接 %s → %s: %v", l.rel, l.target, err)
		}
	}
	// metas 按先序收集，逆序即子项先于父目录。
	for i := len(metas) - 1; i >= 0; i-- {
		if err := applyMeta(root, metas[i]); err != nil {
			t.Fatalf("CreateTree: 设置 %s 的元数据: %v", metas[i].rel, err)
		}
	}
}

func abs(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(rel))
}

func sortedNames(d Dir) []string {
	names := make([]string, 0, len(d))
	for name := range d {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// needsRoot 返回树中第一个需要 root 才能创建的条目说明；不需要时返回 ""。
func needsRoot(d Dir, dir string) string {
	foreign := func(o *Owner) bool {
		return o != nil && (int(o.UID) != os.Geteuid() || int(o.GID) != os.Getegid())
	}
	for _, name := range sortedNames(d) {
		rel := path.Join(dir, name)
		var owner *Owner
		switch e := d[name].(type) {
		case Device:
			return rel + " 是设备文件"
		case Dir:
			if why := needsRoot(e, rel); why != "" {
				return why
			}
		case MetaDir:
			owner = e.Owner
			if why := needsRoot(e.Entries, rel); why != "" {
				return why
			}
		case File:
			owner = e.Owner
		case Symlink:
			owner = e.Owner
		case FIFO:
			owner = e.Owner
		case Socket:
			owner = e.Owner
		}
		if foreign(owner) {
			return rel + " 指定了其他属主"
		}
	}
	return ""
}

func createDir(root, dir string, d Dir, links *[]pendingLink, metas *[]pendingMeta) error {
	for _, name := range sortedNames(d) {
		if name == "" || name == "." || name == ".." || containsSlash(name) {
			return fmt.Errorf("目录 %q 中的条目名 %q 非法", dir, name)
		}
		rel := path.Join(dir, name)
		p := abs(root, rel)
		meta := pendingMeta{rel: rel}
		switch e := d[name].(type) {
		case File:
			if err := writeFile(p, rel, e); err != nil {
				return err
			}
			meta.mode, meta.owner, meta.mtime, meta.atime = orDefault(e.Mode, DefaultFileMode), e.Owner, e.Mtime, e.Atime
		case Dir:
			if err := os.Mkdir(p, 0o700); err != nil {
				return err
			}
			*metas = append(*metas, pendingMeta{rel: rel, mode: DefaultDirMode})
			if err := createDir(root, rel, e, links, metas); err != nil {
				return err
			}
			continue
		case MetaDir:
			if err := os.Mkdir(p, 0o700); err != nil {
				return err
			}
			*metas = append(*metas, pendingMeta{
				rel: rel, mode: orDefault(e.Mode, DefaultDirMode), owner: e.Owner, mtime: e.Mtime, atime: e.Atime,
			})
			if err := createDir(root, rel, e.Entries, links, metas); err != nil {
				return err
			}
			continue
		case Symlink:
			if err := os.Symlink(e.Target, p); err != nil {
				return err
			}
			meta.symlink, meta.owner, meta.mtime, meta.atime = true, e.Owner, e.Mtime, e.Atime
		case Hardlink:
			*links = append(*links, pendingLink{rel: rel, target: e.Target})
			continue
		case FIFO:
			if err := syscall.Mkfifo(p, 0o600); err != nil {
				return fmt.Errorf("mkfifo %s: %w", rel, err)
			}
			meta.mode, meta.owner, meta.mtime, meta.atime = orDefault(e.Mode, DefaultNodeMode), e.Owner, e.Mtime, e.Atime
		case Socket:
			if err := syscall.Mknod(p, syscall.S_IFSOCK|0o600, 0); err != nil {
				return fmt.Errorf("mknod %s: %w", rel, err)
			}
			meta.mode, meta.owner, meta.mtime, meta.atime = orDefault(e.Mode, DefaultNodeMode), e.Owner, e.Mtime, e.Atime
		case Device:
			kind := uint32(syscall.S_IFBLK)
			if e.Char {
				kind = syscall.S_IFCHR
			}
			if err := syscall.Mknod(p, kind|0o600, int(mkdev(e.Major, e.Minor))); err != nil {
				return fmt.Errorf("mknod %s: %w", rel, err)
			}
			meta.mode, meta.owner, meta.mtime, meta.atime = orDefault(e.Mode, DefaultNodeMode), e.Owner, e.Mtime, e.Atime
		default:
			return fmt.Errorf("%s: 不支持的条目类型 %T", rel, e)
		}
		*metas = append(*metas, meta)
	}
	return nil
}

func containsSlash(s string) bool {
	for i := range len(s) {
		if s[i] == '/' {
			return true
		}
	}
	return false
}

func orDefault(mode, def uint32) uint32 {
	if mode == 0 {
		return def
	}
	return mode
}

// writeFile 写入 File 的内容：Content 优先，否则流式写入 Size 字节伪随机数据。
func writeFile(p, rel string, e File) error {
	if e.Content != "" && e.Size != 0 && e.Size != int64(len(e.Content)) {
		return fmt.Errorf("%s: Content 长度 %d 与 Size %d 矛盾", rel, len(e.Content), e.Size)
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if e.Content != "" {
		_, err = io.WriteString(f, e.Content)
	} else if e.Size > 0 {
		_, err = io.CopyN(f, pseudoRandom(rel), e.Size)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// pseudoRandom 返回由 rel 决定的无限伪随机字节流：同一路径内容相同，不同路径内容不同。
func pseudoRandom(rel string) io.Reader {
	h := fnv.New64a()
	h.Write([]byte(rel))
	seed := h.Sum64()
	return &prngReader{r: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
}

type prngReader struct {
	r   *rand.Rand
	buf [8]byte
	n   int // buf 中尚未读出的字节数
}

func (p *prngReader) Read(b []byte) (int, error) {
	for i := range b {
		if p.n == 0 {
			v := p.r.Uint64()
			for j := range p.buf {
				p.buf[j] = byte(v >> (8 * j))
			}
			p.n = len(p.buf)
		}
		b[i] = p.buf[len(p.buf)-p.n]
		p.n--
	}
	return len(b), nil
}

func applyMeta(root string, m pendingMeta) error {
	p := abs(root, m.rel)
	if m.owner != nil {
		if err := os.Lchown(p, int(m.owner.UID), int(m.owner.GID)); err != nil {
			return err
		}
	}
	if !m.symlink {
		// syscall.Chmod 保留 setuid/setgid/sticky 原始位；chown 会清除它们，故在其后。
		if err := syscall.Chmod(p, m.mode); err != nil {
			return fmt.Errorf("chmod: %w", err)
		}
	}
	mtime := m.mtime
	if mtime.IsZero() {
		mtime = DefaultTime
	}
	atime := m.atime
	if atime.IsZero() {
		atime = mtime
	}
	return lutimes(p, atime, mtime)
}

// utimensat 的参数，取值见 Linux include/uapi/linux/fcntl.h（各架构相同）；syscall 包未导出。
const (
	atFDCWD           = -100
	atSymlinkNofollow = 0x100
)

// lutimes 设置 atime/mtime（纳秒），对软链接作用于链接本身（utimensat + AT_SYMLINK_NOFOLLOW）。
func lutimes(p string, atime, mtime time.Time) error {
	ts := [2]syscall.Timespec{
		syscall.NsecToTimespec(atime.UnixNano()),
		syscall.NsecToTimespec(mtime.UnixNano()),
	}
	ptr, err := syscall.BytePtrFromString(p)
	if err != nil {
		return err
	}
	dirfd := atFDCWD
	_, _, errno := syscall.Syscall6(syscall.SYS_UTIMENSAT, uintptr(dirfd), uintptr(unsafe.Pointer(ptr)),
		uintptr(unsafe.Pointer(&ts[0])), atSymlinkNofollow, 0, 0)
	if errno != 0 {
		return fmt.Errorf("utimensat: %w", errno)
	}
	return nil
}

// makeRemovable 给 root 下全部目录加上属主 rwx，使 os.RemoveAll 能删除只读目录。
func makeRemovable(root string) {
	// WalkDir 先对目录调用回调、再读取其内容，所以先 chmod 的目录随后可以读取。
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = os.Chmod(p, 0o700)
		}
		return nil
	})
}

// mkdev/devMajor/devMinor 按 glibc 的 dev_t 编码（gnu_dev_makedev）组合与拆分设备号。
func mkdev(major, minor uint32) uint64 {
	return uint64(major&0xfff)<<8 | uint64(major&^0xfff)<<32 |
		uint64(minor&0xff) | uint64(minor&^0xff)<<12
}

func devMajor(dev uint64) uint32 {
	return uint32((dev>>8)&0xfff) | uint32((dev>>32)&^0xfff)
}

func devMinor(dev uint64) uint32 {
	return uint32(dev&0xff) | uint32((dev>>12)&^0xff)
}
