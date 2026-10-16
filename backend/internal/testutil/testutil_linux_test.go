//go:build linux

package testutil

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"
)

// recorder 截获 AssertTreeEqual / CreateTree / Golden 的报告，用于验证夹具本身。
type recorder struct {
	testing.TB
	errors  []string
	fatal   string
	skipped bool
}

func (r *recorder) Helper()                   {}
func (r *recorder) Logf(string, ...any)       {}
func (r *recorder) Errorf(f string, a ...any) { r.errors = append(r.errors, fmt.Sprintf(f, a...)) }
func (r *recorder) Fatalf(f string, a ...any) {
	r.fatal = fmt.Sprintf(f, a...)
	runtime.Goexit()
}
func (r *recorder) Skipf(string, ...any) {
	r.skipped = true
	runtime.Goexit()
}

// record 在独立 goroutine 中运行 fn，使 Fatalf/Skipf 的 Goexit 只结束 fn。
func record(t *testing.T, fn func(tb testing.TB)) *recorder {
	r := &recorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn(r)
	}()
	<-done
	return r
}

var t1 = time.Date(2025, 7, 8, 9, 10, 11, 987654321, time.UTC)

// base 返回覆盖全部非 root 类型的目录树声明；每次返回新 map，便于逐项篡改。
func base() Dir {
	return Dir{
		"a.txt":    File{Content: "hello", Mode: 0o640},
		"b":        File{Content: "bbb"},
		"big.bin":  File{Size: 200 << 10},
		"empty":    File{},
		"suid":     File{Content: "x", Mode: 0o4755},
		"hard":     Hardlink{Target: "a.txt"},
		"hard2":    Hardlink{Target: "sub/b.txt"},
		"ln":       Symlink{Target: "a.txt"},
		"dangling": Symlink{Target: "/nonexistent/x", Mtime: t1},
		"pipe":     FIFO{},
		"sock":     Socket{Mode: 0o755},
		"sub": MetaDir{Mode: 0o750, Mtime: t1, Entries: Dir{
			"b.txt": File{Content: "b"},
			"deep":  Dir{"c": File{Content: "c", Atime: t1}},
		}},
	}
}

func create(t *testing.T, tree Dir) string {
	t.Helper()
	root := t.TempDir()
	CreateTree(t, root, tree)
	return root
}

// kinds 把差异归约为排序后的 "要素 路径" 列表，便于精确断言。
func kinds(diffs []diff) []string {
	var out []string
	for _, d := range diffs {
		out = append(out, d.elem.String()+" "+d.path)
	}
	sort.Strings(out)
	return out
}

