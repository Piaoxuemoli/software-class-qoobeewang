//go:build linux

package walk_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/filter"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/testutil"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/walk"
)

// stubFilter 是测试用的过滤实现：decisions 按路径给出结论，缺省 Include。
type stubFilter struct {
	decisions map[string]filter.Decision
	seen      []string // Decide 收到的全部路径（判定顺序即遍历顺序）
}

func (f *stubFilter) Decide(fi fsmeta.FileInfo) filter.Decision {
	f.seen = append(f.seen, fi.Path)
	if d, ok := f.decisions[fi.Path]; ok {
		return d
	}
	return filter.Include
}

// collect 跑一次遍历并返回保留条目的序列。
func collect(t *testing.T, root string, opts walk.Options) ([]fsmeta.FileInfo, walk.Stats) {
	t.Helper()
	var out []fsmeta.FileInfo
	stats, err := walk.Walk(context.Background(), root, opts, func(fi fsmeta.FileInfo) error {
		out = append(out, fi)
		return nil
	})
	if err != nil {
		t.Fatalf("Walk err = %v", err)
	}
	return out, stats
}

func paths(infos []fsmeta.FileInfo) []string {
	p := make([]string, len(infos))
	for i, fi := range infos {
		p[i] = fi.Path
	}
	return p
}

