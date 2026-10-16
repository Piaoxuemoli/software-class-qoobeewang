//go:build linux

package fsmeta_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/testutil"
)

func lstat(t *testing.T, root, rel string) fsmeta.FileInfo {
	t.Helper()
	fi, err := fsmeta.Lstat(root, rel)
	if err != nil {
		t.Fatalf("Lstat(%q) err = %v", rel, err)
	}
	return fi
}

func TestLstat(t *testing.T) {
	t.Run("TC-FSM-001 普通文件", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"a.txt": testutil.File{Content: "hello", Mode: 0o640},
		})
		fi := lstat(t, root, "a.txt")
		if fi.Type != fsmeta.TypeRegular {
			t.Errorf("Type = %v, want 普通文件", fi.Type)
		}
		if fi.Path != "a.txt" || fi.Mode != 0o640 || fi.Size != 5 {
			t.Errorf("Path/Mode/Size = %q/%#o/%d", fi.Path, fi.Mode, fi.Size)
		}
		if fi.HardlinkTo != "" || fi.LinkTarget != "" {
			t.Errorf("普通文件不应有链接字段: %+v", fi)
		}
	})

	t.Run("TC-FSM-002 目录", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"sub": testutil.Dir{"x": testutil.File{Content: "x"}},
		})
		fi := lstat(t, root, "sub")
		if fi.Type != fsmeta.TypeDir {
			t.Errorf("Type = %v, want 目录", fi.Type)
		}
		if fi.Size != 0 {
			t.Errorf("目录 Size 应为 0，得到 %d", fi.Size)
		}
	})

	t.Run("TC-FSM-003 软链接记录目标且 Size 为 0", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"real.txt": testutil.File{Content: "data"},
			"ln":       testutil.Symlink{Target: "real.txt"},
			"abs":      testutil.Symlink{Target: "/nonexistent/xyz"},
		})
		fi := lstat(t, root, "ln")
		if fi.Type != fsmeta.TypeSymlink || fi.LinkTarget != "real.txt" {
			t.Errorf("Type/LinkTarget = %v/%q", fi.Type, fi.LinkTarget)
		}
		if fi.Size != 0 {
			t.Errorf("软链接 Size 应为 0，得到 %d", fi.Size)
		}
		if got := lstat(t, root, "abs").LinkTarget; got != "/nonexistent/xyz" {
			t.Errorf("悬空/绝对软链接目标应原样保存，得到 %q", got)
		}
	})

	t.Run("TC-FSM-004 FIFO", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{"pipe": testutil.FIFO{}})
		fi := lstat(t, root, "pipe")
		if fi.Type != fsmeta.TypeFIFO {
			t.Errorf("Type = %v, want FIFO", fi.Type)
		}
	})

	t.Run("TC-FSM-005 套接字", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{"sock": testutil.Socket{}})
		fi := lstat(t, root, "sock")
		if fi.Type != fsmeta.TypeSocket {
			t.Errorf("Type = %v, want 套接字", fi.Type)
		}
	})

	t.Run("TC-FSM-006 字符/块设备拆出主次设备号", func(t *testing.T) {
		if os.Geteuid() != 0 {
			t.Skip("需要 root 创建设备节点")
		}
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"chr": testutil.Device{Char: true, Major: 1, Minor: 7},
			"blk": testutil.Device{Char: false, Major: 259, Minor: 8},
		})
		fi := lstat(t, root, "chr")
		if fi.Type != fsmeta.TypeCharDevice || fi.DevMajor != 1 || fi.DevMinor != 7 {
			t.Errorf("字符设备 = %v %d:%d", fi.Type, fi.DevMajor, fi.DevMinor)
		}
		fi = lstat(t, root, "blk")
		if fi.Type != fsmeta.TypeBlockDevice || fi.DevMajor != 259 || fi.DevMinor != 8 {
			t.Errorf("块设备 = %v %d:%d", fi.Type, fi.DevMajor, fi.DevMinor)
		}
	})

	t.Run("TC-FSM-007 纳秒时间", func(t *testing.T) {
		root := t.TempDir()
		mt := time.Date(2020, 3, 4, 5, 6, 7, 123456789, time.UTC)
		testutil.CreateTree(t, root, testutil.Dir{
			"f": testutil.File{Content: "x", Mtime: mt, Atime: mt},
		})
		fi := lstat(t, root, "f")
		if !fi.Mtime.Equal(mt) {
			t.Errorf("Mtime = %v, want %v", fi.Mtime, mt)
		}
		if !fi.Atime.Equal(mt) {
			t.Errorf("Atime = %v, want %v", fi.Atime, mt)
		}
		if fi.Ctime.IsZero() {
			t.Error("Ctime 不应为零值")
		}
	})

	t.Run("TC-FSM-008 属主属组", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"mine":  testutil.File{Content: "x"},
			"owned": testutil.File{Content: "x", Owner: &testutil.Owner{UID: 0, GID: 0}},
		})
		fi := lstat(t, root, "mine")
		if fi.UID != uint32(os.Geteuid()) || fi.GID != uint32(os.Getegid()) {
			t.Errorf("UID/GID = %d/%d", fi.UID, fi.GID)
		}
		if got := lstat(t, root, "owned"); got.UID != 0 || got.GID != 0 {
			t.Errorf("owned UID/GID = %d/%d, want 0/0", got.UID, got.GID)
		}
	})

	t.Run("TC-FSM-009 statx 读取 Btime", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"f": testutil.File{Content: "x"},
		})
		fi := lstat(t, root, "f")
		if fi.Btime.IsZero() {
			t.Skip("文件系统不支持 statx btime")
		}
		if !fi.CreateTime().Equal(fi.Btime) {
			t.Errorf("有 Btime 时 CreateTime 应取 Btime，得到 %v", fi.CreateTime())
		}
	})

	t.Run("TC-FSM-010 Dev/Ino/Nlink 与硬链接", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"orig":  testutil.File{Content: "x"},
			"alias": testutil.Hardlink{Target: "orig"},
			"solo":  testutil.File{Content: "y"},
		})
		a := lstat(t, root, "orig")
		b := lstat(t, root, "alias")
		if a.Dev == 0 || a.Ino == 0 || a.Nlink != 2 {
			t.Errorf("orig Dev/Ino/Nlink = %d/%d/%d", a.Dev, a.Ino, a.Nlink)
		}
		if a.Dev != b.Dev || a.Ino != b.Ino {
			t.Error("硬链接应共享 (Dev, Ino)")
		}
		if s := lstat(t, root, "solo"); s.Nlink != 1 {
			t.Errorf("solo Nlink = %d, want 1", s.Nlink)
		}
	})

	t.Run("TC-FSM-011 权限位含 setuid/sticky", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"suid": testutil.File{Content: "x", Mode: 0o4750},
			"sty":  testutil.MetaDir{Entries: testutil.Dir{}, Mode: 0o1755},
		})
		if got := lstat(t, root, "suid").Mode; got != 0o4750 {
			t.Errorf("suid Mode = %#o, want 04750", got)
		}
		if got := lstat(t, root, "sty").Mode; got != 0o1755 {
			t.Errorf("sty Mode = %#o, want 01755", got)
		}
	})

	t.Run("TC-FSM-012 不存在的路径返回错误", func(t *testing.T) {
		root := t.TempDir()
		if _, err := fsmeta.Lstat(root, "nope"); err == nil {
			t.Error("Lstat 不存在路径应返回错误")
		}
	})

	t.Run("TC-FSM-013 嵌套相对路径", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"d1": testutil.Dir{"d2": testutil.Dir{"f": testutil.File{Content: "z"}}},
		})
		fi := lstat(t, root, "d1/d2/f")
		if fi.Path != "d1/d2/f" || fi.Type != fsmeta.TypeRegular {
			t.Errorf("Path/Type = %q/%v", fi.Path, fi.Type)
		}
	})
}

// TestLstatSymlinkNotFollowed 确认 Lstat 看的是链接本身而不是目标。
func TestLstatSymlinkNotFollowed(t *testing.T) {
	root := t.TempDir()
	testutil.CreateTree(t, root, testutil.Dir{
		"dir": testutil.Dir{"f": testutil.File{Content: "x"}},
		"dln": testutil.Symlink{Target: "dir"},
	})
	fi := lstat(t, root, "dln")
	if fi.Type != fsmeta.TypeSymlink {
		t.Errorf("指向目录的软链接 Type = %v, want 软链接", fi.Type)
	}
	var st syscall.Stat_t
	if err := syscall.Lstat(filepath.Join(root, "dln"), &st); err != nil {
		t.Fatal(err)
	}
	if uint64(st.Ino) != fi.Ino {
		t.Errorf("Ino = %d, want 链接本身的 inode %d", fi.Ino, st.Ino)
	}
}