func lstat(t *testing.T, p string) syscall.Stat_t {
	t.Helper()
	var st syscall.Stat_t
	if err := syscall.Lstat(p, &st); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestCreateTree(t *testing.T) {
	old := syscall.Umask(0o077)
	t.Cleanup(func() { syscall.Umask(old) })
	root := create(t, base())

	// 必须最先运行：后面的子测试会读取文件与软链接，刷新其 atime。
	t.Run("时间：缺省 DefaultTime，目录与软链接本身的时间不被改写", func(t *testing.T) {
		cases := []struct {
			rel          string
			mtime, atime time.Time
		}{
			{"b", DefaultTime, DefaultTime},
			{"ln", DefaultTime, DefaultTime},
			{"dangling", t1, t1},
			{"sub", t1, t1},
			{"sub/deep", DefaultTime, DefaultTime},
			{"sub/deep/c", DefaultTime, t1},
		}
		for _, c := range cases {
			st := lstat(t, filepath.Join(root, c.rel))
			if m := time.Unix(st.Mtim.Unix()); !m.Equal(c.mtime) {
				t.Errorf("%s mtime = %v，期望 %v", c.rel, m, c.mtime)
			}
			if a := time.Unix(st.Atim.Unix()); !a.Equal(c.atime) {
				t.Errorf("%s atime = %v，期望 %v", c.rel, a, c.atime)
			}
		}
	})

	t.Run("内容与权限不受 umask 影响", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(root, "a.txt"))
		if err != nil || string(data) != "hello" {
			t.Fatalf("a.txt = %q, %v", data, err)
		}
		for rel, want := range map[string]uint32{
			"a.txt": 0o640, "b": DefaultFileMode, "suid": 0o4755, "pipe": DefaultNodeMode,
			"sock": 0o755, "sub": 0o750, "sub/deep": DefaultDirMode,
		} {
			if got := lstat(t, filepath.Join(root, rel)).Mode & 0o7777; got != want {
				t.Errorf("%s 权限 = %04o，期望 %04o", rel, got, want)
			}
		}
	})

	t.Run("类型", func(t *testing.T) {
		for rel, want := range map[string]uint32{
			"pipe": syscall.S_IFIFO, "sock": syscall.S_IFSOCK, "ln": syscall.S_IFLNK,
			"sub": syscall.S_IFDIR, "empty": syscall.S_IFREG,
		} {
			if got := lstat(t, filepath.Join(root, rel)).Mode & syscall.S_IFMT; got != want {
				t.Errorf("%s 类型 = %o，期望 %o", rel, got, want)
			}
		}
		if target, err := os.Readlink(filepath.Join(root, "dangling")); err != nil || target != "/nonexistent/x" {
			t.Errorf("dangling → %q, %v", target, err)
		}
	})

	t.Run("Size 生成按路径确定的伪随机内容", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(root, "big.bin"))
		if err != nil || len(data) != 200<<10 {
			t.Fatalf("big.bin len = %d, %v", len(data), err)
		}
		other := create(t, Dir{"big.bin": File{Size: 200 << 10}, "x": File{Size: 200 << 10}})
		same, _ := os.ReadFile(filepath.Join(other, "big.bin"))
		diff, _ := os.ReadFile(filepath.Join(other, "x"))
		if !bytes.Equal(data, same) {
			t.Error("同一路径的伪随机内容不一致")
		}
		if bytes.Equal(data, diff) {
			t.Error("不同路径的伪随机内容相同")
		}
	})

	t.Run("硬链接共享 inode", func(t *testing.T) {
		for link, target := range map[string]string{"hard": "a.txt", "hard2": "sub/b.txt"} {
			if lstat(t, filepath.Join(root, link)).Ino != lstat(t, filepath.Join(root, target)).Ino {
				t.Errorf("%s 与 %s 不是同一 inode", link, target)
			}
		}
	})

	t.Run("非法声明报 Fatal", func(t *testing.T) {
		for name, tree := range map[string]Dir{
			"硬链接目标不存在":          {"h": Hardlink{Target: "missing"}},
			"条目名含斜杠":            {"a/b": File{}},
			"Content 与 Size 矛盾": {"f": File{Content: "abc", Size: 5}},
			"nil 条目":            {"n": nil},
		} {
			r := record(t, func(tb testing.TB) { CreateTree(tb, t.TempDir(), tree) })
			if r.fatal == "" {
				t.Errorf("%s: 未报 Fatal", name)
			}
		}
	})

	t.Run("只读目录仍可被 TempDir 清理", func(t *testing.T) {
		// 若清理失败，t.TempDir 的清理会让本测试失败。
		create(t, Dir{"ro": MetaDir{Mode: 0o500, Entries: Dir{"f": File{Mode: 0o400}}}})
	})
}

func TestCreateTreeNeedsRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("以 root 运行时不会跳过")
	}
	for name, tree := range map[string]Dir{
		"设备":   {"dev": Device{Char: true, Major: 1, Minor: 3}},
		"他人属主": {"d": Dir{"f": File{Owner: &Owner{UID: 0, GID: 0}}}},
	} {
		if r := record(t, func(tb testing.TB) { CreateTree(tb, t.TempDir(), tree) }); !r.skipped {
			t.Errorf("%s: 非 root 未跳过", name)
		}
	}
	own := &Owner{UID: uint32(os.Geteuid()), GID: uint32(os.Getegid())}
	create(t, Dir{"f": File{Owner: own}})
}

