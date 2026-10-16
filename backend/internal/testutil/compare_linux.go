//go:build linux

package testutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
)

// Element 是备份正确性五要素之一（课件6 p19，见 docs/project/course-requirements.md §6.2）。
type Element int

// 五要素。数据包括普通文件内容、软链接目标与设备号；元数据包括权限位、属主、时间。
const (
	ElemType Element = iota + 1
	ElemPath
	ElemData
	ElemMeta
	ElemLink
)

func (e Element) String() string {
	switch e {
	case ElemType:
		return "类型"
	case ElemPath:
		return "路径"
	case ElemData:
		return "数据"
	case ElemMeta:
		return "元数据"
	case ElemLink:
		return "链接方式"
	}
	return "?"
}

// Option 调整 AssertTreeEqual 的比较范围。
type Option func(*compareOptions)

type compareOptions struct {
	ignoreOwner bool
	ignoreTimes bool
	withAtime   bool
}

// IgnoreOwner 不比较 UID/GID（非 root 还原无法 chown 时使用）。
func IgnoreOwner() Option { return func(o *compareOptions) { o.ignoreOwner = true } }

// IgnoreTimes 不比较 atime/mtime。
func IgnoreTimes() Option { return func(o *compareOptions) { o.ignoreTimes = true } }

// WithAtime 额外比较 atime。默认不比较：读取文件会按挂载选项刷新 atime。
func WithAtime() Option { return func(o *compareOptions) { o.withAtime = true } }

// maxReported 是 AssertTreeEqual 逐条报告的差异上限，其余只报数量。
const maxReported = 50

// AssertTreeEqual 按五要素比较 want 与 got 两棵目录树（不含两个根目录本身），
// 每处差异报告一条 t.Errorf，格式为 "[要素] 路径: 说明"。不比较 ctime/btime（无法恢复）
// 与 inode 号本身；硬链接按"哪些路径共享同一 inode"的划分比较。
func AssertTreeEqual(t testing.TB, want, got string, opts ...Option) {
	t.Helper()
	diffs, err := diffTrees(want, got, opts...)
	if err != nil {
		t.Fatalf("AssertTreeEqual: %v", err)
	}
	for i, d := range diffs {
		if i == maxReported {
			t.Errorf("……另有 %d 处差异未列出", len(diffs)-maxReported)
			break
		}
		t.Errorf("%s", d)
	}
}

// DiffTrees 与 AssertTreeEqual 口径相同地比较两棵目录树，返回每处差异的说明
// （格式 "[要素] 路径: 说明"，无差异时为空）。供拿不到 testing.TB 的场合使用，
// 如 testscript 自定义命令。
func DiffTrees(want, got string, opts ...Option) ([]string, error) {
	diffs, err := diffTrees(want, got, opts...)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(diffs))
	for i, d := range diffs {
		out[i] = d.String()
	}
	return out, nil
}

type diff struct {
	elem Element
	path string
	msg  string
}

func (d diff) String() string {
	return fmt.Sprintf("[%s] %s: %s", d.elem, d.path, d.msg)
}

// node 是比较所需的 inode 信息。
type node struct {
	typ          fsmeta.FileType
	perm         uint32
	uid, gid     uint32
	size         int64
	atime, mtime time.Time
	target       string
	rdev         uint64
	dev, ino     uint64
}

func diffTrees(want, got string, opts ...Option) ([]diff, error) {
	var o compareOptions
	for _, opt := range opts {
		opt(&o)
	}
	// 先读取全部 inode 信息，再读文件内容，避免读内容刷新 atime 影响比较。
	wantNodes, err := scanTree(want)
	if err != nil {
		return nil, err
	}
	gotNodes, err := scanTree(got)
	if err != nil {
		return nil, err
	}
	wantPeers, gotPeers := hardlinkPeers(wantNodes), hardlinkPeers(gotNodes)

	var diffs []diff
	add := func(e Element, rel, format string, args ...any) {
		diffs = append(diffs, diff{elem: e, path: rel, msg: fmt.Sprintf(format, args...)})
	}
	for _, rel := range unionPaths(wantNodes, gotNodes) {
		w, g := wantNodes[rel], gotNodes[rel]
		switch {
		case g == nil:
			add(ElemPath, rel, "缺失（期望存在类型 %s）", w.typ)
			continue
		case w == nil:
			add(ElemPath, rel, "多出（类型 %s）", g.typ)
			continue
		case w.typ != g.typ:
			add(ElemType, rel, "期望 %s，实际 %s", w.typ, g.typ)
			continue
		}

		switch w.typ {
		case fsmeta.TypeRegular:
			if w.size != g.size {
				add(ElemData, rel, "大小期望 %d，实际 %d", w.size, g.size)
			} else if off, err := firstContentDiff(filepath.Join(want, rel), filepath.Join(got, rel)); err != nil {
				return nil, err
			} else if off >= 0 {
				add(ElemData, rel, "内容在偏移 %d 处不同", off)
			}
		case fsmeta.TypeSymlink:
			if w.target != g.target {
				add(ElemData, rel, "链接目标期望 %q，实际 %q", w.target, g.target)
			}
		case fsmeta.TypeCharDevice, fsmeta.TypeBlockDevice:
			if w.rdev != g.rdev {
				add(ElemData, rel, "设备号期望 %d:%d，实际 %d:%d",
					devMajor(w.rdev), devMinor(w.rdev), devMajor(g.rdev), devMinor(g.rdev))
			}
		}

		if w.typ != fsmeta.TypeSymlink && w.perm != g.perm {
			add(ElemMeta, rel, "权限期望 %04o，实际 %04o", w.perm, g.perm)
		}
		if !o.ignoreOwner && (w.uid != g.uid || w.gid != g.gid) {
			add(ElemMeta, rel, "属主期望 %d:%d，实际 %d:%d", w.uid, w.gid, g.uid, g.gid)
		}
		if !o.ignoreTimes {
			if !w.mtime.Equal(g.mtime) {
				add(ElemMeta, rel, "mtime 期望 %s，实际 %s", fmtTime(w.mtime), fmtTime(g.mtime))
			}
			if o.withAtime && !w.atime.Equal(g.atime) {
				add(ElemMeta, rel, "atime 期望 %s，实际 %s", fmtTime(w.atime), fmtTime(g.atime))
			}
		}

		if w.typ != fsmeta.TypeDir {
			if wp, gp := strings.Join(wantPeers[rel], ", "), strings.Join(gotPeers[rel], ", "); wp != gp {
				add(ElemLink, rel, "期望与 [%s] 共享 inode，实际与 [%s]", wp, gp)
			}
		}
	}
	return diffs, nil
}

func fmtTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// scanTree 不跟随软链接地读取 root 下全部条目（不含 root 本身），键为以 "/" 分隔的相对路径。
func scanTree(root string) (map[string]*node, error) {
	nodes := map[string]*node{}
	err := filepath.WalkDir(root, func(p string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		n, err := lstatNode(p)
		if err != nil {
			return err
		}
		nodes[filepath.ToSlash(rel)] = n
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("扫描目录树 %s: %w", root, err)
	}
	return nodes, nil
}

func lstatNode(p string) (*node, error) {
	var st syscall.Stat_t
	if err := syscall.Lstat(p, &st); err != nil {
		return nil, &os.PathError{Op: "lstat", Path: p, Err: err}
	}
	n := &node{
		perm:  st.Mode & 0o7777,
		uid:   st.Uid,
		gid:   st.Gid,
		size:  st.Size,
		atime: time.Unix(st.Atim.Unix()),
		mtime: time.Unix(st.Mtim.Unix()),
		rdev:  uint64(st.Rdev),
		dev:   uint64(st.Dev),
		ino:   st.Ino,
	}
	switch st.Mode & syscall.S_IFMT {
	case syscall.S_IFREG:
		n.typ = fsmeta.TypeRegular
	case syscall.S_IFDIR:
		n.typ = fsmeta.TypeDir
	case syscall.S_IFLNK:
		n.typ = fsmeta.TypeSymlink
		target, err := os.Readlink(p)
		if err != nil {
			return nil, err
		}
		n.target = target
	case syscall.S_IFCHR:
		n.typ = fsmeta.TypeCharDevice
	case syscall.S_IFBLK:
		n.typ = fsmeta.TypeBlockDevice
	case syscall.S_IFSOCK:
		n.typ = fsmeta.TypeSocket
	case syscall.S_IFIFO:
		n.typ = fsmeta.TypeFIFO
	default:
		return nil, fmt.Errorf("%s: 未知文件类型 %o", p, st.Mode&syscall.S_IFMT)
	}
	if n.typ != fsmeta.TypeRegular {
		n.size = 0
	}
	return n, nil
}

// hardlinkPeers 返回每个非目录条目在本树中与之共享 inode 的其他路径（已排序）。
func hardlinkPeers(nodes map[string]*node) map[string][]string {
	type key struct{ dev, ino uint64 }
	groups := map[key][]string{}
	for rel, n := range nodes {
		if n.typ != fsmeta.TypeDir {
			k := key{n.dev, n.ino}
			groups[k] = append(groups[k], rel)
		}
	}
	peers := map[string][]string{}
	for _, members := range groups {
		if len(members) < 2 {
			continue
		}
		sort.Strings(members)
		for _, rel := range members {
			for _, other := range members {
				if other != rel {
					peers[rel] = append(peers[rel], other)
				}
			}
		}
	}
	return peers
}

func unionPaths(a, b map[string]*node) []string {
	seen := make(map[string]bool, len(a)+len(b))
	for rel := range a {
		seen[rel] = true
	}
	for rel := range b {
		seen[rel] = true
	}
	paths := make([]string, 0, len(seen))
	for rel := range seen {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	return paths
}

// firstContentDiff 流式比较两个等长文件，返回首个不同字节的偏移；完全相同返回 -1。
func firstContentDiff(a, b string) (int64, error) {
	fa, err := os.Open(a)
	if err != nil {
		return 0, err
	}
	defer fa.Close()
	fb, err := os.Open(b)
	if err != nil {
		return 0, err
	}
	defer fb.Close()

	const chunk = 64 << 10
	bufA, bufB := make([]byte, chunk), make([]byte, chunk)
	var off int64
	for {
		na, errA := io.ReadFull(fa, bufA)
		nb, errB := io.ReadFull(fb, bufB)
		n := min(na, nb)
		if i := firstByteDiff(bufA[:n], bufB[:n]); i >= 0 {
			return off + int64(i), nil
		}
		if na != nb {
			return off + int64(n), nil
		}
		off += int64(n)
		endA := errA == io.EOF || errA == io.ErrUnexpectedEOF
		endB := errB == io.EOF || errB == io.ErrUnexpectedEOF
		if errA != nil && !endA {
			return 0, errA
		}
		if errB != nil && !endB {
			return 0, errB
		}
		if endA || endB {
			if endA != endB {
				return off, nil
			}
			return -1, nil
		}
	}
}

func firstByteDiff(a, b []byte) int {
	if bytes.Equal(a, b) {
		return -1
	}
	for i := range a {
		if a[i] != b[i] {
			return i
		}
	}
	return len(a)
}
