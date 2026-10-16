//go:build linux

package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/container"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/filter"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/pack"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/repo"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/testutil"
)

var (
	t1 = time.Date(2019, 3, 4, 5, 6, 7, 123456789, time.UTC)
	t2 = time.Date(2024, 11, 12, 13, 14, 15, 987654321, time.UTC)
)

// basicTree 只含普通文件与目录（M2-5），覆盖空文件、跨缓冲区大文件、特殊权限位、
// 只读目录（其子项必须在目录权限设置前写入）与纳秒时间。
func basicTree() testutil.Dir {
	return testutil.Dir{
		"empty":    testutil.File{},
		"hello.go": testutil.File{Content: "package main\n", Mode: 0o640, Mtime: t1, Atime: t2},
		"big.bin":  testutil.File{Size: 3*ioBufSize + 17},
		"suid":     testutil.File{Content: "#!/bin/sh\n", Mode: 0o4755},
		"docs": testutil.MetaDir{Mode: 0o750, Mtime: t2, Entries: testutil.Dir{
			"readme.md": testutil.File{Content: "# 说明\n"},
			"ro": testutil.MetaDir{Mode: 0o555, Mtime: t1, Entries: testutil.Dir{
				"locked.txt": testutil.File{Content: "x", Mode: 0o400},
			}},
		}},
		"emptydir":   testutil.Dir{},
		"中文 文件名.txt": testutil.File{Content: "unicode"},
		"sticky":     testutil.MetaDir{Mode: 0o1777, Entries: testutil.Dir{}},
	}
}

// allTypesTree 含 M3-1 的全部非设备类型（设备另测，需要 root）。
func allTypesTree() testutil.Dir {
	tree := basicTree()
	tree["links"] = testutil.Dir{
		"rel":      testutil.Symlink{Target: "../hello.go", Mtime: t1},
		"abs":      testutil.Symlink{Target: "/etc/hostname"},
		"dangling": testutil.Symlink{Target: "no/such/file", Mtime: t2},
		"dirlink":  testutil.Symlink{Target: "../docs"},
	}
	tree["hard1"] = testutil.Hardlink{Target: "hello.go"}
	tree["nested"] = testutil.Dir{"hard2": testutil.Hardlink{Target: "big.bin"}}
	tree["pipe"] = testutil.FIFO{Mode: 0o640, Mtime: t1}
	tree["sock"] = testutil.Socket{Mode: 0o755}
	return tree
}

