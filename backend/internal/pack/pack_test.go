package pack

import (
	"bytes"
	"encoding/binary"
	"errors"
	stdcrc32 "hash/crc32"
	"io"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/testutil"
)

var (
	t0 = time.Date(2026, 5, 6, 7, 8, 9, 123456789, time.UTC)
	t1 = time.Date(1969, 12, 31, 23, 59, 58, 1, time.UTC) // 1970 年之前，秒为负
)

type entry struct {
	fi   fsmeta.FileInfo
	data []byte
}

func reg(p, content string) entry {
	return entry{fi: fsmeta.FileInfo{
		Path: p, Type: fsmeta.TypeRegular, Mode: 0o644, UID: 1000, GID: 1000,
		Size: int64(len(content)), Mtime: t0, Atime: t0, Ctime: t0,
	}, data: []byte(content)}
}

func node(p string, typ fsmeta.FileType) fsmeta.FileInfo {
	return fsmeta.FileInfo{Path: p, Type: typ, Mode: 0o755, Mtime: t0, Atime: t0, Ctime: t0}
}

// allTypes 覆盖全部类型、硬链接、btime、负时间、长路径（扩展块）。
func allTypes() []entry {
	dir := node("d", fsmeta.TypeDir)
	dir.Btime = t1
	link := node("ln", fsmeta.TypeSymlink)
	link.Mode, link.LinkTarget = 0o777, "../outside/target"
	hard := reg("d/hard", "")
	hard.fi.HardlinkTo = "d/a.txt"
	char := node("null", fsmeta.TypeCharDevice)
	char.Mode, char.DevMajor, char.DevMinor = 0o666, 1, 3
	blk := node("sda", fsmeta.TypeBlockDevice)
	blk.Mode, blk.DevMajor, blk.DevMinor = 0o660, 8, 0
	suid := reg("suid", "#!/bin/sh\n")
	suid.fi.Mode, suid.fi.Atime = 0o4755, t1
	long := reg(strings.Repeat("长目录名/", 40)+"file", strings.Repeat("x", 513))
	return []entry{
		{fi: dir},
		reg("d/a.txt", "hello"),
		hard,
		{fi: link},
		{fi: node("pipe", fsmeta.TypeFIFO)},
		{fi: node("sock", fsmeta.TypeSocket)},
		{fi: char},
		{fi: blk},
		reg("empty", ""),
		suid,
		long,
	}
}

