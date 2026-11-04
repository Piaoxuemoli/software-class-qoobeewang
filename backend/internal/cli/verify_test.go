package cli

// verify 命令与 --encrypt chacha20 的进程内集成测试（跨平台）：真实走
// Run() → engine → 文件系统。口令经 stdin 传入。

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/jsonl"
)

// runWithInput 执行 CLI，stdin 为 in。
func runWithInput(t *testing.T, in string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = Run(context.Background(), args, strings.NewReader(in), &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func writeTree(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("world!"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// verifyCLIEncrypted 建源目录并用 --encrypt <算法> 备份，返回 CLI 参数与路径所需值。
func verifyCLIEncrypted(t *testing.T, algo string) (source, repo, name string) {
	t.Helper()
	source = t.TempDir()
	repo = filepath.Join(t.TempDir(), "repo")
	writeTree(t, source)
	code, _, stderr := runWithInput(t, "pw\n", "--json", "backup",
		"--source", source, "--repo", repo, "--name", "t1",
		"--compress", "huffman", "--encrypt", algo, "--password-stdin")
	if code != 0 {
		t.Fatalf("backup %s: code=%d stderr=%s", algo, code, stderr)
	}
	return source, repo, "t1"
}

func TestVerifyCommand(t *testing.T) {
	t.Run("TC-CLI-200 verify 全链路（ChaCha20 备份 → 验证 → 还原）", func(t *testing.T) {
		_, repo, name := verifyCLIEncrypted(t, "chacha20")

		code, stdout, stderr := runWithInput(t, "pw\n", "--json", "verify", name,
			"--repo", repo, "--password-stdin")
		if code != 0 {
			t.Fatalf("verify: code=%d stderr=%s", code, stderr)
		}
		var res jsonl.VerifyResult
		line := lastJSONLine(t, stdout)
		if err := json.Unmarshal([]byte(line), &res); err != nil {
			t.Fatalf("解析 %q: %v", line, err)
		}
		if res.Command != jsonl.CommandVerify || res.Files != 2 || res.Dirs != 1 || res.Bytes != 11 {
			t.Fatalf("res = %+v", res)
		}
		if !res.Compressed || res.Encryption != "chacha20" {
			t.Fatalf("res = %+v（期望 compressed=true, encryption=chacha20）", res)
		}

		// 还原核对内容一致，证明 verify 的"可还原"结论真实。
		dst := filepath.Join(t.TempDir(), "out")
		code, _, stderr = runWithInput(t, "pw\n", "--json", "restore",
			"--backup", name, "--target", dst, "--repo", repo, "--password-stdin")
		if code != 0 {
			t.Fatalf("restore: code=%d stderr=%s", code, stderr)
		}
		for rel, want := range map[string]string{"a.txt": "hello", filepath.Join("sub", "b.txt"): "world!"} {
			got, err := os.ReadFile(filepath.Join(dst, rel))
			if err != nil || string(got) != want {
				t.Fatalf("%s = %q, %v", rel, got, err)
			}
		}
	})

	t.Run("TC-CLI-201 verify 未给口令 → E_PASSWORD_REQUIRED（退出码 3）", func(t *testing.T) {
		_, repo, name := verifyCLIEncrypted(t, "aes256")
		code, stdout, _ := runWithInput(t, "", "--json", "verify", name, "--repo", repo)
		if code != 3 {
			t.Fatalf("code = %d, want 3", code)
		}
		var e jsonl.Error
		if err := json.Unmarshal([]byte(lastJSONLine(t, stdout)), &e); err != nil {
			t.Fatal(err)
		}
		if e.Code != jsonl.CodePasswordRequired {
			t.Fatalf("code = %s", e.Code)
		}
	})

	t.Run("TC-CLI-202 verify 错误口令 → E_AUTH", func(t *testing.T) {
		_, repo, name := verifyCLIEncrypted(t, "chacha20")
		code, stdout, _ := runWithInput(t, "wrong\n", "--json", "verify", name,
			"--repo", repo, "--password-stdin")
		if code != 3 {
			t.Fatalf("code = %d, want 3", code)
		}
		var e jsonl.Error
		if err := json.Unmarshal([]byte(lastJSONLine(t, stdout)), &e); err != nil {
			t.Fatal(err)
		}
		if e.Code != jsonl.CodeAuth {
			t.Fatalf("code = %s", e.Code)
		}
	})

	t.Run("TC-CLI-203 verify 不存在的备份 → E_NOT_FOUND", func(t *testing.T) {
		repo := t.TempDir()
		code, stdout, _ := runWithInput(t, "", "--json", "verify", "nope", "--repo", repo)
		if code != 1 {
			t.Fatalf("code = %d, want 1", code)
		}
		var e jsonl.Error
		if err := json.Unmarshal([]byte(lastJSONLine(t, stdout)), &e); err != nil {
			t.Fatal(err)
		}
		if e.Code != jsonl.CodeNotFound {
			t.Fatalf("code = %s", e.Code)
		}
	})

	t.Run("TC-CLI-204 未加密 verify 提供口令 → warn 后成功", func(t *testing.T) {
		source := t.TempDir()
		writeTree(t, source)
		repo := filepath.Join(t.TempDir(), "repo")
		code, _, stderr := runWithInput(t, "", "--json", "backup",
			"--source", source, "--repo", repo, "--name", "p1")
		if code != 0 {
			t.Fatalf("backup: %s", stderr)
		}
		code, stdout, _ := runWithInput(t, "pw\n", "--json", "verify", "p1",
			"--repo", repo, "--password-stdin")
		if code != 0 {
			t.Fatalf("verify code = %d", code)
		}
		var res jsonl.VerifyResult
		if err := json.Unmarshal([]byte(lastJSONLine(t, stdout)), &res); err != nil {
			t.Fatal(err)
		}
		if res.Encryption != "none" || res.Files != 2 || res.Dirs != 1 {
			t.Fatalf("res = %+v", res)
		}
	})

	t.Run("TC-CLI-205 --encrypt 非法取值 → E_USAGE", func(t *testing.T) {
		source := t.TempDir()
		code, _, stderr := runWithInput(t, "pw\n", "backup",
			"--source", source, "--encrypt", "rot13", "--password-stdin")
		if code != 2 {
			t.Fatalf("code = %d, want 2", code)
		}
		if !strings.Contains(stderr, "none|aes256|chacha20") {
			t.Fatalf("stderr = %q", stderr)
		}
	})

	t.Run("TC-CLI-206 --help 列出 verify 与 chacha20", func(t *testing.T) {
		code, stdout, _ := runWithInput(t, "", "backup", "--help")
		if code != 0 || !strings.Contains(stdout, "chacha20") {
			t.Fatalf("backup --help 缺少 chacha20")
		}
		code, stdout, _ = runWithInput(t, "", "verify", "--help")
		if code != 0 || !strings.Contains(stdout, "verify") {
			t.Fatalf("verify --help: %q", stdout)
		}
		code, stdout, _ = runWithInput(t, "", "--help")
		if code != 0 || !strings.Contains(stdout, "verify") {
			t.Fatalf("主 usage 缺少 verify")
		}
	})

	t.Run("TC-CLI-207 info 暴露算法名", func(t *testing.T) {
		_, repo, _ := verifyCLIEncrypted(t, "chacha20")
		code, stdout, _ := runWithInput(t, "", "--json", "info", "t1", "--repo", repo)
		if code != 0 {
			t.Fatalf("info code = %d", code)
		}
		var res jsonl.InfoResult
		if err := json.Unmarshal([]byte(lastJSONLine(t, stdout)), &res); err != nil {
			t.Fatal(err)
		}
		if res.Encryption != "chacha20" {
			t.Fatalf("encryption = %q", res.Encryption)
		}
	})
}

// lastJSONLine 返回 stdout 的最后一个非空行（result/error 事件）。
func lastJSONLine(t *testing.T, stdout string) string {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) == 0 {
		t.Fatalf("stdout 为空")
	}
	return lines[len(lines)-1]
}
