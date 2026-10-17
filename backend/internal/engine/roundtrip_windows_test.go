//go:build windows

package engine

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/container"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/pack"
)

// Windows 冒烟：不依赖 testutil（仅 Linux），手工断言 regular/dir 往返、
// 只读位与时间戳恢复、特殊文件静默跳过（含本地日志）。

func TestBackupRestoreRoundtrip(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "a.txt"), "hello", 0o644)
	writeFile(t, filepath.Join(src, "ro.txt"), "locked", 0o444)
	writeFile(t, filepath.Join(src, "sub", "中文文件.txt"), "内容", 0o644)
	repo := filepath.Join(t.TempDir(), "repo")
	dst := filepath.Join(t.TempDir(), "out")

	ctx := context.Background()
	bres, err := Backup(ctx, BackupOptions{Source: src, Repo: repo, Name: "w1"}, nil)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if bres.Files != 3 || bres.Dirs != 1 {
		t.Errorf("Files=%d Dirs=%d, want 3/1", bres.Files, bres.Dirs)
	}
	if _, err := os.Stat(filepath.Join(repo, "w1.dbk")); err != nil {
		t.Fatalf("备份文件不存在: %v", err)
	}
	rres, err := Restore(ctx, RestoreOptions{Backup: bres.Path, Target: dst}, nil)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if rres.Files != 3 || rres.Dirs != 1 || rres.Warnings != 0 {
		t.Errorf("Restore=%+v, want Files=3 Dirs=1 Warnings=0", rres)
	}
	for _, rel := range []string{"a.txt", "ro.txt", filepath.Join("sub", "中文文件.txt")} {
		got, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Fatalf("还原缺失 %s: %v", rel, err)
		}
		want, _ := os.ReadFile(filepath.Join(src, rel))
		if string(got) != string(want) {
			t.Errorf("%s 内容 %q, want %q", rel, got, want)
		}
	}
	if st, _ := os.Stat(filepath.Join(dst, "ro.txt")); st.Mode().Perm() != 0o444 {
		t.Errorf("ro.txt 还原后 Perm=%o, want 444", st.Mode().Perm())
	}
}

// 特殊文件（FIFO/设备/套接字）在 Windows 上必须静默跳过：
// 返回成功、不产生 warn、不写文件，只记本地日志。
func TestRestoreSkipsSpecialFiles(t *testing.T) {
	skipLog := filepath.Join(t.TempDir(), "skip.log")
	t.Setenv("DATABACKUP_SKIPLOG", skipLog)

	// 手工构造含 FIFO 头记录的 .dbk（pack → container，明文不压缩）。
	dbk := filepath.Join(t.TempDir(), "sp.dbk")
	f, err := os.Create(dbk)
	if err != nil {
		t.Fatal(err)
	}
	bw := bufio.NewWriter(f)
	cw, err := container.NewWriter(bw, container.Header{
		Version: container.FormatVersion, CreatedAt: time.Now(), Source: "linux",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	pw := pack.NewWriter(cw)
	for _, fi := range []fsmeta.FileInfo{
		{Path: "d", Type: fsmeta.TypeDir, Mode: 0o755},
		{Path: "d/pipe", Type: fsmeta.TypeFIFO, Mode: 0o644},
		{Path: "d/cdev", Type: fsmeta.TypeCharDevice, Mode: 0o640, DevMajor: 1, DevMinor: 3},
		{Path: "d/sock", Type: fsmeta.TypeSocket, Mode: 0o644},
		{Path: "ok.txt", Type: fsmeta.TypeRegular, Mode: 0o644, Size: 4},
	} {
		if err := pw.WriteHeader(fi); err != nil {
			t.Fatal(err)
		}
		if fi.Type == fsmeta.TypeRegular {
			if _, err := pw.Write([]byte("data")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := pw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := cw.Finish(container.Summary{Files: 4, Dirs: 1, Bytes: 4}); err != nil {
		t.Fatal(err)
	}
	if err := bw.Flush(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	var logs []string
	rep := captureReporter{logs: &logs}
	dst := filepath.Join(t.TempDir(), "out")
	res, err := Restore(context.Background(), RestoreOptions{Backup: dbk, Target: dst}, rep)
	if err != nil {
		t.Fatalf("Restore 应静默跳过特殊文件: %v", err)
	}
	if res.Files != 1 || res.Dirs != 1 {
		t.Errorf("Files=%d Dirs=%d, want 1/1（特殊文件不计数）", res.Files, res.Dirs)
	}
	if res.Warnings != 0 {
		t.Errorf("Warnings=%d, want 0（静默跳过不发 warn）", res.Warnings)
	}
	for _, rel := range []string{"d/pipe", "d/cdev", "d/sock"} {
		if _, err := os.Lstat(filepath.Join(dst, rel)); err == nil {
			t.Errorf("%s 不应被创建", rel)
		}
	}
	data, err := os.ReadFile(filepath.Join(dst, "ok.txt"))
	if err != nil || string(data) != "data" {
		t.Errorf("ok.txt 还原异常: %v %q", err, data)
	}
	logData, err := os.ReadFile(skipLog)
	if err != nil {
		t.Fatalf("跳过日志未写入 %s: %v", skipLog, err)
	}
	for _, want := range []string{"d/pipe", "d/cdev", "d/sock"} {
		if !strings.Contains(string(logData), want) {
			t.Errorf("跳过日志缺少 %s 的记录:\n%s", want, logData)
		}
	}
}

func writeFile(t *testing.T, path, content string, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatal(err)
	}
	if perm != 0o644 {
		if err := os.Chmod(path, perm); err != nil {
			t.Fatal(err)
		}
	}
}

type captureReporter struct {
	logs *[]string
}

func (captureReporter) Progress(Progress) {}
func (r captureReporter) Log(l Level, msg string) {
	if l == LevelWarn {
		*r.logs = append(*r.logs, msg)
	}
}
