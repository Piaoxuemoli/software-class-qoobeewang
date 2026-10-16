package repo_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/container"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/repo"
)

// writeDBK 用 container.NewWriter 在 dir 下写一个真实的 .dbk 文件。
func writeDBK(t *testing.T, dir, name string, createdAt time.Time, source string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	cw, err := container.NewWriter(f, container.Header{
		Compression: container.CompressionHuffman,
		Encryption:  container.EncryptionNone,
		CreatedAt:   createdAt,
		Source:      source,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cw.Write([]byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := cw.Finish(container.Summary{Files: 3, Dirs: 1, Bytes: 7}); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func infoIsZero(i container.Info) bool {
	return reflect.DeepEqual(i, container.Info{})
}

func names(bs []repo.Backup) []string {
	n := make([]string, len(bs))
	for i, b := range bs {
		n[i] = b.Name
	}
	return n
}

func TestRepo(t *testing.T) {
	t.Run("TC-REPO-001 List 列出真实备份", func(t *testing.T) {
		dir := t.TempDir()
		writeDBK(t, dir, "a.dbk", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), "/src/a")
		r, err := repo.New(dir)
		if err != nil {
			t.Fatal(err)
		}
		bs, err := r.List()
		if err != nil {
			t.Fatal(err)
		}
		if len(bs) != 1 {
			t.Fatalf("len = %d", len(bs))
		}
		b := bs[0]
		if b.Corrupt || b.Name != "a" || b.Info.Header.Source != "/src/a" {
			t.Errorf("%+v", b)
		}
		if b.Info.Summary.Files != 3 || b.Info.Header.Compression != container.CompressionHuffman {
			t.Errorf("Info = %+v", b.Info)
		}
		if filepath.Base(b.Path) != "a.dbk" || !filepath.IsAbs(b.Path) {
			t.Errorf("Path = %q", b.Path)
		}
	})

	t.Run("TC-REPO-002 损坏与空文件以 Corrupt 列出且不影响其他", func(t *testing.T) {
		dir := t.TempDir()
		writeDBK(t, dir, "good.dbk", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), "/s")
		if err := os.WriteFile(filepath.Join(dir, "broken.dbk"), []byte("not a dbk"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "empty.dbk"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		r, _ := repo.New(dir)
		bs, err := r.List()
		if err != nil {
			t.Fatal(err)
		}
		if len(bs) != 3 {
			t.Fatalf("len = %d, want 3", len(bs))
		}
		byName := map[string]repo.Backup{}
		for _, b := range bs {
			byName[b.Name] = b
		}
		if byName["good"].Corrupt {
			t.Error("good 被误标为损坏")
		}
		for _, n := range []string{"broken", "empty"} {
			if !byName[n].Corrupt {
				t.Errorf("%s 应为 Corrupt", n)
			}
			if !infoIsZero(byName[n].Info) {
				t.Errorf("%s Info 应为零值", n)
			}
		}
	})

	t.Run("TC-REPO-003 忽略 .partial、子目录 x.dbk 与其他扩展名", func(t *testing.T) {
		dir := t.TempDir()
		writeDBK(t, dir, "a.dbk", time.Now(), "/s")
		for _, f := range []string{"b.dbk.partial", "c.txt", "d"} {
			if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Mkdir(filepath.Join(dir, "sub.dbk"), 0o755); err != nil {
			t.Fatal(err)
		}
		r, _ := repo.New(dir)
		bs, err := r.List()
		if err != nil {
			t.Fatal(err)
		}
		if len(bs) != 1 || bs[0].Name != "a" {
			t.Errorf("names = %v", names(bs))
		}
	})

	t.Run("TC-REPO-004 排序：时间降序，相同按名字升序，损坏按 ModTime", func(t *testing.T) {
		dir := t.TempDir()
		writeDBK(t, dir, "b.dbk", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "/s")
		writeDBK(t, dir, "a.dbk", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "/s") // 与 b 同一时间
		writeDBK(t, dir, "new.dbk", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), "/s")
		broken := filepath.Join(dir, "old.dbk")
		if err := os.WriteFile(broken, []byte("bad"), 0o600); err != nil {
			t.Fatal(err)
		}
		old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)
		if err := os.Chtimes(broken, old, old); err != nil {
			t.Fatal(err)
		}
		r, _ := repo.New(dir)
		bs, err := r.List()
		if err != nil {
			t.Fatal(err)
		}
		if got := names(bs); got[0] != "new" || got[1] != "a" || got[2] != "b" || got[3] != "old" {
			t.Errorf("names = %v, want [new a b old]", got)
		}
	})

	t.Run("TC-REPO-005 仓库目录不存在返回空切片", func(t *testing.T) {
		r, err := repo.New(filepath.Join(t.TempDir(), "nope"))
		if err != nil {
			t.Fatal(err)
		}
		bs, err := r.List()
		if err != nil {
			t.Fatal(err)
		}
		if bs == nil || len(bs) != 0 {
			t.Errorf("bs = %v, want 非 nil 空切片", bs)
		}
	})

	t.Run("TC-REPO-006 Resolve：名称、带后缀、路径三种", func(t *testing.T) {
		dir := t.TempDir()
		r, _ := repo.New(dir)
		got, err := r.Resolve("my")
		if err != nil || got != filepath.Join(dir, "my.dbk") {
			t.Errorf("Resolve(my) = %q, %v", got, err)
		}
		got, err = r.Resolve("my.dbk")
		if err != nil || got != filepath.Join(dir, "my.dbk") {
			t.Errorf("Resolve(my.dbk) = %q, %v", got, err)
		}
		p := "somewhere/x.dbk"
		got, err = r.Resolve(p)
		if err != nil {
			t.Fatal(err)
		}
		if !filepath.IsAbs(got) || filepath.Base(got) != "x.dbk" {
			t.Errorf("路径形式 Resolve = %q", got)
		}
	})

	t.Run("TC-REPO-007 Resolve 非法名称", func(t *testing.T) {
		r, _ := repo.New(t.TempDir())
		for _, bad := range []string{"", ".hidden", ".dbk", "a b\x00c"} {
			if _, err := r.Resolve(bad); !errors.Is(err, repo.ErrInvalidName) {
				t.Errorf("Resolve(%q) err = %v, want ErrInvalidName", bad, err)
			}
		}
	})

	t.Run("TC-REPO-008 Info：不存在与损坏", func(t *testing.T) {
		dir := t.TempDir()
		writeDBK(t, dir, "ok.dbk", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), "/src/x")
		if err := os.WriteFile(filepath.Join(dir, "bad.dbk"), []byte("junk"), 0o600); err != nil {
			t.Fatal(err)
		}
		r, _ := repo.New(dir)
		b, err := r.Info("ok")
		if err != nil || b.Name != "ok" || b.Info.Header.Source != "/src/x" {
			t.Errorf("Info(ok) = %+v, %v", b, err)
		}
		if _, err := r.Info("missing"); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("Info(missing) err = %v, want ErrNotFound", err)
		}
		b, err = r.Info("bad")
		if !errors.Is(err, container.ErrCorrupt) {
			t.Errorf("Info(bad) err = %v, want ErrCorrupt", err)
		}
		if !b.Corrupt {
			t.Error("Info(bad) 应返回 Corrupt=true 的 Backup")
		}
	})

	t.Run("TC-REPO-009 Delete 正常删除并返回信息", func(t *testing.T) {
		dir := t.TempDir()
		p := writeDBK(t, dir, "a.dbk", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), "/s")
		r, _ := repo.New(dir)
		b, err := r.Delete("a")
		if err != nil {
			t.Fatal(err)
		}
		if b.Name != "a" || b.Path != p {
			t.Errorf("Delete = %+v", b)
		}
		if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
			t.Error("文件未被删除")
		}
	})

	t.Run("TC-REPO-010 Delete：非 .dbk、不存在、目录、损坏备份", func(t *testing.T) {
		dir := t.TempDir()
		r, _ := repo.New(dir)
		// 非 .dbk 路径
		txt := filepath.Join(dir, "x.txt")
		if err := os.WriteFile(txt, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Delete(txt); !errors.Is(err, repo.ErrInvalidName) {
			t.Errorf("Delete(x.txt) err = %v, want ErrInvalidName", err)
		}
		// 不存在
		if _, err := r.Delete("nope"); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("Delete(nope) err = %v, want ErrNotFound", err)
		}
		// 名为 x.dbk 的目录
		if err := os.Mkdir(filepath.Join(dir, "d.dbk"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Delete("d.dbk"); !errors.Is(err, repo.ErrInvalidName) {
			t.Errorf("Delete(目录) err = %v, want ErrInvalidName", err)
		}
		// 损坏备份也能删除
		bad := filepath.Join(dir, "bad.dbk")
		if err := os.WriteFile(bad, []byte("junk"), 0o600); err != nil {
			t.Fatal(err)
		}
		b, err := r.Delete("bad")
		if err != nil {
			t.Fatalf("Delete(bad) err = %v", err)
		}
		if !b.Corrupt {
			t.Error("损坏备份删除后应返回 Corrupt=true")
		}
		if _, err := os.Stat(bad); !errors.Is(err, os.ErrNotExist) {
			t.Error("损坏文件未被删除")
		}
	})

	t.Run("TC-REPO-011 ValidateName 边界", func(t *testing.T) {
		ok200 := strings.Repeat("a", 200)
		if err := repo.ValidateName(ok200); err != nil {
			t.Errorf("200 字节应合法: %v", err)
		}
		bad := map[string]string{
			"201 字节": strings.Repeat("a", 201),
			"空":      "",
			"点开头":    ".name",
			"含斜杠":    "a/b",
			"含 NUL":  "a\x00b",
			"全是点":    "..",
		}
		for desc, n := range bad {
			if err := repo.ValidateName(n); !errors.Is(err, repo.ErrInvalidName) {
				t.Errorf("ValidateName(%s) err = %v, want ErrInvalidName", desc, err)
			}
		}
	})

	t.Run("TC-REPO-012 DefaultName", func(t *testing.T) {
		now := time.Date(2026, 10, 16, 8, 30, 45, 0, time.UTC)
		suffix := "-" + now.In(time.Local).Format("20060102-150405")
		cases := []struct{ in, base string }{
			{"/", "root"},
			{"/home/u/docs/", "docs"},
			{".hidden", "hidden"},
			{"..", "root"},
			{"docs", "docs"},
		}
		for _, c := range cases {
			got := repo.DefaultName(c.in, now)
			want := c.base + suffix
			if got != want {
				t.Errorf("DefaultName(%q) = %q, want %q", c.in, got, want)
			}
		}
		// 超长中文名：截短到 200 字节且不切断字符
		long := strings.Repeat("文", 100) // 300 字节
		got := repo.DefaultName(long, now)
		if len(got) > 200 {
			t.Errorf("超长名 %d 字节 > 200", len(got))
		}
		if err := repo.ValidateName(got); err != nil {
			t.Errorf("截短结果不合法: %v", err)
		}
		// 184 字节上限（200-16），61 个"文"=183 字节，第 62 个会被切断 → 截到 183
		if !strings.HasPrefix(got, strings.Repeat("文", 61)+"-") {
			t.Errorf("截短边界错误: %q...", got[:40])
		}
	})

	t.Run("TC-REPO-013 New 把 dir 转为绝对路径且不要求存在", func(t *testing.T) {
		r, err := repo.New("relative/dir")
		if err != nil {
			t.Fatal(err)
		}
		if !filepath.IsAbs(r.Dir) {
			t.Errorf("Dir = %q, want 绝对路径", r.Dir)
		}
	})
}
