//go:build windows

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/cli"
)

// Windows 进程内冒烟：backup → list → info → restore → delete 全链路。
// testscript 套件只在 Linux 跑（main_test.go 的构建标签），这里覆盖最小闭环。

func runCLI(args ...string) (int, string, string) {
	var out, err bytes.Buffer
	code := cli.Run(context.Background(), args, strings.NewReader(""), &out, &err)
	return code, out.String(), err.String()
}

func TestSmokeRoundtrip(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "src")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"a.txt": "hello", "中文.txt": "中文内容", filepath.Join("sub", "b.txt"): "sub",
	} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo := filepath.Join(base, "repo")
	dst := filepath.Join(base, "dst")
	t.Setenv("USERPROFILE", base) // 隔离默认仓库/配置（os.UserHomeDir 读 USERPROFILE）

	code, _, stderr := runCLI("backup", "--source", src, "--repo", repo, "--name", "w1")
	if code != 0 {
		t.Fatalf("backup: code=%d stderr=%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(repo, "w1.dbk")); err != nil {
		t.Fatalf("备份文件不存在: %v", err)
	}

	code, out, _ := runCLI("list", "--repo", repo)
	if code != 0 || !strings.Contains(out, "w1") {
		t.Fatalf("list: code=%d out=%q", code, out)
	}
	code, out, _ = runCLI("info", "w1", "--repo", repo)
	if code != 0 || !strings.Contains(out, "files: 3") {
		t.Fatalf("info: code=%d out=%q", code, out)
	}
	code, _, stderr = runCLI("restore", "--backup", "w1", "--target", dst, "--repo", repo)
	if code != 0 {
		t.Fatalf("restore: code=%d stderr=%s", code, stderr)
	}
	got, err := os.ReadFile(filepath.Join(dst, "中文.txt"))
	if err != nil || string(got) != "中文内容" {
		t.Fatalf("中文文件还原异常: %v %q", err, got)
	}
	code, out, _ = runCLI("delete", "w1", "--repo", repo)
	if code != 0 {
		t.Fatalf("delete: code=%d", code)
	}
	if _, err := os.Stat(filepath.Join(repo, "w1.dbk")); !os.IsNotExist(err) {
		t.Fatalf("删除后仍存在: %v", err)
	}
}

// 加密+压缩往返：错误口令必须 E_AUTH（退出码 3）。
func TestSmokeEncrypted(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "src")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, "s.txt"), []byte("secret-data-pad-pad-pad-pad"), 0o644)
	repo := filepath.Join(base, "repo")
	t.Setenv("USERPROFILE", base)

	var out, errB bytes.Buffer
	code := cli.Run(context.Background(),
		[]string{"backup", "--source", src, "--repo", repo, "--name", "enc",
			"--compress", "huffman", "--encrypt", "aes256", "--password-stdin"},
		strings.NewReader("pw123\n"), &out, &errB)
	if code != 0 {
		t.Fatalf("encrypted backup: code=%d stderr=%s", code, errB.String())
	}
	// 错误口令 → 退出码 3。
	out.Reset()
	errB.Reset()
	code = cli.Run(context.Background(),
		[]string{"restore", "--backup", "enc", "--target", filepath.Join(base, "bad"),
			"--repo", repo, "--password-stdin"},
		strings.NewReader("wrong\n"), &out, &errB)
	if code != 3 {
		t.Fatalf("错误口令 code=%d, want 3", code)
	}
	// 正确口令还原成功。
	out.Reset()
	errB.Reset()
	dst := filepath.Join(base, "ok")
	code = cli.Run(context.Background(),
		[]string{"restore", "--backup", "enc", "--target", dst,
			"--repo", repo, "--password-stdin"},
		strings.NewReader("pw123\n"), &out, &errB)
	if code != 0 {
		t.Fatalf("正确口令 restore: code=%d stderr=%s", code, errB.String())
	}
	got, _ := os.ReadFile(filepath.Join(dst, "s.txt"))
	if string(got) != "secret-data-pad-pad-pad-pad" {
		t.Fatalf("内容不符: %q", got)
	}
}
