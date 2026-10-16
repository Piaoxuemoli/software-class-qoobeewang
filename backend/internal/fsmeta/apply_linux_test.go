//go:build linux

package fsmeta_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/testutil"
)

func TestCreateNode(t *testing.T) {
	for _, tc := range []struct {
		id  string
		typ fsmeta.FileType
	}{
		{"TC-FSM-020 FIFO", fsmeta.TypeFIFO},
		{"TC-FSM-021 套接字", fsmeta.TypeSocket},
	} {
		t.Run(tc.id, func(t *testing.T) {
			root := t.TempDir()
			if err := fsmeta.CreateNode(filepath.Join(root, "n"), fsmeta.FileInfo{Type: tc.typ}); err != nil {
				t.Fatalf("CreateNode: %v", err)
			}
			if got := lstat(t, root, "n").Type; got != tc.typ {
				t.Errorf("Type = %v, want %v", got, tc.typ)
			}
		})
	}

	t.Run("TC-FSM-022 字符/块设备保留主次设备号", func(t *testing.T) {
		if os.Geteuid() != 0 {
			t.Skip("需要 root")
		}
		root := t.TempDir()
		devs := []fsmeta.FileInfo{
			{Path: "c", Type: fsmeta.TypeCharDevice, DevMajor: 1, DevMinor: 3},
			{Path: "b", Type: fsmeta.TypeBlockDevice, DevMajor: 259, DevMinor: 300},
		}
		for _, want := range devs {
			if err := fsmeta.CreateNode(filepath.Join(root, want.Path), want); err != nil {
				t.Fatalf("CreateNode(%s): %v", want.Path, err)
			}
			got := lstat(t, root, want.Path)
			if got.Type != want.Type || got.DevMajor != want.DevMajor || got.DevMinor != want.DevMinor {
				t.Errorf("%s: got %v %d:%d, want %v %d:%d", want.Path,
					got.Type, got.DevMajor, got.DevMinor, want.Type, want.DevMajor, want.DevMinor)
			}
		}
	})

	t.Run("TC-FSM-023 非特殊节点类型被拒绝", func(t *testing.T) {
		root := t.TempDir()
		for _, typ := range []fsmeta.FileType{fsmeta.TypeRegular, fsmeta.TypeDir, fsmeta.TypeSymlink} {
			if err := fsmeta.CreateNode(filepath.Join(root, "x"), fsmeta.FileInfo{Type: typ}); err == nil {
				t.Errorf("CreateNode(%v) 应返回错误", typ)
			}
		}
	})

	t.Run("TC-FSM-024 已存在时返回错误", func(t *testing.T) {
		root := t.TempDir()
		p := filepath.Join(root, "n")
		fi := fsmeta.FileInfo{Type: fsmeta.TypeFIFO}
		if err := fsmeta.CreateNode(p, fi); err != nil {
			t.Fatal(err)
		}
		if err := fsmeta.CreateNode(p, fi); !os.IsExist(err) {
			t.Errorf("err = %v, want EEXIST", err)
		}
	})
}

