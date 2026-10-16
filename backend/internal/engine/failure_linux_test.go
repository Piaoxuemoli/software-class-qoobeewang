//go:build linux

package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/container"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/filter"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/pack"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/repo"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/testutil"
)

func TestBackupErrors(t *testing.T) {
	t.Run("TC-ENG-020 源目录不存在 / 不是目录", func(t *testing.T) {
		e := newEnv(t, testutil.Dir{"f": testutil.File{}})
		_, err := Backup(t.Context(), BackupOptions{Source: filepath.Join(e.src, "nope"), Repo: e.repo}, nil)
		if !errors.Is(err, ErrSourceNotFound) {
			t.Errorf("不存在: err = %v", err)
		}
		_, err = Backup(t.Context(), BackupOptions{Source: filepath.Join(e.src, "f"), Repo: e.repo}, nil)
		if !errors.Is(err, ErrSourceNotDir) {
			t.Errorf("不是目录: err = %v", err)
		}
		if _, err := DryRun(t.Context(), BackupOptions{Source: filepath.Join(e.src, "nope")}, nil); !errors.Is(err, ErrSourceNotFound) {
			t.Errorf("DryRun 不存在: err = %v", err)
		}
		if listDir(t, e.repo) != nil {
			t.Error("失败的备份不应创建仓库目录")
		}
	})

	t.Run("TC-ENG-021 同名备份已存在 → ErrExists，原备份不变", func(t *testing.T) {
		e := newEnv(t, testutil.Dir{"f": testutil.File{Content: "1"}})
		first, _ := e.backup(t, BackupOptions{})
		before := readInfo(t, first.Path)
		_, err := Backup(t.Context(), BackupOptions{Source: e.src, Repo: e.repo, Name: "snap"}, nil)
		if !errors.Is(err, ErrExists) {
			t.Fatalf("err = %v, want ErrExists", err)
		}
		if after := readInfo(t, first.Path); after.Size != before.Size || !slices.Equal(listDir(t, e.repo), []string{"snap.dbk"}) {
			t.Errorf("原备份被改动或有残留: %v", listDir(t, e.repo))
		}
	})

	t.Run("TC-ENG-022 非法备份名 → repo.ErrInvalidName", func(t *testing.T) {
		e := newEnv(t, testutil.Dir{})
		for _, name := range []string{".hidden", "a/b"} {
			if _, err := Backup(t.Context(), BackupOptions{Source: e.src, Repo: e.repo, Name: name}, nil); !errors.Is(err, repo.ErrInvalidName) {
				t.Errorf("%q: err = %v", name, err)
			}
		}
	})

	t.Run("TC-ENG-023 排除目录：相对、带斜杠、源内绝对路径均生效；源外 → ErrInvalidOptions", func(t *testing.T) {
		e := newEnv(t, testutil.Dir{
			"keep": testutil.Dir{"k": testutil.File{}},
			"a":    testutil.Dir{"x": testutil.File{}},
			"b":    testutil.Dir{"y": testutil.File{}},
			"c":    testutil.Dir{"d": testutil.Dir{"z": testutil.File{}}},
		})
		res, err := DryRun(t.Context(), BackupOptions{Source: e.src, Criteria: filter.Criteria{
			ExcludeDirs: []string{"a", "b/", filepath.Join(e.src, "c", "d")},
		}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		// 保留：keep、keep/k、c；跳过：a、b、c/d（各计 1）。
		if res.Files != 1 || res.Dirs != 2 || res.Skipped != 3 {
			t.Errorf("DryRun = %+v", res)
		}
		for _, bad := range []string{"/elsewhere", e.src, "..", "../src/a", "."} {
			_, err := DryRun(t.Context(), BackupOptions{Source: e.src, Criteria: filter.Criteria{ExcludeDirs: []string{bad}}}, nil)
			if !errors.Is(err, ErrInvalidOptions) {
				t.Errorf("%q: err = %v, want ErrInvalidOptions", bad, err)
			}
		}
	})

	t.Run("TC-ENG-024 仓库位于源目录内时自动排除；仓库即源目录 → ErrInvalidOptions", func(t *testing.T) {
		e := newEnv(t, testutil.Dir{"f": testutil.File{Content: "x"}})
		inner := filepath.Join(e.src, "backups")
		res, err := Backup(t.Context(), BackupOptions{Source: e.src, Repo: inner, Name: "one"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		res2, err := Backup(t.Context(), BackupOptions{Source: e.src, Repo: inner, Name: "two"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if res.Files != 1 || res2.Files != 1 || res2.Dirs != 0 {
			t.Errorf("仓库目录被备份进去了: %+v / %+v", res, res2)
		}
		if _, err := Backup(t.Context(), BackupOptions{Source: e.src, Repo: e.src, Name: "x"}, nil); !errors.Is(err, ErrInvalidOptions) {
			t.Errorf("repo == source: err = %v", err)
		}
	})

	t.Run("TC-ENG-025 源目录是指向目录的软链接", func(t *testing.T) {
		e := newEnv(t, testutil.Dir{"f": testutil.File{Content: "x"}})
		ln := filepath.Join(filepath.Dir(e.src), "srclink")
		if err := os.Symlink(e.src, ln); err != nil {
			t.Fatal(err)
		}
		res, err := Backup(t.Context(), BackupOptions{Source: ln, Repo: e.repo, Name: "n"}, nil)
		if err != nil || res.Files != 1 {
			t.Fatalf("res = %+v, err = %v", res, err)
		}
		if got := readInfo(t, res.Path).Header.Source; got != ln {
			t.Errorf("Source = %q, want %q", got, ln)
		}
	})

	t.Run("TC-ENG-026 无法读取的文件发 warn 并跳过，备份仍成功（非 root）", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root 可以读取任何文件")
		}
		e := newEnv(t, testutil.Dir{
			"ok":     testutil.File{Content: "ok"},
			"secret": testutil.File{Content: "s"},
			"h":      testutil.Hardlink{Target: "secret"},
			"closed": testutil.Dir{"in": testutil.File{}},
		})
		// testutil 的 Mode 0 表示默认权限，0000 只能事后设置。
		for _, p := range []string{"secret", "closed"} {
			if err := os.Chmod(filepath.Join(e.src, p), 0); err != nil {
				t.Fatal(err)
			}
		}
		res, rec := e.backup(t, BackupOptions{})
		// closed 目录本身可以 lstat，内容读不出：closed 计 1 个目录，secret 与 h 跳过。
		if res.Files != 1 || res.Dirs != 1 || res.Warnings != 3 {
			t.Errorf("res = %+v, warns = %v", res, rec.warns())
		}
	})

	t.Run("TC-ENG-027 dry-run 统计与 backup 一致且不写任何文件", func(t *testing.T) {
		e := newEnv(t, allTypesTree())
		c := filter.Criteria{ExcludeNames: []string{`\.go$`}}
		dr, err := DryRun(t.Context(), BackupOptions{Source: e.src, Repo: e.repo, Criteria: c, Password: []byte("ignored")}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if listDir(t, e.repo) != nil {
			t.Fatal("dry-run 创建了仓库目录")
		}
		bres, _ := e.backup(t, BackupOptions{Criteria: c})
		if dr.Files != bres.Files || dr.Dirs != bres.Dirs || dr.Bytes != bres.Bytes || dr.Skipped != bres.Skipped || dr.Skipped == 0 {
			t.Errorf("dry-run %+v ≠ backup %+v", dr, bres)
		}
	})

	t.Run("TC-ENG-028 仓库目录不存在时自动创建并发 info", func(t *testing.T) {
		e := newEnv(t, testutil.Dir{})
		e.repo = filepath.Join(e.repo, "deep", "er")
		_, rec := e.backup(t, BackupOptions{})
		if len(rec.logs) == 0 || rec.logs[0].level != LevelInfo {
			t.Errorf("logs = %v", rec.logs)
		}
	})
}

func TestRestoreErrors(t *testing.T) {
	newBackup := func(t *testing.T, password []byte) (env, string) {
		e := newEnv(t, testutil.Dir{"f": testutil.File{Size: 100000}, "d": testutil.Dir{"g": testutil.File{Content: "g"}}})
		res, _ := e.backup(t, BackupOptions{Password: password, Compression: container.CompressionHuffman})
		return e, res.Path
	}

	t.Run("TC-ENG-030 目标非空或不是目录 → ErrTargetNotEmpty", func(t *testing.T) {
		e, bk := newBackup(t, nil)
		if err := os.MkdirAll(filepath.Join(e.dst, "x"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := Restore(t.Context(), RestoreOptions{Backup: bk, Target: e.dst}, nil); !errors.Is(err, ErrTargetNotEmpty) {
			t.Errorf("非空: err = %v", err)
		}
		file := filepath.Join(e.dst, "x", "file")
		if err := os.WriteFile(file, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Restore(t.Context(), RestoreOptions{Backup: bk, Target: file}, nil); !errors.Is(err, ErrTargetNotEmpty) {
			t.Errorf("是文件: err = %v", err)
		}
	})

	t.Run("TC-ENG-031 已存在的空目录可以作为目标", func(t *testing.T) {
		e, bk := newBackup(t, nil)
		if err := os.Mkdir(e.dst, 0o755); err != nil {
			t.Fatal(err)
		}
		e.restore(t, bk, nil)
		testutil.AssertTreeEqual(t, e.src, e.dst)
	})

	t.Run("TC-ENG-032 备份不存在 → ErrBackupNotFound", func(t *testing.T) {
		e, _ := newBackup(t, nil)
		_, err := Restore(t.Context(), RestoreOptions{Backup: filepath.Join(e.repo, "nope.dbk"), Target: e.dst}, nil)
		if !errors.Is(err, ErrBackupNotFound) {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("TC-ENG-033 错误口令 → ErrAuth；缺口令 → ErrPasswordRequired；均不创建目标", func(t *testing.T) {
		e, bk := newBackup(t, []byte("right"))
		if _, err := Restore(t.Context(), RestoreOptions{Backup: bk, Target: e.dst, Password: []byte("wrong")}, nil); !errors.Is(err, container.ErrAuth) {
			t.Errorf("错误口令: err = %v", err)
		}
		if _, err := Restore(t.Context(), RestoreOptions{Backup: bk, Target: e.dst}, nil); !errors.Is(err, container.ErrPasswordRequired) {
			t.Errorf("缺口令: err = %v", err)
		}
		if _, err := os.Lstat(e.dst); !os.IsNotExist(err) {
			t.Error("认证失败时不应创建还原目录")
		}
	})

	t.Run("TC-ENG-034 篡改任一字节被检出，校验前不写任何文件", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			password []byte
			want     error
		}{
			{"加密", []byte("pw"), container.ErrAuth},
			{"未加密", nil, container.ErrCorrupt},
		} {
			t.Run(tc.name, func(t *testing.T) {
				e, bk := newBackup(t, tc.password)
				data, err := os.ReadFile(bk)
				if err != nil {
					t.Fatal(err)
				}
				data[len(data)/2] ^= 0x01
				if err := os.WriteFile(bk, data, 0o600); err != nil {
					t.Fatal(err)
				}
				_, err = Restore(t.Context(), RestoreOptions{Backup: bk, Target: e.dst, Password: tc.password}, nil)
				if !errors.Is(err, tc.want) {
					t.Errorf("err = %v, want %v", err, tc.want)
				}
				if _, err := os.Lstat(e.dst); !os.IsNotExist(err) {
					t.Error("校验失败时不应创建还原目录")
				}
			})
		}
	})

	t.Run("TC-ENG-035 未加密备份提供口令时发 warn 并忽略", func(t *testing.T) {
		e, bk := newBackup(t, nil)
		res, rec := e.restore(t, bk, []byte("unneeded"))
		if !rec.hasWarn("未加密") || res.Warnings != 1 {
			t.Errorf("warns = %v, Warnings = %d", rec.warns(), res.Warnings)
		}
	})

	t.Run("TC-ENG-036 不是普通文件的备份路径 → ErrCorrupt", func(t *testing.T) {
		e, _ := newBackup(t, nil)
		_, err := Restore(t.Context(), RestoreOptions{Backup: e.repo, Target: e.dst}, nil)
		if !errors.Is(err, container.ErrCorrupt) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestCraftedArchives(t *testing.T) {
	reg := func(p, data string) entry {
		return entry{fi: fsmeta.FileInfo{Path: p, Type: fsmeta.TypeRegular}, data: data}
	}
	for _, tc := range []struct {
		id      string
		entries []entry
		want    error
	}{
		{"TC-ENG-040 路径重复 → ErrCorrupt", []entry{reg("a", "1"), reg("a", "2")}, pack.ErrCorrupt},
		{"TC-ENG-041 指向软链接的硬链接 → ErrUnsafePath", []entry{
			{fi: fsmeta.FileInfo{Path: "s", Type: fsmeta.TypeSymlink, LinkTarget: "/"}},
			{fi: fsmeta.FileInfo{Path: "h", Type: fsmeta.TypeRegular, HardlinkTo: "s"}},
			reg("h/tmp/escaped", "pwned"),
		}, pack.ErrUnsafePath},
		{"TC-ENG-042 硬链接目标不存在 → ErrCorrupt", []entry{
			{fi: fsmeta.FileInfo{Path: "h", Type: fsmeta.TypeRegular, HardlinkTo: "missing"}},
		}, pack.ErrCorrupt},
		{"TC-ENG-043 软链接与同名目录 → ErrCorrupt", []entry{
			{fi: fsmeta.FileInfo{Path: "d", Type: fsmeta.TypeSymlink, LinkTarget: "/tmp"}},
			{fi: fsmeta.FileInfo{Path: "d", Type: fsmeta.TypeDir, Mode: 0o755}},
		}, pack.ErrCorrupt},
	} {
		t.Run(tc.id, func(t *testing.T) {
			base := t.TempDir()
			bk, out := filepath.Join(base, "x.dbk"), filepath.Join(base, "out")
			craft(t, bk, tc.entries)
			rec := &recorder{}
			_, err := Restore(t.Context(), RestoreOptions{Backup: bk, Target: out}, rec)
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
			if !rec.hasWarn("目标目录内容不完整") {
				t.Errorf("缺少目标不完整的 warn: %v", rec.warns())
			}
			for _, name := range listDir(t, base) {
				if name != "x.dbk" && name != "out" {
					t.Errorf("在还原目录外写出了 %s", name)
				}
			}
		})
	}
}

func TestCancel(t *testing.T) {
	bigTree := func() testutil.Dir {
		return testutil.Dir{"a": testutil.File{Size: 4 * ioBufSize}, "b": testutil.File{Size: 4 * ioBufSize}}
	}

	t.Run("TC-ENG-050 pack 阶段取消：返回 Canceled，.partial 被删除、无残留", func(t *testing.T) {
		e := newEnv(t, bigTree())
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		sawPartial := false
		rec := &recorder{hook: func(p Progress) {
			if p.Phase == PhasePack && p.Bytes > 0 {
				_, err := os.Stat(filepath.Join(e.repo, "snap.dbk.partial"))
				sawPartial = err == nil
				cancel()
			}
		}}
		old := progressInterval
		progressInterval = 0
		defer func() { progressInterval = old }()
		_, err := Backup(ctx, BackupOptions{Source: e.src, Repo: e.repo, Name: "snap", Password: []byte("p")}, rec)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want Canceled", err)
		}
		if err.Error() != "操作已取消" {
			t.Errorf("message = %q", err.Error())
		}
		if !sawPartial {
			t.Error("pack 阶段应写入 .partial")
		}
		if got := listDir(t, e.repo); len(got) != 0 {
			t.Errorf("取消后仓库有残留: %v", got)
		}
	})

	t.Run("TC-ENG-051 scan 阶段取消", func(t *testing.T) {
		e := newEnv(t, bigTree())
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := Backup(ctx, BackupOptions{Source: e.src, Repo: e.repo, Name: "s"}, nil); !errors.Is(err, context.Canceled) {
			t.Errorf("Backup err = %v", err)
		}
		if _, err := DryRun(ctx, BackupOptions{Source: e.src}, nil); !errors.Is(err, context.Canceled) {
			t.Errorf("DryRun err = %v", err)
		}
		if got := listDir(t, e.repo); len(got) != 0 {
			t.Errorf("取消后仓库有残留: %v", got)
		}
	})

	t.Run("TC-ENG-052 还原中取消：返回 Canceled 并发目标不完整 warn", func(t *testing.T) {
		e := newEnv(t, bigTree())
		bres, _ := e.backup(t, BackupOptions{})
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		rec := &recorder{hook: func(p Progress) {
			if p.Bytes > 0 {
				cancel()
			}
		}}
		old := progressInterval
		progressInterval = 0
		defer func() { progressInterval = old }()
		_, err := Restore(ctx, RestoreOptions{Backup: bres.Path, Target: e.dst}, rec)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
		if !rec.hasWarn("还原已取消，目标目录内容不完整") {
			t.Errorf("warns = %v", rec.warns())
		}
	})

	t.Run("TC-ENG-053 校验阶段取消不创建目标", func(t *testing.T) {
		e := newEnv(t, bigTree())
		bres, _ := e.backup(t, BackupOptions{})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := Restore(ctx, RestoreOptions{Backup: bres.Path, Target: e.dst}, nil); !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v", err)
		}
		if _, err := os.Lstat(e.dst); !os.IsNotExist(err) {
			t.Error("不应创建还原目录")
		}
	})
}
