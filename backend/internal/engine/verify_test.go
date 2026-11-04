package engine

// Verify 的跨平台测试（无 build tag）：树只含普通文件与目录；Linux 专有类型
// （软链接、特殊文件等）由 verify_linux_test.go 覆盖。

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/container"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/pack"
)

// verifyRep 记录进度与日志（与 helpers_linux_test.go 的 recorder 分名，避免重声明）。
type verifyRep struct {
	progress []Progress
	warns    []string
}

func (r *verifyRep) Progress(p Progress) { r.progress = append(r.progress, p) }
func (r *verifyRep) Log(l Level, msg string) {
	if l == LevelWarn {
		r.warns = append(r.warns, msg)
	}
}

func vfWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// vfBackup 建一个含 3 文件 1 目录的源目录并备份，返回备份路径与仓库目录。
func vfBackup(t *testing.T, encryption container.Encryption, compress bool, password []byte) (string, string) {
	t.Helper()
	src := t.TempDir()
	vfWrite(t, filepath.Join(src, "a.txt"), "hello")
	vfWrite(t, filepath.Join(src, "sub", "b.txt"), "world")
	vfWrite(t, filepath.Join(src, "sub", "big.bin"), string(bytes.Repeat([]byte{0xAB}, 100_003)))
	repo := filepath.Join(t.TempDir(), "repo")
	opts := BackupOptions{Source: src, Repo: repo, Name: "v", Password: password, Encryption: encryption}
	if compress {
		opts.Compression = container.CompressionHuffman
	}
	res, err := Backup(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	return res.Path, repo
}

func TestVerifyOK(t *testing.T) {
	cases := []struct {
		name       string
		encryption container.Encryption
		compress   bool
		password   []byte
	}{
		{"TC-VFY-001 未加密不压缩", container.EncryptionNone, false, nil},
		{"TC-VFY-002 AES-256-CBC + Huffman", container.EncryptionAES256CBC, true, []byte("pw-aes")},
		{"TC-VFY-003 ChaCha20 + Huffman", container.EncryptionChaCha20, true, []byte("pw-cc")},
		{"TC-VFY-004 ChaCha20 不压缩", container.EncryptionChaCha20, false, []byte("pw-cc2")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path, _ := vfBackup(t, c.encryption, c.compress, c.password)
			rep := &verifyRep{}
			res, err := Verify(context.Background(), VerifyOptions{Backup: path, Password: c.password}, rep)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			// 3 个普通文件、1 个目录（sub）、100_003+10 字节。
			if res.Files != 3 || res.Dirs != 1 || res.Bytes != 100_013 {
				t.Fatalf("res = %+v", res)
			}
			if res.Compressed != c.compress || res.Encryption != c.encryption {
				t.Fatalf("res = %+v（compress=%v enc=%v）", res, c.compress, c.encryption)
			}
			if len(rep.warns) != 0 {
				t.Fatalf("不应有 warn：%v", rep.warns)
			}
			var last Progress
			sawVerify := false
			for _, p := range rep.progress {
				if p.Phase == PhaseVerify {
					sawVerify = true
					last = p
				}
			}
			if !sawVerify {
				t.Fatal("缺少 verify 阶段进度")
			}
			if !last.HasTotal || last.TotalFiles != 3 || last.TotalBytes != 100_013 ||
				last.Files != 3 || last.Bytes != 100_013 {
				t.Fatalf("最终进度 = %+v", last)
			}
		})
	}
}

func TestVerifyPassword(t *testing.T) {
	path, _ := vfBackup(t, container.EncryptionChaCha20, true, []byte("pw"))

	t.Run("TC-VFY-010 未给口令返回 ErrPasswordRequired", func(t *testing.T) {
		if _, err := Verify(context.Background(), VerifyOptions{Backup: path}, nil); !errors.Is(err, container.ErrPasswordRequired) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("TC-VFY-011 错误口令返回 ErrAuth", func(t *testing.T) {
		if _, err := Verify(context.Background(), VerifyOptions{Backup: path, Password: []byte("bad")}, nil); !errors.Is(err, container.ErrAuth) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("TC-VFY-012 未加密备份给口令发 warn 并忽略", func(t *testing.T) {
		plain, _ := vfBackup(t, container.EncryptionNone, false, nil)
		rep := &verifyRep{}
		if _, err := Verify(context.Background(), VerifyOptions{Backup: plain, Password: []byte("x")}, rep); err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if len(rep.warns) != 1 {
			t.Fatalf("warns = %v", rep.warns)
		}
	})
}

func TestVerifyCorrupt(t *testing.T) {
	path, _ := vfBackup(t, container.EncryptionAES256CBC, true, []byte("pw"))

	t.Run("TC-VFY-020 备份不存在", func(t *testing.T) {
		if _, err := Verify(context.Background(), VerifyOptions{Backup: "no-such.dbk", Password: []byte("pw")}, nil); !errors.Is(err, ErrBackupNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("TC-VFY-021 截断的备份", func(t *testing.T) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		f := filepath.Join(t.TempDir(), "cut.dbk")
		if err := os.WriteFile(f, data[:len(data)-40], 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Verify(context.Background(), VerifyOptions{Backup: f, Password: []byte("pw")}, nil); !errors.Is(err, container.ErrCorrupt) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("TC-VFY-022 篡改密文", func(t *testing.T) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		data[len(data)/2] ^= 1
		f := filepath.Join(t.TempDir(), "t.dbk")
		if err := os.WriteFile(f, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Verify(context.Background(), VerifyOptions{Backup: f, Password: []byte("pw")}, nil); !errors.Is(err, container.ErrAuth) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("TC-VFY-023 负载统计与尾部不符", func(t *testing.T) {
		// 直接构造一个合法容器：负载是"空 pack 流"（只有结尾零块），
		// 尾部却声称有 5 个文件——HMAC 通过、走查通过、交叉核对失败。
		f := filepath.Join(t.TempDir(), "mismatch.dbk")
		of, err := os.Create(f)
		if err != nil {
			t.Fatal(err)
		}
		cw, err := container.NewWriter(of, container.Header{
			Encryption: container.EncryptionAES256CBC, CreatedAt: time.Now(), Source: "/src", Iterations: 1000,
		}, []byte("pw"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := cw.Write(emptyPackStream(t)); err != nil {
			t.Fatal(err)
		}
		if err := cw.Finish(container.Summary{Files: 5, Dirs: 0, Bytes: 0}); err != nil {
			t.Fatal(err)
		}
		of.Close()
		_, err = Verify(context.Background(), VerifyOptions{Backup: f, Password: []byte("pw")}, nil)
		if !errors.Is(err, container.ErrCorrupt) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("TC-VFY-024 取消传播", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := Verify(ctx, VerifyOptions{Backup: path, Password: []byte("pw")}, nil); !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
	})
}

// emptyPackStream 生成只有结尾零块的 pack 流。
func emptyPackStream(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	pw := pack.NewWriter(&buf)
	if err := pw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