func TestRoundTrip(t *testing.T) {
	t.Run("TC-ENG-001 普通文件与目录往返五要素一致（FR-01、FR-02）", func(t *testing.T) {
		e := newEnv(t, basicTree())
		bres, rres := e.roundTrip(t, BackupOptions{})
		if bres.Files != 7 || bres.Dirs != 4 || rres.Files != 7 || rres.Dirs != 4 {
			t.Errorf("files/dirs: backup %d/%d restore %d/%d, want 7/4", bres.Files, bres.Dirs, rres.Files, rres.Dirs)
		}
		wantBytes := int64(len("package main\n") + 3*ioBufSize + 17 + len("#!/bin/sh\n") + len("# 说明\n") + 1 + len("unicode"))
		if bres.Bytes != wantBytes || rres.Bytes != wantBytes {
			t.Errorf("bytes: backup %d restore %d, want %d", bres.Bytes, rres.Bytes, wantBytes)
		}
		if bres.Compressed || bres.Encrypted || bres.Skipped != 0 || bres.Warnings != 0 {
			t.Errorf("result flags = %+v", bres)
		}
		if st, err := os.Stat(bres.Path); err != nil || st.Size() != bres.OutputBytes {
			t.Errorf("OutputBytes %d 与文件大小不符: %v", bres.OutputBytes, err)
		}
		if rres.Target != e.dst {
			t.Errorf("Target = %q, want %q", rres.Target, e.dst)
		}
	})

	t.Run("TC-ENG-002 atime 也被恢复", func(t *testing.T) {
		e := newEnv(t, testutil.Dir{"f": testutil.File{Content: "x", Mtime: t1, Atime: t2}})
		bres, _ := e.backup(t, BackupOptions{})
		e.restore(t, bres.Path, nil)
		if got := lstat(t, filepath.Join(e.dst, "f")); !got.Atime.Equal(t2) || !got.Mtime.Equal(t1) {
			t.Errorf("atime/mtime = %v/%v, want %v/%v", got.Atime, got.Mtime, t2, t1)
		}
	})

	t.Run("TC-ENG-003 软/硬链接、FIFO、套接字往返一致（M3-1）", func(t *testing.T) {
		e := newEnv(t, allTypesTree())
		bres, _ := e.roundTrip(t, BackupOptions{})
		// 非目录：7 个基础文件 + 4 软链接 + 2 硬链接 + FIFO + 套接字；目录：4 + links + nested。
		if bres.Files != 15 || bres.Dirs != 6 {
			t.Errorf("Files/Dirs = %d/%d, want 15/6", bres.Files, bres.Dirs)
		}
	})

	t.Run("TC-ENG-004 字符/块设备往返（root）", func(t *testing.T) {
		e := newEnv(t, testutil.Dir{
			"null": testutil.Device{Char: true, Major: 1, Minor: 3, Mode: 0o666},
			"blk":  testutil.Device{Major: 259, Minor: 300, Mode: 0o660, Mtime: t1},
			"dev":  testutil.Dir{"tty": testutil.Device{Char: true, Major: 4, Minor: 1}},
		})
		e.roundTrip(t, BackupOptions{})
	})

	t.Run("TC-ENG-005 属主属组往返（root，M3-2）", func(t *testing.T) {
		o := &testutil.Owner{UID: 1234, GID: 5678}
		e := newEnv(t, testutil.Dir{
			"f":  testutil.File{Content: "x", Owner: o, Mode: 0o2755},
			"ln": testutil.Symlink{Target: "f", Owner: &testutil.Owner{UID: 4321, GID: 8765}},
			"d":  testutil.MetaDir{Owner: o, Mode: 0o700, Entries: testutil.Dir{"g": testutil.File{Owner: o}}},
			"p":  testutil.FIFO{Owner: o},
		})
		e.roundTrip(t, BackupOptions{})
	})

	t.Run("TC-ENG-006 Huffman 压缩往返全类型并记录压缩率（M4-2）", func(t *testing.T) {
		e := newEnv(t, allTypesTree())
		if bres, _ := e.roundTrip(t, BackupOptions{Compression: container.CompressionHuffman}); !bres.Compressed {
			t.Error("Compressed = false")
		}

		// 压缩率：源代码风格的文本（伪随机数据不可压缩，不参与统计）。
		var src strings.Builder
		for i := range 2000 {
			fmt.Fprintf(&src, "func handler%d(w io.Writer, r *Request) error { // 处理第 %d 个请求\n\treturn nil\n}\n", i, i)
		}
		text := testutil.Dir{"main.go": testutil.File{Content: src.String()}}
		comp, plain := newEnv(t, text), newEnv(t, text)
		cres, _ := comp.backup(t, BackupOptions{Compression: container.CompressionHuffman})
		pres, _ := plain.backup(t, BackupOptions{})
		ratio := float64(cres.OutputBytes) / float64(pres.OutputBytes)
		t.Logf("压缩率（文本 %d 字节）: %d / %d = %.3f", cres.Bytes, cres.OutputBytes, pres.OutputBytes, ratio)
		if ratio >= 0.8 {
			t.Errorf("文本压缩率 %.3f 不低于 0.8", ratio)
		}
	})

	t.Run("TC-ENG-007 AES-256 加密往返（M5-5）", func(t *testing.T) {
		e := newEnv(t, allTypesTree())
		bres, _ := e.roundTrip(t, BackupOptions{Password: []byte("correct horse")})
		if !bres.Encrypted || bres.Compressed {
			t.Errorf("Encrypted/Compressed = %v/%v", bres.Encrypted, bres.Compressed)
		}
	})

	t.Run("TC-ENG-008 压缩 + 加密组合往返（M5-5）", func(t *testing.T) {
		tree := allTypesTree()
		tree["huge"] = testutil.File{Size: 2*(1<<20) + 12345} // 跨 Huffman 块
		e := newEnv(t, tree)
		bres, _ := e.roundTrip(t, BackupOptions{
			Compression: container.CompressionHuffman, Password: []byte("pw"),
		})
		if !bres.Encrypted || !bres.Compressed {
			t.Errorf("Encrypted/Compressed = %v/%v", bres.Encrypted, bres.Compressed)
		}
		info := readInfo(t, bres.Path)
		if info.Header.Encryption != container.EncryptionAES256CBC || info.Header.Compression != container.CompressionHuffman {
			t.Errorf("容器头 = %+v", info.Header)
		}
		if info.Summary.Files != bres.Files || info.Summary.Dirs != bres.Dirs || info.Summary.Bytes != bres.Bytes {
			t.Errorf("容器尾 %+v 与结果 %+v 不一致", info.Summary, bres)
		}
		if info.Header.Source != e.src {
			t.Errorf("容器头 Source = %q, want %q", info.Header.Source, e.src)
		}
	})

	t.Run("TC-ENG-009 空源目录", func(t *testing.T) {
		e := newEnv(t, testutil.Dir{})
		bres, rres := e.roundTrip(t, BackupOptions{Compression: container.CompressionHuffman, Password: []byte("p")})
		if bres.Files != 0 || bres.Dirs != 0 || rres.Files != 0 {
			t.Errorf("results = %+v / %+v", bres, rres)
		}
	})
}

// criteriaTypes 返回只保留所列类型字母的过滤条件。
func criteriaTypes(letters ...string) filter.Criteria {
	var c filter.Criteria
	for _, l := range letters {
		typ, err := fsmeta.ParseFileType(l)
		if err != nil {
			panic(err)
		}
		c.Types = append(c.Types, typ)
	}
	return c
}

