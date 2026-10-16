//go:build linux

package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/container"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/pack"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/testutil"
)

// recorder 记录 engine 的全部进度与日志；hook 非 nil 时在每次进度报告后调用。
type recorder struct {
	progress []Progress
	logs     []logLine
	hook     func(Progress)
}

type logLine struct {
	level Level
	msg   string
}

func (r *recorder) Progress(p Progress) {
	r.progress = append(r.progress, p)
	if r.hook != nil {
		r.hook(p)
	}
}

func (r *recorder) Log(l Level, msg string) { r.logs = append(r.logs, logLine{l, msg}) }

// warns 返回全部 warn 日志。
func (r *recorder) warns() []string {
	var out []string
	for _, l := range r.logs {
		if l.level == LevelWarn {
			out = append(out, l.msg)
		}
	}
	return out
}

func (r *recorder) hasWarn(substr string) bool {
	for _, w := range r.warns() {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

// env 是一次往返测试的目录：src 源、repo 仓库、dst 还原目标（初始不存在）。
type env struct {
	src, repo, dst string
}

func newEnv(t *testing.T, tree testutil.Dir) env {
	t.Helper()
	base := t.TempDir()
	e := env{
		src:  filepath.Join(base, "src"),
		repo: filepath.Join(base, "repo"),
		dst:  filepath.Join(base, "dst"),
	}
	if err := os.Mkdir(e.src, 0o755); err != nil {
		t.Fatal(err)
	}
	testutil.CreateTree(t, e.src, tree)
	t.Cleanup(func() { makeWritable(e.dst) })
	return e
}

// makeWritable 恢复还原目录中只读目录的写权限，使 t.TempDir 能清理。
func makeWritable(root string) {
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = os.Chmod(p, 0o700)
		}
		return nil
	})
}

func (e env) backup(t *testing.T, opts BackupOptions) (BackupResult, *recorder) {
	t.Helper()
	opts.Source, opts.Repo = e.src, e.repo
	if opts.Name == "" {
		opts.Name = "snap"
	}
	rec := &recorder{}
	res, err := Backup(context.Background(), opts, rec)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	return res, rec
}

func (e env) restore(t *testing.T, backup string, password []byte) (RestoreResult, *recorder) {
	t.Helper()
	rec := &recorder{}
	res, err := Restore(context.Background(), RestoreOptions{Backup: backup, Target: e.dst, Password: password}, rec)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	return res, rec
}

// roundTrip 备份 → 还原 → 按五要素比较源与还原结果。
func (e env) roundTrip(t *testing.T, opts BackupOptions) (BackupResult, RestoreResult) {
	t.Helper()
	bres, _ := e.backup(t, opts)
	rres, _ := e.restore(t, bres.Path, opts.Password)
	testutil.AssertTreeEqual(t, e.src, e.dst)
	return bres, rres
}

// entry 是 craft 写入的一个条目。
type entry struct {
	fi   fsmeta.FileInfo
	data string
}

// craft 绕过 engine 直接用 pack + container 写一个未加密备份，用于构造异常归档。
func craft(t *testing.T, path string, entries []entry) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cw, err := container.NewWriter(f, container.Header{CreatedAt: time.Now(), Source: "/crafted"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	pw := pack.NewWriter(cw)
	var sum container.Summary
	for _, e := range entries {
		fi := e.fi
		if fi.Mode == 0 {
			fi.Mode = 0o644
		}
		if fi.Mtime.IsZero() {
			fi.Mtime, fi.Atime = testutil.DefaultTime, testutil.DefaultTime
		}
		fi.UID, fi.GID = uint32(os.Geteuid()), uint32(os.Getegid())
		if fi.Type == fsmeta.TypeRegular && !fi.IsHardlink() {
			fi.Size = int64(len(e.data))
		}
		if err := pw.WriteHeader(fi); err != nil {
			t.Fatalf("WriteHeader(%s): %v", fi.Path, err)
		}
		if _, err := pw.Write([]byte(e.data)); err != nil {
			t.Fatal(err)
		}
		sum.Files++
	}
	if err := pw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := cw.Finish(sum); err != nil {
		t.Fatal(err)
	}
}

// listDir 返回目录下的文件名（不存在时为 nil）。
func listDir(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	return names
}

func lstat(t *testing.T, p string) fsmeta.FileInfo {
	t.Helper()
	fi, err := fsmeta.Lstat(filepath.Dir(p), filepath.Base(p))
	if err != nil {
		t.Fatal(err)
	}
	return fi
}