func TestAssertTreeEqual(t *testing.T) {
	t.Run("同一声明生成的两棵树五要素一致", func(t *testing.T) {
		want, got := create(t, base()), create(t, base())
		diffs, err := diffTrees(want, got, WithAtime())
		if err != nil {
			t.Fatal(err)
		}
		if len(diffs) != 0 {
			t.Errorf("diffs = %v", diffs)
		}
		AssertTreeEqual(t, want, got)
	})

	// 每个要素至少一个反例：篡改 got 的声明或磁盘内容后，比较器必须给出且只给出预期差异。
	cases := []struct {
		name   string
		mutate func(d Dir)
		after  func(t *testing.T, root string)
		opts   []Option
		want   []string
	}{
		{name: "类型：普通文件变目录", mutate: func(d Dir) { d["empty"] = Dir{} },
			want: []string{"类型 empty"}},
		{name: "类型：FIFO 变普通文件", mutate: func(d Dir) { d["pipe"] = File{Mode: DefaultNodeMode} },
			want: []string{"类型 pipe"}},
		{name: "路径：缺失与多出", mutate: func(d Dir) { delete(d, "empty"); d["extra"] = File{} },
			want: []string{"路径 empty", "路径 extra"}},
		{name: "路径：嵌套条目缺失", mutate: func(d Dir) {
			sub := d["sub"].(MetaDir)
			sub.Entries = Dir{"b.txt": File{Content: "b"}, "deep": Dir{}}
			d["sub"] = sub
		}, want: []string{"路径 sub/deep/c"}},
		{name: "数据：等长内容不同", mutate: func(d Dir) { d["b"] = File{Content: "bbc"} },
			want: []string{"数据 b"}},
		{name: "数据：大小不同", mutate: func(d Dir) { d["b"] = File{Content: "bbbb"} },
			want: []string{"数据 b"}},
		{name: "数据：大文件末字节被改（恢复 mtime 后）", after: func(t *testing.T, root string) {
			p := filepath.Join(root, "big.bin")
			f, err := os.OpenFile(p, os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.WriteAt([]byte{0}, 200<<10-1); err != nil {
				t.Fatal(err)
			}
			f.Close()
			if err := lutimes(p, DefaultTime, DefaultTime); err != nil {
				t.Fatal(err)
			}
		}, want: []string{"数据 big.bin"}},
		{name: "数据：软链接目标不同", mutate: func(d Dir) { d["ln"] = Symlink{Target: "b"} },
			want: []string{"数据 ln"}},
		{name: "元数据：文件权限", mutate: func(d Dir) { d["b"] = File{Content: "bbb", Mode: 0o600} },
			want: []string{"元数据 b"}},
		{name: "元数据：setuid 位", mutate: func(d Dir) { d["suid"] = File{Content: "x", Mode: 0o755} },
			want: []string{"元数据 suid"}},
		{name: "元数据：目录权限", mutate: func(d Dir) {
			sub := d["sub"].(MetaDir)
			sub.Mode = 0o700
			d["sub"] = sub
		}, want: []string{"元数据 sub"}},
		{name: "元数据：mtime 差 1ns", mutate: func(d Dir) {
			d["b"] = File{Content: "bbb", Mtime: DefaultTime.Add(time.Nanosecond)}
		}, want: []string{"元数据 b"}},
		{name: "元数据：软链接本身的 mtime", mutate: func(d Dir) { d["ln"] = Symlink{Target: "a.txt", Mtime: t1} },
			want: []string{"元数据 ln"}},
		{name: "元数据：目录 mtime", mutate: func(d Dir) {
			sub := d["sub"].(MetaDir)
			sub.Mtime = DefaultTime
			d["sub"] = sub
		}, want: []string{"元数据 sub"}},
		{name: "元数据：atime 只在 WithAtime 时比较", mutate: func(d Dir) {
			d["b"] = File{Content: "bbb", Atime: t1}
		}, opts: []Option{WithAtime()}, want: []string{"元数据 b"}},
		{name: "IgnoreTimes 忽略 mtime", mutate: func(d Dir) {
			d["b"] = File{Content: "bbb", Mtime: t1}
		}, opts: []Option{IgnoreTimes()}},
		{name: "默认不比较 atime", mutate: func(d Dir) { d["b"] = File{Content: "bbb", Atime: t1} }},
		{name: "链接方式：硬链接被拆成独立副本", mutate: func(d Dir) {
			d["hard"] = File{Content: "hello", Mode: 0o640}
		}, want: []string{"链接方式 a.txt", "链接方式 hard"}},
		{name: "链接方式：硬链接指向另一 inode", mutate: func(d Dir) {
			d["hard2"] = Hardlink{Target: "b"}
			d["b"] = File{Content: "b"}
		}, want: []string{"数据 b", "链接方式 b", "链接方式 hard2", "链接方式 sub/b.txt"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wantTree, gotTree := base(), base()
			if c.mutate != nil {
				c.mutate(gotTree)
			}
			want, got := create(t, wantTree), create(t, gotTree)
			if c.after != nil {
				c.after(t, got)
			}
			diffs, err := diffTrees(want, got, c.opts...)
			if err != nil {
				t.Fatal(err)
			}
			if k := kinds(diffs); !slices.Equal(k, c.want) {
				t.Errorf("差异 = %v，期望 %v\n%v", k, c.want, diffs)
			}
		})
	}

	t.Run("元数据：属主（需要 root）", func(t *testing.T) {
		if os.Geteuid() != 0 {
			t.Skip("需要 root")
		}
		gotTree := base()
		gotTree["b"] = File{Content: "bbb", Owner: &Owner{UID: 12345, GID: 23456}}
		want, got := create(t, base()), create(t, gotTree)
		diffs, err := diffTrees(want, got)
		if err != nil {
			t.Fatal(err)
		}
		if k := kinds(diffs); !slices.Equal(k, []string{"元数据 b"}) {
			t.Errorf("差异 = %v", diffs)
		}
		if diffs, _ := diffTrees(want, got, IgnoreOwner()); len(diffs) != 0 {
			t.Errorf("IgnoreOwner 后仍有差异 %v", diffs)
		}
	})

	t.Run("数据：设备号（需要 root）", func(t *testing.T) {
		if os.Geteuid() != 0 {
			t.Skip("需要 root")
		}
		tree := func(minor uint32) Dir {
			return Dir{
				"null": Device{Char: true, Major: 1, Minor: minor},
				"blk":  Device{Major: 300, Minor: 70000, Mode: 0o660},
			}
		}
		want := create(t, tree(3))
		st := lstat(t, filepath.Join(want, "blk"))
		if st.Mode&syscall.S_IFMT != syscall.S_IFBLK || devMajor(uint64(st.Rdev)) != 300 || devMinor(uint64(st.Rdev)) != 70000 {
			t.Errorf("blk mode=%o rdev=%d:%d", st.Mode, devMajor(uint64(st.Rdev)), devMinor(uint64(st.Rdev)))
		}
		if diffs, _ := diffTrees(want, create(t, tree(3))); len(diffs) != 0 {
			t.Errorf("相同设备树有差异 %v", diffs)
		}
		diffs, _ := diffTrees(want, create(t, tree(5)))
		if k := kinds(diffs); !slices.Equal(k, []string{"数据 null"}) {
			t.Errorf("差异 = %v", diffs)
		}
	})

	t.Run("报告格式与上限", func(t *testing.T) {
		gotTree := base()
		gotTree["b"] = File{Content: "bbc"}
		r := record(t, func(tb testing.TB) { AssertTreeEqual(tb, create(t, base()), create(t, gotTree)) })
		if len(r.errors) != 1 || !strings.HasPrefix(r.errors[0], "[数据] b: ") {
			t.Errorf("errors = %q", r.errors)
		}

		many := Dir{}
		for i := range maxReported + 10 {
			many[fmt.Sprintf("f%03d", i)] = File{}
		}
		r = record(t, func(tb testing.TB) { AssertTreeEqual(tb, t.TempDir(), create(t, many)) })
		if len(r.errors) != maxReported+1 || !strings.Contains(r.errors[maxReported], "另有 10 处") {
			t.Errorf("报告了 %d 条，最后一条 %q", len(r.errors), r.errors[len(r.errors)-1])
		}
	})

	t.Run("目录不存在报 Fatal", func(t *testing.T) {
		r := record(t, func(tb testing.TB) { AssertTreeEqual(tb, t.TempDir(), "/nonexistent/testutil") })
		if r.fatal == "" {
			t.Error("未报 Fatal")
		}
	})
}

func TestDevNumbers(t *testing.T) {
	for _, c := range [][2]uint32{{0, 0}, {1, 3}, {8, 17}, {259, 0}, {300, 70000}, {0xfff, 0xff}, {0xfffff, 0xfffff}} {
		dev := mkdev(c[0], c[1])
		if devMajor(dev) != c[0] || devMinor(dev) != c[1] {
			t.Errorf("mkdev(%d,%d) = %#x → %d:%d", c[0], c[1], dev, devMajor(dev), devMinor(dev))
		}
	}
	// 与 glibc 编码对照：8:17 是 0x811。
	if got := mkdev(8, 17); got != 0x811 {
		t.Errorf("mkdev(8,17) = %#x，期望 0x811", got)
	}
}

func TestGolden(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testdata", "x.golden")

	r := record(t, func(tb testing.TB) { Golden(tb, path, []byte("abc")) })
	if r.fatal == "" || !strings.Contains(r.fatal, "-update") {
		t.Errorf("缺少 golden 文件时 fatal = %q", r.fatal)
	}

	*update = true
	Golden(t, path, []byte("abc"))
	*update = false
	if data, err := os.ReadFile(path); err != nil || string(data) != "abc" {
		t.Fatalf("更新后内容 %q, %v", data, err)
	}

	Golden(t, path, []byte("abc"))
	r = record(t, func(tb testing.TB) { Golden(tb, path, []byte("abd")) })
	if len(r.errors) != 1 || !strings.Contains(r.errors[0], "偏移 2") {
		t.Errorf("errors = %q", r.errors)
	}
}