func readInfo(t *testing.T, path string) container.Info {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, err := container.ReadInfo(f)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func TestMetadata(t *testing.T) {
	t.Run("TC-ENG-010 非 root 还原跳过 chown 并只发一次 warn（M3-2）", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("需要非 root")
		}
		base := t.TempDir()
		bk := filepath.Join(base, "x.dbk")
		craftOwned(t, bk, 0, 0) // 属主 root，当前用户无法 chown
		rec := &recorder{}
		res, err := Restore(t.Context(), RestoreOptions{Backup: bk, Target: filepath.Join(base, "out")}, rec)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, w := range rec.warns() {
			if w == "非 root 用户，已跳过属主恢复（chown）" {
				n++
			}
		}
		if n != 1 || res.Warnings != 1 {
			t.Errorf("chown warn 次数 = %d, Warnings = %d, want 1/1; logs=%v", n, res.Warnings, rec.logs)
		}
		if got := lstat(t, filepath.Join(base, "out", "a")); int(got.UID) != os.Geteuid() || got.Mode != 0o640 {
			t.Errorf("a: uid %d mode %#o", got.UID, got.Mode)
		}
	})

	t.Run("TC-ENG-011 属主与当前用户相同时不发 chown warn", func(t *testing.T) {
		e := newEnv(t, basicTree())
		bres, _ := e.backup(t, BackupOptions{})
		_, rec := e.restore(t, bres.Path, nil)
		if len(rec.warns()) != 0 {
			t.Errorf("warns = %v", rec.warns())
		}
	})

	t.Run("TC-ENG-012 只读目录的子项还原后目录元数据才设置", func(t *testing.T) {
		e := newEnv(t, testutil.Dir{
			"ro": testutil.MetaDir{Mode: 0o500, Mtime: t1, Entries: testutil.Dir{
				"sub": testutil.MetaDir{Mode: 0o500, Mtime: t2, Entries: testutil.Dir{
					"f": testutil.File{Content: "deep"},
				}},
			}},
		})
		e.roundTrip(t, BackupOptions{})
		if got := lstat(t, filepath.Join(e.dst, "ro")); got.Mode != 0o500 || !got.Mtime.Equal(t1) {
			t.Errorf("ro: mode %#o mtime %v", got.Mode, got.Mtime)
		}
	})

	t.Run("TC-ENG-013 只备份普通文件时还原自动补建父目录（0755）", func(t *testing.T) {
		e := newEnv(t, testutil.Dir{"a": testutil.MetaDir{Mode: 0o700, Entries: testutil.Dir{
			"b": testutil.Dir{"f": testutil.File{Content: "x"}},
		}}})
		bres, _ := e.backup(t, BackupOptions{Criteria: criteriaTypes("f")})
		if bres.Dirs != 0 || bres.Files != 1 || bres.Skipped != 2 {
			t.Errorf("result = %+v", bres)
		}
		e.restore(t, bres.Path, nil)
		for _, d := range []string{"a", "a/b"} {
			if got := lstat(t, filepath.Join(e.dst, d)); got.Mode != 0o755 || int(got.UID) != os.Geteuid() {
				t.Errorf("%s: mode %#o uid %d", d, got.Mode, got.UID)
			}
		}
	})
}

// craftOwned 写一个含两个文件 a、b，属主为 uid:gid、权限 0640 的未加密备份。
func craftOwned(t *testing.T, path string, uid, gid uint32) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cw, err := container.NewWriter(f, container.Header{CreatedAt: time.Now(), Source: "/x"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	pw := pack.NewWriter(cw)
	for _, name := range []string{"a", "b"} {
		fi := fsmeta.FileInfo{
			Path: name, Type: fsmeta.TypeRegular, Mode: 0o640, UID: uid, GID: gid, Size: 1,
			Atime: testutil.DefaultTime, Mtime: testutil.DefaultTime,
		}
		if err := pw.WriteHeader(fi); err != nil {
			t.Fatal(err)
		}
		if _, err := pw.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := pw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := cw.Finish(container.Summary{Files: 2, Bytes: 2}); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultName(t *testing.T) {
	t.Run("TC-ENG-014 缺省备份名为 repo.DefaultName", func(t *testing.T) {
		e := newEnv(t, testutil.Dir{"f": testutil.File{}})
		before := time.Now()
		res, err := Backup(t.Context(), BackupOptions{Source: e.src, Repo: e.repo}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if res.Name != repo.DefaultName(e.src, before) && res.Name != repo.DefaultName(e.src, time.Now()) {
			t.Errorf("Name = %q", res.Name)
		}
		if !strings.HasPrefix(res.Name, "src-") || res.Path != filepath.Join(e.repo, res.Name+".dbk") {
			t.Errorf("Name/Path = %q / %q", res.Name, res.Path)
		}
	})
}