func writeArchive(t testing.TB, entries []entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := NewWriter(&buf)
	for _, e := range entries {
		if err := w.WriteHeader(e.fi); err != nil {
			t.Fatalf("WriteHeader(%q): %v", e.fi.Path, err)
		}
		if _, err := w.Write(e.data); err != nil {
			t.Fatalf("Write(%q): %v", e.fi.Path, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return buf.Bytes()
}

func readArchive(b []byte) ([]entry, error) {
	r := NewReader(bytes.NewReader(b))
	var out []entry
	for {
		fi, err := r.Next()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		data, err := io.ReadAll(r)
		if err != nil {
			return out, err
		}
		out = append(out, entry{fi: fi, data: data})
	}
}

func sameInfo(a, b fsmeta.FileInfo) bool {
	return a.Path == b.Path && a.Type == b.Type && a.Mode == b.Mode && a.UID == b.UID && a.GID == b.GID &&
		a.Size == b.Size && a.Atime.Equal(b.Atime) && a.Mtime.Equal(b.Mtime) && a.Ctime.Equal(b.Ctime) &&
		a.Btime.Equal(b.Btime) && a.LinkTarget == b.LinkTarget && a.HardlinkTo == b.HardlinkTo &&
		a.DevMajor == b.DevMajor && a.DevMinor == b.DevMinor && b.Dev == 0 && b.Ino == 0 && b.Nlink == 0
}

func assertRoundTrip(t *testing.T, entries []entry) []byte {
	t.Helper()
	b := writeArchive(t, entries)
	if len(b)%BlockSize != 0 {
		t.Fatalf("归档长度 %d 不是 %d 的倍数", len(b), BlockSize)
	}
	got, err := readArchive(b)
	if err != nil {
		t.Fatalf("读取: %v", err)
	}
	if len(got) != len(entries) {
		t.Fatalf("读出 %d 个条目，期望 %d", len(got), len(entries))
	}
	for i := range entries {
		want := entries[i].fi
		if want.IsHardlink() {
			want.Size = 0
		}
		if !sameInfo(want, got[i].fi) {
			t.Errorf("条目 %d:\n  写入 %+v\n  读出 %+v", i, want, got[i].fi)
		}
		if !bytes.Equal(entries[i].data, got[i].data) {
			t.Errorf("条目 %q 数据不一致（%d vs %d 字节）", want.Path, len(entries[i].data), len(got[i].data))
		}
	}
	return b
}

func TestGolden(t *testing.T) {
	t.Run("TC-PACK-001 空归档是两个零块", func(t *testing.T) {
		b := writeArchive(t, nil)
		testutil.Golden(t, "testdata/empty.golden", b)
		if !bytes.Equal(b, make([]byte, 2*BlockSize)) {
			t.Error("空归档不是 1024 个 0")
		}
	})
	t.Run("TC-PACK-002 单个普通文件", func(t *testing.T) {
		b := writeArchive(t, []entry{reg("a.txt", "hello")})
		testutil.Golden(t, "testdata/single-file.golden", b)
		if len(b) != 4*BlockSize || string(b[:4]) != Magic || string(b[BlockSize:BlockSize+5]) != "hello" {
			t.Errorf("布局不符：长度 %d", len(b))
		}
	})
	t.Run("TC-PACK-003 全部类型与长路径", func(t *testing.T) {
		testutil.Golden(t, "testdata/all-types.golden", writeArchive(t, allTypes()))
	})
}

func TestRoundTrip(t *testing.T) {
	t.Run("TC-PACK-010 全部类型往返一致", func(t *testing.T) {
		assertRoundTrip(t, allTypes())
	})

	t.Run("TC-PACK-011 数据长度边界", func(t *testing.T) {
		var entries []entry
		for i, n := range []int{0, 1, 511, 512, 513, 1023, 1024, 1 << 20, 1<<20 + 7} {
			data := make([]byte, n)
			for j := range data {
				data[j] = byte(j*7 + i)
			}
			entries = append(entries, reg("f"+string(rune('a'+i)), string(data)))
		}
		b := assertRoundTrip(t, entries)
		var want int
		for _, e := range entries {
			want += BlockSize + (len(e.data)+BlockSize-1)/BlockSize*BlockSize
		}
		if want += 2 * BlockSize; len(b) != want {
			t.Errorf("归档长度 %d，期望 %d", len(b), want)
		}
	})

	t.Run("TC-PACK-012 名字区与扩展块边界", func(t *testing.T) {
		for _, n := range []int{1, inlineNameLen - 1, inlineNameLen, inlineNameLen + 1,
			inlineNameLen + BlockSize, inlineNameLen + BlockSize + 1, MaxNameLen} {
			p := strings.Repeat("p", n)
			assertRoundTrip(t, []entry{reg(p, "data")})

			// 路径 + 软链接目标跨越内联区与扩展块。
			sl := node(strings.Repeat("s", max(1, n/2)), fsmeta.TypeSymlink)
			sl.LinkTarget = strings.Repeat("t", n)
			b := assertRoundTrip(t, []entry{{fi: sl}})
			if ext := extBlocksFor(len(sl.Path) + len(sl.LinkTarget)); len(b) != (1+ext+2)*BlockSize {
				t.Errorf("名字长 %d：归档 %d 字节，期望 %d 个扩展块", n, len(b), ext)
			}
		}
	})

	t.Run("TC-PACK-013 Unicode 与特殊字符路径", func(t *testing.T) {
		assertRoundTrip(t, []entry{
			reg("中文 目录/文件 名.txt", "内容"),
			reg("emoji-😀/a\tb\nc", "x"),
			reg("-leading-dash", ""),
			reg(" space ", ""),
		})
	})

	t.Run("TC-PACK-014 时间与属主取值范围", func(t *testing.T) {
		e := reg("t", "")
		e.fi.UID, e.fi.GID = 1<<32-1, 0
		e.fi.Mtime = time.Unix(1<<40, 999999999)
		e.fi.Atime = time.Time{}
		e.fi.Ctime = t1
		e.fi.Btime = time.Unix(0, 0) // Unix 纪元本身也是有效 btime
		e.fi.Mode = 0o7777
		assertRoundTrip(t, []entry{e})
	})

	t.Run("TC-PACK-015 随机条目序列往返", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(1, 2))
		for round := range 20 {
			var entries []entry
			for i := range 30 {
				p := strings.Repeat("d/", rng.IntN(5)) + string(rune('a'+round)) + strings.Repeat("n", rng.IntN(600)) + string(rune('0'+i%10)) + string(rune('A'+i/10))
				switch rng.IntN(4) {
				case 0:
					entries = append(entries, entry{fi: node(p, fsmeta.TypeDir)})
				case 1:
					sl := node(p, fsmeta.TypeSymlink)
					sl.LinkTarget = strings.Repeat("../", rng.IntN(3)) + "x"
					entries = append(entries, entry{fi: sl})
				default:
					data := make([]byte, rng.IntN(3000))
					for j := range data {
						data[j] = byte(rng.Uint32())
					}
					e := reg(p, string(data))
					e.fi.Mtime = time.Unix(rng.Int64N(1<<34)-1<<33, rng.Int64N(1e9))
					entries = append(entries, e)
				}
			}
			assertRoundTrip(t, entries)
		}
	})

	t.Run("TC-PACK-016 Read 使用 1 字节缓冲", func(t *testing.T) {
		b := writeArchive(t, []entry{reg("a", "hello world"), reg("b", "x")})
		r := NewReader(bytes.NewReader(b))
		if _, err := r.Next(); err != nil {
			t.Fatal(err)
		}
		var got []byte
		one := make([]byte, 1)
		for {
			n, err := r.Read(one)
			got = append(got, one[:n]...)
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		if string(got) != "hello world" {
			t.Errorf("got %q", got)
		}
		if fi, err := r.Next(); err != nil || fi.Path != "b" {
			t.Errorf("Next = %v, %v", fi.Path, err)
		}
	})

	t.Run("TC-PACK-017 Next 跳过未读完的数据", func(t *testing.T) {
		b := writeArchive(t, []entry{reg("a", strings.Repeat("z", 5000)), reg("b", "x")})
		r := NewReader(bytes.NewReader(b))
		r.Next()
		io.CopyN(io.Discard, r, 100)
		if fi, err := r.Next(); err != nil || fi.Path != "b" {
			t.Fatalf("Next = %q, %v", fi.Path, err)
		}
		if _, err := r.Next(); err != io.EOF {
			t.Errorf("结尾 = %v，期望 io.EOF", err)
		}
		if _, err := r.Next(); err != io.EOF {
			t.Errorf("结尾之后 = %v，期望仍为 io.EOF", err)
		}
		if n, err := r.Read(make([]byte, 1)); n != 0 || err != io.EOF {
			t.Errorf("结尾之后 Read = %d, %v", n, err)
		}
	})
}

func TestWriterErrors(t *testing.T) {
	newW := func() (*Writer, *bytes.Buffer) {
		var buf bytes.Buffer
		return NewWriter(&buf), &buf
	}

	t.Run("TC-PACK-020 非法路径返回 ErrUnsafePath", func(t *testing.T) {
		for _, p := range []string{"", "/abs", "a/../b", "..", ".", "./a", "a/./b", "a//b", "a/", "a\x00b"} {
			w, buf := newW()
			if err := w.WriteHeader(reg(p, "").fi); !errors.Is(err, ErrUnsafePath) {
				t.Errorf("路径 %q: err = %v", p, err)
			}
			if buf.Len() != 0 {
				t.Errorf("路径 %q: 失败后仍写出了 %d 字节", p, buf.Len())
			}
		}
		hard := reg("h", "")
		hard.fi.HardlinkTo = "../etc/passwd"
		w, _ := newW()
		if err := w.WriteHeader(hard.fi); !errors.Is(err, ErrUnsafePath) {
			t.Errorf("硬链接目标: err = %v", err)
		}
	})

	t.Run("TC-PACK-021 数据超长返回 ErrWriteTooLong 并写到声明长度为止", func(t *testing.T) {
		w, _ := newW()
		w.WriteHeader(reg("a", "abc").fi)
		if n, err := w.Write([]byte("abcdef")); n != 3 || !errors.Is(err, ErrWriteTooLong) {
			t.Errorf("Write = %d, %v", n, err)
		}
		if err := w.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		w, _ = newW()
		w.WriteHeader(node("d", fsmeta.TypeDir))
		if _, err := w.Write([]byte("x")); !errors.Is(err, ErrWriteTooLong) {
			t.Errorf("目录写数据: %v", err)
		}
	})

	t.Run("TC-PACK-022 数据不足时 WriteHeader 与 Close 报错", func(t *testing.T) {
		w, _ := newW()
		w.WriteHeader(reg("a", "abc").fi)
		w.Write([]byte("ab"))
		if err := w.WriteHeader(reg("b", "").fi); err == nil {
			t.Error("WriteHeader 未报错")
		}
		if err := w.Close(); err == nil {
			t.Error("Close 未报错")
		}
	})

	t.Run("TC-PACK-023 非法字段", func(t *testing.T) {
		dirWithSize := node("d", fsmeta.TypeDir)
		dirWithSize.Size = 1
		hardWithSize := reg("h", "abc")
		hardWithSize.fi.HardlinkTo = "a"
		badMode := reg("m", "")
		badMode.fi.Mode = 0o10000
		neg := reg("n", "")
		neg.fi.Size = -1
		emptyLink := node("l", fsmeta.TypeSymlink)
		strayLink := node("f", fsmeta.TypeFIFO)
		strayLink.LinkTarget = "x"
		hardDir := node("hd", fsmeta.TypeDir)
		hardDir.HardlinkTo = "a"
		tooLong := reg(strings.Repeat("x", MaxNameLen+1), "")
		for name, fi := range map[string]fsmeta.FileInfo{
			"目录带长度": dirWithSize, "硬链接带长度": hardWithSize.fi, "权限超出 0o7777": badMode.fi,
			"负长度": neg.fi, "软链接目标为空": emptyLink, "FIFO 带链接目标": strayLink,
			"目录作为硬链接": hardDir, "路径超长": tooLong.fi, "类型为 0": {Path: "z"},
		} {
			w, _ := newW()
			if err := w.WriteHeader(fi); err == nil {
				t.Errorf("%s: 未报错", name)
			}
		}
	})

	t.Run("TC-PACK-024 Close 之后的操作", func(t *testing.T) {
		w, buf := newW()
		w.Close()
		if err := w.Close(); err != nil {
			t.Errorf("重复 Close: %v", err)
		}
		if err := w.WriteHeader(reg("a", "").fi); err == nil {
			t.Error("Close 后 WriteHeader 未报错")
		}
		if _, err := w.Write(nil); err == nil {
			t.Error("Close 后 Write 未报错")
		}
		if buf.Len() != 2*BlockSize {
			t.Errorf("输出 %d 字节", buf.Len())
		}
	})

	t.Run("TC-PACK-025 底层写错误保持", func(t *testing.T) {
		boom := errors.New("磁盘已满")
		w := NewWriter(failWriter{boom})
		if err := w.WriteHeader(reg("a", "x").fi); !errors.Is(err, boom) {
			t.Fatalf("err = %v", err)
		}
		if err := w.Close(); !errors.Is(err, boom) {
			t.Errorf("Close err = %v", err)
		}
	})
}

type failWriter struct{ err error }

func (f failWriter) Write([]byte) (int, error) { return 0, f.err }

// rechecksum 重新计算 b 开头条目（头块 + 扩展块）的校验和，用于构造"校验和正确但字段非法"的输入。
func rechecksum(b []byte) {
	ext := int(binary.LittleEndian.Uint32(b[offExtBlocks:]))
	hdr := b[:BlockSize*(1+ext)]
	binary.LittleEndian.PutUint32(hdr[offChecksum:], 0)
	binary.LittleEndian.PutUint32(hdr[offChecksum:], crc32(hdr))
}

func TestReaderCorrupt(t *testing.T) {
	valid := func() []byte { return writeArchive(t, []entry{reg("a.txt", "hello"), reg("b", "x")}) }
	le := binary.LittleEndian
	cases := []struct {
		name   string
		mutate func(b []byte) []byte
	}{
		{"TC-PACK-040 魔数错误", func(b []byte) []byte { b[0] = 'X'; rechecksum(b); return b }},
		{"TC-PACK-041 版本不支持", func(b []byte) []byte { b[offVersion] = 2; rechecksum(b); return b }},
		{"TC-PACK-042 校验和不符（名字被篡改）", func(b []byte) []byte { b[offName] = 'A'; return b }},
		{"TC-PACK-043 校验和不符（数据区之外的字段被篡改）", func(b []byte) []byte { b[offUID] ^= 1; return b }},
		{"TC-PACK-044 未知类型码", func(b []byte) []byte { b[offType] = 'Z'; rechecksum(b); return b }},
		{"TC-PACK-045 权限位超出", func(b []byte) []byte { le.PutUint32(b[offMode:], 0o170000); rechecksum(b); return b }},
		{"TC-PACK-046 纳秒越界", func(b []byte) []byte { le.PutUint32(b[offMtimeNsec:], 1e9); rechecksum(b); return b }},
		{"TC-PACK-047 负长度", func(b []byte) []byte { le.PutUint64(b[offSize:], 1<<63); rechecksum(b); return b }},
		{"TC-PACK-048 目录带长度", func(b []byte) []byte { b[offType] = typeDir; rechecksum(b); return b }},
		{"TC-PACK-049 名字长度超上限", func(b []byte) []byte { le.PutUint32(b[offPathLen:], MaxNameLen+1); rechecksum(b); return b }},
		{"TC-PACK-050 扩展块数不符", func(b []byte) []byte { le.PutUint32(b[offExtBlocks:], 1); return b }},
		{"TC-PACK-051 名字区填充非 0", func(b []byte) []byte { b[BlockSize-1] = 1; rechecksum(b); return b }},
		{"TC-PACK-052 保留字节非 0", func(b []byte) []byte { b[offReserved2] = 1; rechecksum(b); return b }},
		{"TC-PACK-053 未定义的 flags 位", func(b []byte) []byte { b[offFlags] = 0x80; rechecksum(b); return b }},
		{"TC-PACK-054 未标记却有 btime", func(b []byte) []byte { le.PutUint64(b[offBtimeSec:], 5); rechecksum(b); return b }},
		{"TC-PACK-055 非链接类型带链接目标", func(b []byte) []byte {
			le.PutUint32(b[offPathLen:], 4)
			le.PutUint32(b[offLinkLen:], 1)
			rechecksum(b)
			return b
		}},
		{"TC-PACK-056 软链接目标为空", func(b []byte) []byte {
			b[offType] = typeSymlink
			le.PutUint64(b[offSize:], 0)
			rechecksum(b)
			return b
		}},
		{"TC-PACK-057 头块截断", func(b []byte) []byte { return b[:100] }},
		{"TC-PACK-058 数据截断", func(b []byte) []byte { return b[:BlockSize+3] }},
		{"TC-PACK-059 缺少结尾零块", func(b []byte) []byte { return b[:len(b)-2*BlockSize] }},
		{"TC-PACK-060 只有一个结尾零块", func(b []byte) []byte { return b[:len(b)-BlockSize] }},
		{"TC-PACK-061 零块后跟非零块", func(b []byte) []byte {
			b[len(b)-1] = 1
			return b
		}},
		{"TC-PACK-062 空输入", func([]byte) []byte { return nil }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := readArchive(c.mutate(valid()))
			if !errors.Is(err, ErrCorrupt) {
				t.Errorf("err = %v，期望 ErrCorrupt", err)
			}
		})
	}

	t.Run("TC-PACK-063 出错后错误保持", func(t *testing.T) {
		b := valid()
		b[offName] = 'A'
		r := NewReader(bytes.NewReader(b))
		_, err1 := r.Next()
		_, err2 := r.Next()
		_, err3 := r.Read(make([]byte, 1))
		if !errors.Is(err1, ErrCorrupt) || err2 != err1 || err3 != err1 {
			t.Errorf("errs = %v / %v / %v", err1, err2, err3)
		}
	})

	t.Run("TC-PACK-064 扩展块截断", func(t *testing.T) {
		b := writeArchive(t, []entry{reg(strings.Repeat("p", 2000), "")})
		if _, err := readArchive(b[:BlockSize+10]); !errors.Is(err, ErrCorrupt) {
			t.Errorf("err = %v", err)
		}
	})
}

// rawEntry 绕过 Writer 的校验直接编码条目，用于构造恶意归档。
func rawEntry(fi fsmeta.FileInfo) []byte {
	return encodeHeader(fi)
}

func TestReaderUnsafePath(t *testing.T) {
	archive := func(fis ...fsmeta.FileInfo) []byte {
		var b []byte
		for _, fi := range fis {
			b = append(b, rawEntry(fi)...)
		}
		return append(b, make([]byte, 2*BlockSize)...)
	}
	symlink := func(p, target string) fsmeta.FileInfo {
		fi := node(p, fsmeta.TypeSymlink)
		fi.LinkTarget = target
		return fi
	}
	hardlink := func(p, target string) fsmeta.FileInfo {
		fi := node(p, fsmeta.TypeRegular)
		fi.HardlinkTo = target
		return fi
	}
	cases := []struct {
		name string
		fis  []fsmeta.FileInfo
	}{
		{"TC-PACK-070 绝对路径", []fsmeta.FileInfo{node("/etc/passwd", fsmeta.TypeRegular)}},
		{"TC-PACK-071 含 .. 段", []fsmeta.FileInfo{node("a/../../etc", fsmeta.TypeDir)}},
		{"TC-PACK-072 单独的 ..", []fsmeta.FileInfo{node("..", fsmeta.TypeDir)}},
		{"TC-PACK-073 空段", []fsmeta.FileInfo{node("a//b", fsmeta.TypeDir)}},
		{"TC-PACK-074 硬链接目标越界", []fsmeta.FileInfo{hardlink("h", "../../etc/shadow")}},
		{"TC-PACK-075 经软链接写出还原目录", []fsmeta.FileInfo{
			symlink("a", "/etc"), node("a/passwd", fsmeta.TypeRegular)}},
		{"TC-PACK-076 经多级目录下的软链接写出", []fsmeta.FileInfo{
			node("d", fsmeta.TypeDir), symlink("d/l", "/"), node("d/l/x/y", fsmeta.TypeDir)}},
		{"TC-PACK-077 硬链接目标经软链接", []fsmeta.FileInfo{
			symlink("a", "/etc"), hardlink("x", "a/passwd")}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := readArchive(archive(c.fis...)); !errors.Is(err, ErrUnsafePath) {
				t.Errorf("err = %v，期望 ErrUnsafePath", err)
			}
		})
	}

	t.Run("TC-PACK-078 同名前缀但非祖先的路径不受影响", func(t *testing.T) {
		got, err := readArchive(archive(symlink("a", "/etc"), node("ab/c", fsmeta.TypeRegular), symlink("x/a", "y")))
		if err != nil || len(got) != 3 {
			t.Errorf("got %d 条, err = %v", len(got), err)
		}
	})
}