func TestApplyMetadata(t *testing.T) {
	atime := time.Date(2020, 5, 6, 7, 8, 9, 111222333, time.UTC)
	mtime := time.Date(2021, 6, 7, 8, 9, 10, 444555666, time.UTC)

	t.Run("TC-FSM-030 普通文件权限（含 setuid/sticky）与纳秒时间", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{"f": testutil.File{Content: "x"}})
		want := fsmeta.FileInfo{Type: fsmeta.TypeRegular, Mode: 0o4751 | 0o1000, Atime: atime, Mtime: mtime}
		if err := fsmeta.ApplyMetadata(filepath.Join(root, "f"), want, fsmeta.ApplyOptions{}); err != nil {
			t.Fatal(err)
		}
		got := lstat(t, root, "f")
		if got.Mode != want.Mode {
			t.Errorf("Mode = %#o, want %#o", got.Mode, want.Mode)
		}
		if !got.Atime.Equal(atime) || !got.Mtime.Equal(mtime) {
			t.Errorf("atime/mtime = %v/%v, want %v/%v", got.Atime, got.Mtime, atime, mtime)
		}
	})

	t.Run("TC-FSM-031 目录只读权限与时间", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{"d": testutil.Dir{"x": testutil.File{}}})
		want := fsmeta.FileInfo{Type: fsmeta.TypeDir, Mode: 0o555, Atime: atime, Mtime: mtime}
		if err := fsmeta.ApplyMetadata(filepath.Join(root, "d"), want, fsmeta.ApplyOptions{}); err != nil {
			t.Fatal(err)
		}
		got := lstat(t, root, "d")
		if got.Mode != 0o555 || !got.Mtime.Equal(mtime) {
			t.Errorf("Mode/Mtime = %#o/%v", got.Mode, got.Mtime)
		}
	})

	t.Run("TC-FSM-032 软链接设置链接本身的时间、不改目标", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"target": testutil.File{Content: "t", Mode: 0o600},
			"ln":     testutil.Symlink{Target: "target"},
		})
		before := lstat(t, root, "target")
		want := fsmeta.FileInfo{Type: fsmeta.TypeSymlink, Mode: 0o777, Atime: atime, Mtime: mtime, LinkTarget: "target"}
		if err := fsmeta.ApplyMetadata(filepath.Join(root, "ln"), want, fsmeta.ApplyOptions{}); err != nil {
			t.Fatal(err)
		}
		if got := lstat(t, root, "ln"); !got.Mtime.Equal(mtime) {
			t.Errorf("链接 mtime = %v, want %v", got.Mtime, mtime)
		}
		after := lstat(t, root, "target")
		if after.Mode != 0o600 || !after.Mtime.Equal(before.Mtime) {
			t.Errorf("目标被改动: mode %#o mtime %v → %v", after.Mode, before.Mtime, after.Mtime)
		}
	})

	t.Run("TC-FSM-033 悬空软链接也能设置时间", func(t *testing.T) {
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{"ln": testutil.Symlink{Target: "/nonexistent/x"}})
		want := fsmeta.FileInfo{Type: fsmeta.TypeSymlink, Atime: atime, Mtime: mtime}
		if err := fsmeta.ApplyMetadata(filepath.Join(root, "ln"), want, fsmeta.ApplyOptions{}); err != nil {
			t.Fatal(err)
		}
		if got := lstat(t, root, "ln"); !got.Mtime.Equal(mtime) {
			t.Errorf("mtime = %v, want %v", got.Mtime, mtime)
		}
	})

	t.Run("TC-FSM-034 root 恢复属主且 chown 不清掉 setgid", func(t *testing.T) {
		if os.Geteuid() != 0 {
			t.Skip("需要 root")
		}
		root := t.TempDir()
		testutil.CreateTree(t, root, testutil.Dir{
			"f":  testutil.File{Content: "x"},
			"ln": testutil.Symlink{Target: "f"},
		})
		want := fsmeta.FileInfo{Type: fsmeta.TypeRegular, Mode: 0o2755, UID: 1234, GID: 5678, Atime: atime, Mtime: mtime}
		if err := fsmeta.ApplyMetadata(filepath.Join(root, "f"), want, fsmeta.ApplyOptions{Chown: true}); err != nil {
			t.Fatal(err)
		}
		got := lstat(t, root, "f")
		if got.UID != 1234 || got.GID != 5678 || got.Mode != 0o2755 {
			t.Errorf("UID/GID/Mode = %d/%d/%#o", got.UID, got.GID, got.Mode)
		}
		ln := fsmeta.FileInfo{Type: fsmeta.TypeSymlink, UID: 4321, GID: 8765, Atime: atime, Mtime: mtime}
		if err := fsmeta.ApplyMetadata(filepath.Join(root, "ln"), ln, fsmeta.ApplyOptions{Chown: true}); err != nil {
			t.Fatal(err)
		}
		if got := lstat(t, root, "ln"); got.UID != 4321 || got.GID != 8765 {
			t.Errorf("软链接属主 = %d:%d，应为 lchown 后的 4321:8765", got.UID, got.GID)
		}
		if got := lstat(t, root, "f"); got.UID != 1234 {
			t.Errorf("lchown 跟随了软链接：目标属主变为 %d", got.UID)
		}
	})

	t.Run("TC-FSM-035 路径不存在返回错误", func(t *testing.T) {
		err := fsmeta.ApplyMetadata(filepath.Join(t.TempDir(), "missing"),
			fsmeta.FileInfo{Type: fsmeta.TypeRegular, Mode: 0o644}, fsmeta.ApplyOptions{})
		if !os.IsNotExist(err) {
			t.Errorf("err = %v, want ENOENT", err)
		}
	})
}