func TestWalk(t *testing.T) {
	t.Run("TC-WALK-001 先序与字节序排序，多次运行结果相同", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"b.txt": testutil.File{Content: "b"},
			"a.txt": testutil.File{Content: "a"},
			"sub": testutil.Dir{
				"z.txt": testutil.File{Content: "z"},
				"y.txt": testutil.File{Content: "y"},
				"deep":  testutil.Dir{"x": testutil.File{Content: "x"}},
			},
		})
		got1, _ := collect(t, root, walk.Options{})
		got2, _ := collect(t, root, walk.Options{})
		want := []string{"a.txt", "b.txt", "sub", "sub/deep", "sub/deep/x", "sub/y.txt", "sub/z.txt"}
		if !slices.Equal(paths(got1), want) {
			t.Errorf("顺序 = %v, want %v", paths(got1), want)
		}
		if !slices.Equal(paths(got1), paths(got2)) {
			t.Error("两次遍历结果不一致")
		}
	})

	t.Run("TC-WALK-002 目录软链接不死循环", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"dir": testutil.Dir{
				"f":      testutil.File{Content: "x"},
				"self":   testutil.Symlink{Target: ".."},
				"parent": testutil.Symlink{Target: "../dir"},
			},
			"top": testutil.Symlink{Target: "dir"},
		})
		got, stats := collect(t, root, walk.Options{})
		want := []string{"dir", "dir/f", "dir/parent", "dir/self", "top"}
		if !slices.Equal(paths(got), want) {
			t.Errorf("顺序 = %v, want %v", paths(got), want)
		}
		for _, fi := range got {
			if fi.Path == "top" || fi.Path == "dir/self" || fi.Path == "dir/parent" {
				if fi.Type != fsmeta.TypeSymlink {
					t.Errorf("%s Type = %v, want 软链接", fi.Path, fi.Type)
				}
			}
		}
		if stats.Dirs != 1 {
			t.Errorf("Dirs = %d, want 1（软链接不算目录）", stats.Dirs)
		}
	})

	t.Run("TC-WALK-003 Exclude：目录仍进入，子项照常判定", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"sub": testutil.Dir{
				"keep.txt": testutil.File{Content: "k"},
				"drop.txt": testutil.File{Content: "d"},
			},
			"top.txt": testutil.File{Content: "t"},
		})
		f := &stubFilter{decisions: map[string]filter.Decision{
			"sub":          filter.Exclude,
			"sub/drop.txt": filter.Exclude,
		}}
		got, stats := collect(t, root, walk.Options{Filter: f})
		want := []string{"sub/keep.txt", "top.txt"}
		if !slices.Equal(paths(got), want) {
			t.Errorf("保留项 = %v, want %v", paths(got), want)
		}
		if stats.Skipped != 2 {
			t.Errorf("Skipped = %d, want 2", stats.Skipped)
		}
		// 被排除的目录本身不进入判定结果，但其子项照常交给 filter。
		if !slices.Equal(f.seen, []string{"sub", "sub/drop.txt", "sub/keep.txt", "top.txt"}) {
			t.Errorf("判定序列 = %v", f.seen)
		}
	})

	t.Run("TC-WALK-004 SkipDir：不进入也不计内部", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"skip": testutil.Dir{
				"a": testutil.File{Content: "a"},
				"b": testutil.File{Content: "b"},
			},
			"keep": testutil.File{Content: "k"},
		})
		f := &stubFilter{decisions: map[string]filter.Decision{"skip": filter.SkipDir}}
		got, stats := collect(t, root, walk.Options{Filter: f})
		if !slices.Equal(paths(got), []string{"keep"}) {
			t.Errorf("保留项 = %v", paths(got))
		}
		if stats.Skipped != 1 {
			t.Errorf("Skipped = %d, want 1（内部不计）", stats.Skipped)
		}
		if slices.Contains(f.seen, "skip/a") || slices.Contains(f.seen, "skip/b") {
			t.Error("SkipDir 的子项不应交给 filter 判定")
		}
	})

	t.Run("TC-WALK-005 硬链接：第二次出现指向第一次", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"a": testutil.File{Content: "hello"},
			"b": testutil.Hardlink{Target: "a"},
			"sub": testutil.Dir{
				"c": testutil.Hardlink{Target: "a"},
			},
		})
		got, stats := collect(t, root, walk.Options{})
		var a, b, c fsmeta.FileInfo
		for _, fi := range got {
			switch fi.Path {
			case "a":
				a = fi
			case "b":
				b = fi
			case "sub/c":
				c = fi
			}
		}
		if a.HardlinkTo != "" || a.Size != 5 {
			t.Errorf("第一次出现 a: %+v", a)
		}
		if b.HardlinkTo != "a" || b.Size != 0 || !b.IsHardlink() {
			t.Errorf("b 应为指向 a 的硬链接: %+v", b)
		}
		if c.HardlinkTo != "a" || c.Size != 0 {
			t.Errorf("c 应为指向 a 的硬链接: %+v", c)
		}
		if stats.Bytes != 5 {
			t.Errorf("Bytes = %d, want 5（硬链接不重复计）", stats.Bytes)
		}
	})

	t.Run("TC-WALK-006 硬链接：第一次被过滤后以下一次为准", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"a": testutil.File{Content: "hello"},
			"b": testutil.Hardlink{Target: "a"},
			"c": testutil.Hardlink{Target: "a"},
		})
		f := &stubFilter{decisions: map[string]filter.Decision{"a": filter.Exclude}}
		got, _ := collect(t, root, walk.Options{Filter: f})
		var b, c fsmeta.FileInfo
		for _, fi := range got {
			if fi.Path == "b" {
				b = fi
			}
			if fi.Path == "c" {
				c = fi
			}
		}
		if b.HardlinkTo != "" || b.Size != 5 {
			t.Errorf("a 被过滤后 b 是新的第一次出现: %+v", b)
		}
		if c.HardlinkTo != "b" {
			t.Errorf("c 应指向 b: %+v", c)
		}
	})

	t.Run("TC-WALK-016 非普通文件的硬链接作为独立条目", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"p1": testutil.FIFO{},
			"p2": testutil.Hardlink{Target: "p1"},
			"l1": testutil.Symlink{Target: "x"},
			"l2": testutil.Hardlink{Target: "l1"},
		})
		got, _ := collect(t, root, walk.Options{})
		for _, fi := range got {
			if fi.IsHardlink() {
				t.Errorf("%s（类型 %s）不应记为硬链接: HardlinkTo=%q", fi.Path, fi.Type, fi.HardlinkTo)
			}
		}
	})

	t.Run("TC-WALK-007 Stats 五个字段", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"f1":   testutil.File{Content: "12345"},
			"f2":   testutil.File{Size: 10},
			"ln":   testutil.Symlink{Target: "f1"},
			"pipe": testutil.FIFO{},
			"sub":  testutil.Dir{"g": testutil.File{Content: "g"}},
			"skip": testutil.Dir{"x": testutil.File{Content: "x"}},
		})
		f := &stubFilter{decisions: map[string]filter.Decision{"skip": filter.SkipDir}}
		_, stats := collect(t, root, walk.Options{Filter: f})
		// 保留非目录：f1 f2 ln pipe sub/g；目录：sub；字节：5+10+1=16；Skipped：skip。
		want := walk.Stats{Files: 5, Dirs: 1, Bytes: 16, Skipped: 1, Errors: 0}
		if stats != want {
			t.Errorf("Stats = %+v, want %+v", stats, want)
		}
	})

	t.Run("TC-WALK-008 OnError 跳过不可读目录并继续", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root 能读 chmod 000 的目录")
		}
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"blocked": testutil.Dir{"secret": testutil.File{Content: "s"}},
			"ok":      testutil.File{Content: "ok"},
		})
		if err := os.Chmod(filepath.Join(root, "blocked"), 0o000); err != nil {
			t.Fatal(err)
		}
		var seen []string
		got, stats := collect(t, root, walk.Options{
			OnError: func(rel string, err error) error {
				seen = append(seen, rel)
				return nil
			},
		})
		if !slices.Equal(paths(got), []string{"blocked", "ok"}) {
			t.Errorf("保留项 = %v（blocked 目录本身仍应输出）", paths(got))
		}
		if stats.Errors != 1 || !slices.Equal(seen, []string{"blocked"}) {
			t.Errorf("Errors = %d, seen = %v", stats.Errors, seen)
		}
	})

	t.Run("TC-WALK-009 OnError 返回非 nil 中止遍历", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root 能读 chmod 000 的目录")
		}
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"blocked": testutil.Dir{"x": testutil.File{Content: "x"}},
			"zz":      testutil.File{Content: "z"},
		})
		if err := os.Chmod(filepath.Join(root, "blocked"), 0o000); err != nil {
			t.Fatal(err)
		}
		sentinel := errors.New("停止")
		var calls int
		stats, err := walk.Walk(context.Background(), root, walk.Options{
			OnError: func(rel string, err error) error {
				calls++
				return sentinel
			},
		}, func(fi fsmeta.FileInfo) error { return nil })
		if !errors.Is(err, sentinel) {
			t.Errorf("err = %v, want %v", err, sentinel)
		}
		if calls != 1 {
			t.Errorf("OnError 调用 %d 次", calls)
		}
		if stats.Errors != 0 {
			t.Errorf("中止时不应计 Errors，得到 %d", stats.Errors)
		}
	})

	t.Run("TC-WALK-010 OnError 为 nil 时错误直接中止", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root 能读 chmod 000 的目录")
		}
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"blocked": testutil.Dir{"x": testutil.File{Content: "x"}},
		})
		if err := os.Chmod(filepath.Join(root, "blocked"), 0o000); err != nil {
			t.Fatal(err)
		}
		_, err := walk.Walk(context.Background(), root, walk.Options{},
			func(fi fsmeta.FileInfo) error { return nil })
		if err == nil {
			t.Error("无 OnError 时不可读目录应使 Walk 失败")
		}
	})

	t.Run("TC-WALK-011 ctx 取消", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"a": testutil.File{Content: "a"},
			"b": testutil.File{Content: "b"},
		})
		ctx, cancel := context.WithCancel(context.Background())
		var calls int
		_, err := walk.Walk(ctx, root, walk.Options{}, func(fi fsmeta.FileInfo) error {
			calls++
			cancel() // 第一个条目后取消
			return nil
		})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
		if calls != 1 {
			t.Errorf("fn 调用 %d 次, want 1", calls)
		}
	})

	t.Run("TC-WALK-012 fn 返回错误时中止并原样返回", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"a": testutil.File{Content: "a"},
			"b": testutil.File{Content: "b"},
		})
		sentinel := errors.New("写入失败")
		_, err := walk.Walk(context.Background(), root, walk.Options{},
			func(fi fsmeta.FileInfo) error { return sentinel })
		if !errors.Is(err, sentinel) {
			t.Errorf("err = %v, want %v", err, sentinel)
		}
	})

	t.Run("TC-WALK-013 root 不是目录时返回错误", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"file": testutil.File{Content: "x"},
		})
		_, err := walk.Walk(context.Background(), filepath.Join(root, "file"),
			walk.Options{}, func(fi fsmeta.FileInfo) error { return nil })
		if err == nil {
			t.Error("root 为普通文件应返回错误")
		}
	})

	t.Run("TC-WALK-014 root 不存在时返回错误", func(t *testing.T) {
		_, err := walk.Walk(context.Background(), filepath.Join(t.TempDir(), "nope"),
			walk.Options{}, func(fi fsmeta.FileInfo) error { return nil })
		if err == nil {
			t.Error("root 不存在应返回错误")
		}
	})

	t.Run("TC-WALK-015 Filter 为 nil 时全部保留", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"f": testutil.File{Content: "x"},
			"d": testutil.Dir{"g": testutil.File{Content: "y"}},
		})
		got, stats := collect(t, root, walk.Options{Filter: nil})
		if !slices.Equal(paths(got), []string{"d", "d/g", "f"}) {
			t.Errorf("保留项 = %v", paths(got))
		}
		if stats.Skipped != 0 {
			t.Errorf("Skipped = %d, want 0", stats.Skipped)
		}
	})
}