func TestCRC32(t *testing.T) {
	if got := crc32([]byte("123456789")); got != 0xCBF43926 {
		t.Errorf("crc32 校验值 = %08x，期望 cbf43926", got)
	}
	rng := rand.New(rand.NewPCG(3, 4))
	for range 100 {
		b := make([]byte, rng.IntN(3000))
		for i := range b {
			b[i] = byte(rng.Uint32())
		}
		if crc32(b) != stdcrc32.ChecksumIEEE(b) {
			t.Fatalf("与标准库 hash/crc32 不一致（长度 %d）", len(b))
		}
	}
}

func FuzzReader(f *testing.F) {
	f.Add(writeArchive(f, nil))
	f.Add(writeArchive(f, []entry{reg("a.txt", "hello")}))
	f.Add(writeArchive(f, allTypes()))
	f.Add([]byte(Magic))
	f.Fuzz(func(t *testing.T, b []byte) {
		r := NewReader(bytes.NewReader(b))
		// 每个条目至少消耗一个块，条目数不会超过输入块数。
		for i := 0; i <= len(b)/BlockSize; i++ {
			fi, err := r.Next()
			if err != nil {
				if err != io.EOF && !errors.Is(err, ErrCorrupt) && !errors.Is(err, ErrUnsafePath) {
					t.Fatalf("意外错误类型: %v", err)
				}
				return
			}
			if checkPath(fi.Path) != nil {
				t.Fatalf("Reader 返回了不安全路径 %q", fi.Path)
			}
			if _, err := io.Copy(io.Discard, r); err != nil && !errors.Is(err, ErrCorrupt) {
				t.Fatalf("读取数据: %v", err)
			}
		}
		t.Fatal("条目数超过输入块数")
	})
}

func BenchmarkWriteRead(b *testing.B) {
	data := bytes.Repeat([]byte("0123456789abcdef"), 1<<16) // 1 MiB
	entries := make([]entry, 16)
	for i := range entries {
		entries[i] = reg(strings.Repeat("d/", i)+"f", string(data))
	}
	b.SetBytes(int64(len(data) * len(entries)))
	for b.Loop() {
		archive := writeArchive(b, entries)
		r := NewReader(bytes.NewReader(archive))
		for {
			if _, err := r.Next(); err == io.EOF {
				break
			} else if err != nil {
				b.Fatal(err)
			}
			io.Copy(io.Discard, r)
		}
	}
}
