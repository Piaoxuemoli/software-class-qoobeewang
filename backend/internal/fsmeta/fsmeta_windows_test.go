//go:build windows

package fsmeta

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// 夹具不依赖 testutil（仅 Linux 可用）：直接用 os 创建常规条目。
// 覆盖：常规文件/目录/只读位/时间戳/软链接（无权限时跳过）/junction 归类。

func TestLstatRegular(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "a.txt")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	fi, err := Lstat(root, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if fi.Type != TypeRegular || fi.Size != 5 {
		t.Errorf("Type=%v Size=%d, want Regular/5", fi.Type, fi.Size)
	}
	if fi.Mode != 0o666 {
		t.Errorf("Mode=%o, want 666", fi.Mode)
	}
	if fi.UID != 0 || fi.GID != 0 {
		t.Errorf("UID/GID=%d/%d, want 0/0", fi.UID, fi.GID)
	}
	if fi.Btime.IsZero() {
		t.Error("Btime 应取 NTFS 创建时间，得到零值")
	}
	if fi.Mtime.IsZero() || time.Since(fi.Mtime) > time.Hour {
		t.Errorf("Mtime 异常: %v", fi.Mtime)
	}
	// Nlink 经 GetFileInformationByHandle 取得 ≥1；失败时兜底为 1。
	if fi.Nlink < 1 {
		t.Errorf("Nlink=%d, want >=1", fi.Nlink)
	}
}

func TestLstatReadonly(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "ro.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o444); err != nil {
		t.Fatal(err)
	}
	fi, err := Lstat(root, "ro.txt")
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode != 0o444 {
		t.Errorf("只读文件 Mode=%o, want 444", fi.Mode)
	}
}

func TestLstatDir(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	fi, err := Lstat(root, "sub")
	if err != nil {
		t.Fatal(err)
	}
	if fi.Type != TypeDir || fi.Mode != 0o755 {
		t.Errorf("Type=%v Mode=%o, want Dir/755", fi.Type, fi.Mode)
	}
}

func TestLstatSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "target.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.txt", filepath.Join(root, "link")); err != nil {
		t.Skipf("创建软链接需要开发者模式/管理员权限: %v", err)
	}
	fi, err := Lstat(root, "link")
	if err != nil {
		t.Fatal(err)
	}
	if fi.Type != TypeSymlink || fi.LinkTarget != "target.txt" {
		t.Errorf("Type=%v LinkTarget=%q, want Symlink/target.txt", fi.Type, fi.LinkTarget)
	}
}

// junction（目录型 reparse point）必须归类为软链接而非目录——
// 否则自指 junction 会让 walk 无限递归。
func TestLstatJunctionIsSymlink(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "jdir")
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, real).CombinedOutput()
	if err != nil {
		t.Skipf("mklink /J 失败: %v (%s)", err, out)
	}
	fi, err := Lstat(root, "jdir")
	if err != nil {
		t.Fatal(err)
	}
	if fi.Type != TypeSymlink {
		t.Errorf("junction Type=%v, want Symlink（当目录递归会死循环）", fi.Type)
	}
	if fi.LinkTarget == "" {
		t.Error("junction LinkTarget 为空，期望能读到目标")
	}
}

func TestHardlinkDetection(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "orig.txt")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(src, filepath.Join(root, "alias.txt")); err != nil {
		t.Skipf("创建硬链接失败（非 NTFS？）: %v", err)
	}
	fi, err := Lstat(root, "alias.txt")
	if err != nil {
		t.Fatal(err)
	}
	if fi.Nlink != 2 {
		t.Errorf("Nlink=%d, want 2", fi.Nlink)
	}
	orig, _ := Lstat(root, "orig.txt")
	if fi.Dev != orig.Dev || fi.Ino != orig.Ino {
		t.Errorf("硬链接 (Dev,Ino) 不一致: (%d,%d) vs (%d,%d)", fi.Dev, fi.Ino, orig.Dev, orig.Ino)
	}
}

func TestApplyMetadataWindows(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "m.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mtime := time.Date(2020, 3, 4, 5, 6, 7, 0, time.Local)
	atime := time.Date(2021, 4, 5, 6, 7, 8, 0, time.Local)
	fi := FileInfo{Path: "m.txt", Type: TypeRegular, Mode: 0o444, Atime: atime, Mtime: mtime}
	if err := ApplyMetadata(p, fi, ApplyOptions{Chown: true}); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if !st.ModTime().Equal(mtime) {
		t.Errorf("Mtime=%v, want %v", st.ModTime(), mtime)
	}
	if st.Mode().Perm() != 0o444 {
		t.Errorf("Perm=%o, want 444（只读）", st.Mode().Perm())
	}
	// 软链接条目的 ApplyMetadata 必须是 no-op。
	if err := ApplyMetadata(p, FileInfo{Type: TypeSymlink}, ApplyOptions{}); err != nil {
		t.Errorf("软链接 ApplyMetadata 应为 nil, got %v", err)
	}
}

func TestCreateNodeUnsupported(t *testing.T) {
	if err := CreateNode(filepath.Join(t.TempDir(), "p"), FileInfo{Type: TypeFIFO}); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("CreateNode 应返回 ErrUnsupported, got %v", err)
	}
}
