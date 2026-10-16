package container

import (
	"bytes"
	stdaes "crypto/aes"
	stdcipher "crypto/cipher"
	stdhmac "crypto/hmac"
	stdpbkdf2 "crypto/pbkdf2"
	stdsha256 "crypto/sha256"
	"errors"
	"io"
	"math/rand/v2"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/testutil"
)

var (
	testTime     = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	testPassword = []byte("correct horse")
)

// testIterations 让加密用例的密钥派生足够快；默认值另有用例覆盖。
const testIterations = 1000

// fixedRand 把盐与 IV 换成可复现的字节序列 0x00, 0x01, 0x02…
func fixedRand(t *testing.T) {
	t.Helper()
	var next byte
	old := randRead
	randRead = func(b []byte) (int, error) {
		for i := range b {
			b[i] = next
			next++
		}
		return len(b), nil
	}
	t.Cleanup(func() { randRead = old })
}

func plainHeader() Header {
	return Header{CreatedAt: testTime, Source: "/src"}
}

func encHeader() Header {
	return Header{
		Compression: CompressionHuffman,
		Encryption:  EncryptionAES256CBC,
		CreatedAt:   testTime,
		Source:      "/src",
		Iterations:  testIterations,
	}
}

// build 写出一个完整容器，payload 按随机大小分块写入。
func build(t testing.TB, h Header, password, payload []byte, s Summary) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := NewWriter(&buf, h, password)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	rng := rand.New(rand.NewPCG(1, 2))
	for rest := payload; len(rest) > 0; {
		n := min(len(rest), rng.IntN(70000))
		if _, err := w.Write(rest[:n]); err != nil {
			t.Fatalf("Write: %v", err)
		}
		rest = rest[n:]
	}
	if err := w.Finish(s); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	return buf.Bytes()
}

func openAll(data, password []byte) ([]byte, error) {
	r, err := Open(bytes.NewReader(data), password)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}

func randomBytes(n int, seed uint64) []byte {
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(rng.Uint32())
	}
	return b
}

func TestGolden(t *testing.T) {
	t.Run("TC-CTR-001 未加密容器字节布局", func(t *testing.T) {
		got := build(t, plainHeader(), nil, []byte("hello"), Summary{Files: 1, Dirs: 1, Bytes: 5})
		testutil.Golden(t, "testdata/plain.golden", got)
		// 与 container-format.md §7 示例逐段对照。
		want := []byte("DBAK\x01\x00\x00\x00\xa5\x35\x57\x69\x00\x00\x00\x00" +
			"\x00\x00\x00\x00\x00\x00\x00\x00\x04\x00\x00\x00\x00\x00\x00\x00/srchello" +
			"DBKE\x00\x00\x00\x00\x05\x00\x00\x00\x00\x00\x00\x00")
		if !bytes.HasPrefix(got, want) {
			t.Fatalf("头部与文档示例不符:\n%x\n%x", got[:len(want)], want)
		}
	})
	t.Run("TC-CTR-002 加密容器字节布局（固定盐与 IV）", func(t *testing.T) {
		fixedRand(t)
		got := build(t, encHeader(), testPassword, []byte("hello"), Summary{Files: 1, Dirs: 1, Bytes: 5})
		testutil.Golden(t, "testdata/encrypted.golden", got)
	})
}

// TestStdlibCrossCheck 只用标准库、按格式文档独立解析容器，确认实现与文档一致。
func TestStdlibCrossCheck(t *testing.T) {
	payload := randomBytes(100_003, 7)
	summary := Summary{Files: 3, Dirs: 2, Bytes: 100_003}

	t.Run("TC-CTR-003 加密：PBKDF2 密钥划分、CBC 密文与 HMAC 范围", func(t *testing.T) {
		data := build(t, encHeader(), testPassword, payload, summary)
		salt, iv := data[fixedLen:fixedLen+SaltLen], data[fixedLen+SaltLen:fixedLen+SaltLen+IVLen]
		hlen := fixedLen + SaltLen + IVLen + len("/src")
		body, mac := data[:len(data)-sumLen], data[len(data)-sumLen:]

		k, err := stdpbkdf2.Key(stdsha256.New, string(testPassword), salt, testIterations, 64)
		if err != nil {
			t.Fatal(err)
		}
		m := stdhmac.New(stdsha256.New, k[32:])
		m.Write(body)
		if !stdhmac.Equal(m.Sum(nil), mac) {
			t.Fatal("HMAC 与标准库不符")
		}
		ct := data[hlen : len(data)-trailerLen-sumLen]
		block, err := stdaes.NewCipher(k[:32])
		if err != nil {
			t.Fatal(err)
		}
		pt := make([]byte, len(ct))
		stdcipher.NewCBCDecrypter(block, iv).CryptBlocks(pt, ct)
		pad := int(pt[len(pt)-1])
		if pad < 1 || pad > 16 || !bytes.Equal(pt[:len(pt)-pad], payload) {
			t.Fatal("标准库解密结果与负载不符")
		}
		tr := data[len(data)-trailerLen-sumLen : len(data)-sumLen]
		if le.Uint64(tr[offPayloadLen:]) != uint64(len(ct)) {
			t.Fatal("尾部 payloadLen 不符")
		}
	})
	t.Run("TC-CTR-004 未加密：负载原样存放，SHA-256 覆盖头部‖负载‖尾部", func(t *testing.T) {
		data := build(t, plainHeader(), nil, payload, summary)
		hlen := fixedLen + len("/src")
		if !bytes.Equal(data[hlen:hlen+len(payload)], payload) {
			t.Fatal("负载不是原样存放")
		}
		sum := stdsha256.Sum256(data[:len(data)-sumLen])
		if !bytes.Equal(sum[:], data[len(data)-sumLen:]) {
			t.Fatal("SHA-256 与标准库不符")
		}
	})
}

func TestRoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		h        Header
		password []byte
		payload  []byte
	}{
		{"TC-CTR-010 未加密空负载", plainHeader(), nil, nil},
		{"TC-CTR-011 未加密 3MiB 随机负载", plainHeader(), nil, randomBytes(3<<20, 1)},
		{"TC-CTR-012 加密空负载", encHeader(), testPassword, nil},
		{"TC-CTR-013 加密 3MiB 随机负载", encHeader(), testPassword, randomBytes(3<<20, 2)},
		{"TC-CTR-014 加密负载恰为分组整数倍", encHeader(), testPassword, randomBytes(4096, 3)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := Summary{Files: 10, Dirs: 2, Bytes: int64(len(c.payload))}
			data := build(t, c.h, c.password, c.payload, s)
			r, err := Open(bytes.NewReader(data), c.password)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			got, err := io.ReadAll(iotest.HalfReader(r))
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			if !bytes.Equal(got, c.payload) {
				t.Fatalf("负载不一致：%d 字节 vs %d 字节", len(got), len(c.payload))
			}
			if n, err := r.Read(make([]byte, 1)); n != 0 || err != io.EOF {
				t.Fatalf("EOF 之后 Read = %d, %v", n, err)
			}
			if r.Info().Summary != s || r.Info().Size != int64(len(data)) {
				t.Fatalf("Info = %+v", r.Info())
			}
		})
	}

	t.Run("TC-CTR-015 ReadInfo 读出全部头尾字段", func(t *testing.T) {
		h := encHeader()
		h.CreatedAt = time.Date(2026, 10, 15, 23, 55, 0, 987654321, time.FixedZone("CST", 8*3600))
		s := Summary{Files: 120, Dirs: 14, Bytes: 5242880}
		data := build(t, h, testPassword, []byte("x"), s)
		info, err := ReadInfo(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		got := info.Header
		if got.Version != FormatVersion || got.Compression != CompressionHuffman ||
			got.Encryption != EncryptionAES256CBC || got.Source != "/src" ||
			got.Iterations != testIterations || len(got.Salt) != SaltLen || len(got.IV) != IVLen {
			t.Fatalf("Header = %+v", got)
		}
		if !got.CreatedAt.Equal(h.CreatedAt.Truncate(time.Second)) {
			t.Fatalf("CreatedAt = %v，应按秒精度保存 %v", got.CreatedAt, h.CreatedAt)
		}
		if info.Summary != s || info.Size != int64(len(data)) {
			t.Fatalf("Info = %+v", info)
		}
		r, err := Open(bytes.NewReader(data), testPassword)
		if err != nil {
			t.Fatal(err)
		}
		if ri := r.Info(); !bytes.Equal(ri.Header.Salt, got.Salt) || ri.Summary != s {
			t.Fatalf("Reader.Info 与 ReadInfo 不一致")
		}
	})
	t.Run("TC-CTR-016 Iterations 为 0 时使用默认值", func(t *testing.T) {
		h := encHeader()
		h.Iterations = 0
		data := build(t, h, testPassword, []byte("abc"), Summary{})
		info, err := ReadInfo(bytes.NewReader(data))
		if err != nil || info.Header.Iterations != DefaultIterations {
			t.Fatalf("Iterations = %d, %v", info.Header.Iterations, err)
		}
		if got, err := openAll(data, testPassword); err != nil || string(got) != "abc" {
			t.Fatalf("openAll = %q, %v", got, err)
		}
	})
	t.Run("TC-CTR-017 每次写入的盐与 IV 都随机生成", func(t *testing.T) {
		h := encHeader()
		h.Salt, h.IV = make([]byte, SaltLen), make([]byte, IVLen) // 调用方传入的值被忽略
		a := build(t, h, testPassword, nil, Summary{})
		b := build(t, h, testPassword, nil, Summary{})
		ia, _ := ReadInfo(bytes.NewReader(a))
		ib, _ := ReadInfo(bytes.NewReader(b))
		if bytes.Equal(ia.Header.Salt, ib.Header.Salt) || bytes.Equal(ia.Header.IV, ib.Header.IV) ||
			bytes.Equal(ia.Header.Salt, make([]byte, SaltLen)) {
			t.Fatal("盐或 IV 未随机生成")
		}
	})
}

func TestPassword(t *testing.T) {
	enc := build(t, encHeader(), testPassword, []byte("secret data"), Summary{Files: 1})
	plain := build(t, plainHeader(), nil, []byte("public data"), Summary{Files: 1})

	t.Run("TC-CTR-020 错误口令返回 ErrAuth", func(t *testing.T) {
		if _, err := Open(bytes.NewReader(enc), []byte("wrong")); !errors.Is(err, ErrAuth) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("TC-CTR-021 加密备份未给口令返回 ErrPasswordRequired，ReadInfo 无需口令", func(t *testing.T) {
		for _, pw := range [][]byte{nil, {}} {
			if _, err := Open(bytes.NewReader(enc), pw); !errors.Is(err, ErrPasswordRequired) {
				t.Fatalf("err = %v", err)
			}
		}
		if info, err := ReadInfo(bytes.NewReader(enc)); err != nil || info.Summary.Files != 1 {
			t.Fatalf("ReadInfo = %+v, %v", info, err)
		}
	})
	t.Run("TC-CTR-022 未加密备份忽略口令", func(t *testing.T) {
		if got, err := openAll(plain, []byte("anything")); err != nil || string(got) != "public data" {
			t.Fatalf("openAll = %q, %v", got, err)
		}
	})
	t.Run("TC-CTR-023 负载中看不到明文", func(t *testing.T) {
		if bytes.Contains(enc, []byte("secret")) {
			t.Fatal("加密容器中出现了明文")
		}
	})
}

func isDetected(err error) bool {
	return errors.Is(err, ErrAuth) || errors.Is(err, ErrCorrupt) || errors.Is(err, ErrUnsupportedVersion)
}

func TestTamper(t *testing.T) {
	payload := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	files := map[string]struct {
		data     []byte
		password []byte
		mismatch error // 只改负载或尾部统计时应返回的错误
	}{
		"加密":  {build(t, encHeader(), testPassword, payload, Summary{Files: 1, Dirs: 1, Bytes: 36}), testPassword, ErrAuth},
		"未加密": {build(t, plainHeader(), nil, payload, Summary{Files: 1, Dirs: 1, Bytes: 36}), nil, ErrCorrupt},
	}
	for name, f := range files {
		t.Run("TC-CTR-030 篡改任一字节都被检出（"+name+"）", func(t *testing.T) {
			for i := range f.data {
				for _, mask := range []byte{0x01, 0x80} {
					d := bytes.Clone(f.data)
					d[i] ^= mask
					if _, err := openAll(d, f.password); !isDetected(err) {
						t.Fatalf("偏移 %d 异或 %#x 未被检出：%v", i, mask, err)
					}
				}
			}
		})
		t.Run("TC-CTR-031 篡改负载与尾部统计（"+name+"）", func(t *testing.T) {
			info, _ := ReadInfo(bytes.NewReader(f.data))
			hlen := int(fixedLen + int64(len(info.Header.Salt)+len(info.Header.IV)+len(info.Header.Source)))
			trailer := len(f.data) - trailerLen - sumLen
			for _, off := range []int{hlen, len(f.data) - trailerLen - sumLen - 1, trailer + offFiles, trailer + offBytes + 7} {
				d := bytes.Clone(f.data)
				d[off] ^= 0x01
				if _, err := openAll(d, f.password); !errors.Is(err, f.mismatch) {
					t.Fatalf("偏移 %d：err = %v，期望 %v", off, err, f.mismatch)
				}
			}
		})
		t.Run("TC-CTR-032 任意长度截断都被检出（"+name+"）", func(t *testing.T) {
			for n := range len(f.data) {
				if _, err := openAll(f.data[:n], f.password); !errors.Is(err, ErrCorrupt) {
					t.Fatalf("截断到 %d 字节：err = %v", n, err)
				}
				if _, err := ReadInfo(bytes.NewReader(f.data[:n])); !errors.Is(err, ErrCorrupt) {
					t.Fatalf("ReadInfo 截断到 %d 字节：err = %v", n, err)
				}
			}
		})
		t.Run("TC-CTR-033 末尾追加字节被检出（"+name+"）", func(t *testing.T) {
			if _, err := openAll(append(bytes.Clone(f.data), 0), f.password); !errors.Is(err, ErrCorrupt) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestReadInfoErrors(t *testing.T) {
	plain := build(t, plainHeader(), nil, []byte("p"), Summary{})
	enc := build(t, encHeader(), testPassword, []byte("p"), Summary{})
	patch := func(src []byte, off int, b ...byte) []byte {
		d := bytes.Clone(src)
		copy(d[off:], b)
		return d
	}
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"TC-CTR-040 魔数不符", patch(plain, 0, 'X'), ErrCorrupt},
		{"TC-CTR-041 版本为 0", patch(plain, offVersion, 0, 0), ErrCorrupt},
		{"TC-CTR-042 版本高于支持的版本", patch(plain, offVersion, 2, 0), ErrUnsupportedVersion},
		{"TC-CTR-043 未知压缩算法", patch(plain, offCompression, 2), ErrCorrupt},
		{"TC-CTR-044 未知加密算法", patch(plain, offEncryption, 2), ErrCorrupt},
		{"TC-CTR-045 未加密却有迭代次数", patch(plain, offIterations, 1), ErrCorrupt},
		{"TC-CTR-046 加密迭代次数为 0", patch(enc, offIterations, 0, 0, 0, 0), ErrCorrupt},
		{"TC-CTR-047 迭代次数超过上限", patch(enc, offIterations, 0x81, 0x96, 0x98, 0x00), ErrCorrupt},
		{"TC-CTR-048 盐长度不符", patch(enc, offSaltLen, 8), ErrCorrupt},
		{"TC-CTR-049 IV 长度不符", patch(enc, offIVLen, 0), ErrCorrupt},
		{"TC-CTR-050 头部保留字段非 0", patch(plain, offReserved2+3, 1), ErrCorrupt},
		{"TC-CTR-051 源目录超长", patch(plain, offSourceLen, 0x01, 0x00, 0x01, 0x00), ErrCorrupt},
		{"TC-CTR-052 头部长度超出文件", patch(plain, offSourceLen, 0xff, 0x00), ErrCorrupt},
		{"TC-CTR-053 尾部魔数不符", patch(plain, len(plain)-trailerLen-sumLen, 'X'), ErrCorrupt},
		{"TC-CTR-054 尾部保留字段非 0", patch(plain, len(plain)-trailerLen-sumLen+offTrailerReserved, 1), ErrCorrupt},
		{"TC-CTR-055 统计为负", patch(plain, len(plain)-sumLen-1, 0x80), ErrCorrupt},
		{"TC-CTR-056 负载长度与文件长度不符", patch(plain, len(plain)-trailerLen-sumLen+offPayloadLen, 9), ErrCorrupt},
		{"TC-CTR-057 全零文件", make([]byte, 200), ErrCorrupt},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := ReadInfo(bytes.NewReader(c.data)); !errors.Is(err, c.want) {
				t.Fatalf("ReadInfo err = %v，期望 %v", err, c.want)
			}
			if _, err := Open(bytes.NewReader(c.data), testPassword); !errors.Is(err, c.want) {
				t.Fatalf("Open err = %v，期望 %v", err, c.want)
			}
		})
	}

	t.Run("TC-CTR-058 密文长度不是分组整数倍", func(t *testing.T) {
		// 把 1 字节源目录"挪"给负载：头部变短 1 字节、payloadLen 加 1，长度关系仍自洽。
		d := bytes.Clone(enc)
		le.PutUint32(d[offSourceLen:], uint32(len("/src")-1))
		tr := len(d) - trailerLen - sumLen
		le.PutUint64(d[tr+offPayloadLen:], le.Uint64(d[tr+offPayloadLen:])+1)
		if _, err := ReadInfo(bytes.NewReader(d)); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("TC-CTR-059 Seek 失败原样返回，不归为损坏", func(t *testing.T) {
		boom := errors.New("boom")
		_, err := ReadInfo(failingSeeker{err: boom})
		if !errors.Is(err, boom) || errors.Is(err, ErrCorrupt) {
			t.Fatalf("err = %v", err)
		}
	})
}

type failingSeeker struct{ err error }

func (f failingSeeker) Read([]byte) (int, error)       { return 0, f.err }
func (f failingSeeker) Seek(int64, int) (int64, error) { return 0, f.err }

// limitWriter 写满 n 字节后返回 err（err 为 nil 时表现为短写）。
type limitWriter struct {
	n   int
	err error
	buf bytes.Buffer
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if len(p) <= l.n {
		l.n -= len(p)
		return l.buf.Write(p)
	}
	k := l.n
	l.n = 0
	l.buf.Write(p[:k])
	return k, l.err
}

func TestWriterErrors(t *testing.T) {
	t.Run("TC-CTR-060 未加密却提供口令", func(t *testing.T) {
		if _, err := NewWriter(io.Discard, plainHeader(), []byte("x")); err == nil {
			t.Fatal("期望错误")
		}
	})
	t.Run("TC-CTR-061 加密却未提供口令", func(t *testing.T) {
		if _, err := NewWriter(io.Discard, encHeader(), nil); !errors.Is(err, ErrPasswordRequired) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("TC-CTR-062 非法头部参数", func(t *testing.T) {
		bad := []Header{
			{Compression: 2},
			{Encryption: 2},
			{Source: strings.Repeat("a", MaxSourceLen+1)},
		}
		for _, h := range bad {
			if _, err := NewWriter(io.Discard, h, nil); err == nil {
				t.Fatalf("%+v：期望错误", h)
			}
		}
		h := encHeader()
		h.Iterations = MaxIterations + 1
		if _, err := NewWriter(io.Discard, h, testPassword); err == nil {
			t.Fatal("迭代次数超限：期望错误")
		}
	})
	t.Run("TC-CTR-063 源目录恰为上限可以写入并读回", func(t *testing.T) {
		h := plainHeader()
		h.Source = strings.Repeat("a", MaxSourceLen)
		info, err := ReadInfo(bytes.NewReader(build(t, h, nil, nil, Summary{})))
		if err != nil || info.Header.Source != h.Source {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("TC-CTR-064 写头部失败", func(t *testing.T) {
		boom := errors.New("boom")
		if _, err := NewWriter(&limitWriter{n: 10, err: boom}, plainHeader(), nil); !errors.Is(err, boom) {
			t.Fatalf("err = %v", err)
		}
	})
	for _, enc := range []bool{false, true} {
		h, pw, kind := plainHeader(), []byte(nil), "未加密"
		if enc {
			h, pw, kind = encHeader(), testPassword, "加密"
		}
		t.Run("TC-CTR-065 写负载失败后错误粘滞（"+kind+"）", func(t *testing.T) {
			boom := errors.New("boom")
			w, err := NewWriter(&limitWriter{n: 100, err: boom}, h, pw)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write(make([]byte, 10000)); !errors.Is(err, boom) {
				t.Fatalf("Write err = %v", err)
			}
			if _, err := w.Write([]byte("x")); !errors.Is(err, boom) {
				t.Fatalf("再次 Write err = %v", err)
			}
			if err := w.Finish(Summary{}); !errors.Is(err, boom) {
				t.Fatalf("Finish err = %v", err)
			}
		})
		t.Run("TC-CTR-066 写尾部或校验值时短写（"+kind+"）", func(t *testing.T) {
			full := build(t, h, pw, []byte("abc"), Summary{})
			for _, cut := range []int{len(full) - sumLen - 1, len(full) - 1} {
				lw := &limitWriter{n: cut}
				w, err := NewWriter(lw, h, pw)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := w.Write([]byte("abc")); err != nil {
					t.Fatal(err)
				}
				if err := w.Finish(Summary{}); !errors.Is(err, io.ErrShortWrite) {
					t.Fatalf("cut=%d Finish err = %v", cut, err)
				}
			}
		})
	}
	t.Run("TC-CTR-067 Finish 之后不能再写或再 Finish", func(t *testing.T) {
		w, err := NewWriter(io.Discard, plainHeader(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := w.Finish(Summary{}); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("x")); !errors.Is(err, errFinished) {
			t.Fatalf("Write err = %v", err)
		}
		if err := w.Finish(Summary{}); !errors.Is(err, errFinished) {
			t.Fatalf("Finish err = %v", err)
		}
	})
	t.Run("TC-CTR-068 统计为负", func(t *testing.T) {
		w, err := NewWriter(io.Discard, plainHeader(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := w.Finish(Summary{Bytes: -1}); err == nil {
			t.Fatal("期望错误")
		}
	})
	t.Run("TC-CTR-069 随机数生成失败", func(t *testing.T) {
		boom := errors.New("no entropy")
		old := randRead
		randRead = func([]byte) (int, error) { return 0, boom }
		t.Cleanup(func() { randRead = old })
		if _, err := NewWriter(io.Discard, encHeader(), testPassword); !errors.Is(err, boom) {
			t.Fatalf("err = %v", err)
		}
	})
}

// mutableFile 允许在 Open 校验之后修改或截短内容，模拟校验与读取之间文件被改动。
type mutableFile struct {
	data []byte
	off  int64
}

func (m *mutableFile) Read(p []byte) (int, error) {
	if m.off >= int64(len(m.data)) {
		return 0, io.EOF
	}
	n := copy(p, m.data[m.off:])
	m.off += int64(n)
	return n, nil
}

func (m *mutableFile) Seek(off int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		m.off = off
	case io.SeekEnd:
		m.off = int64(len(m.data)) + off
	default:
		return 0, errors.New("unsupported whence")
	}
	return m.off, nil
}

func TestModifiedAfterVerify(t *testing.T) {
	payload := randomBytes(100_000, 9)
	t.Run("TC-CTR-070 校验后被截短返回 ErrCorrupt", func(t *testing.T) {
		f := &mutableFile{data: build(t, plainHeader(), nil, payload, Summary{})}
		r, err := Open(f, nil)
		if err != nil {
			t.Fatal(err)
		}
		f.data = f.data[:1000]
		if _, err := io.ReadAll(r); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("err = %v", err)
		}
		if _, err := r.Read(make([]byte, 1)); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("错误应粘滞：%v", err)
		}
	})
	t.Run("TC-CTR-071 校验后密文末组被改返回 ErrCorrupt", func(t *testing.T) {
		data := build(t, encHeader(), testPassword, payload, Summary{})
		f := &mutableFile{data: data}
		r, err := Open(f, testPassword)
		if err != nil {
			t.Fatal(err)
		}
		last := len(data) - trailerLen - sumLen - 1
		data[last-IVLen] ^= 0xff // 倒数第二组密文决定末组明文的填充字节
		if _, err := io.ReadAll(r); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("err = %v", err)
		}
	})
}

func FuzzOpen(f *testing.F) {
	f.Add(build(f, plainHeader(), nil, []byte("hello"), Summary{Files: 1}))
	f.Add(build(f, encHeader(), testPassword, []byte("hello"), Summary{Files: 1}))
	f.Add([]byte("DBAK"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if _, err := ReadInfo(bytes.NewReader(data)); err != nil && !isDetected(err) {
			t.Fatalf("ReadInfo 返回了非哨兵错误：%v", err)
		}
		// 不给口令：加密样本在密钥派生前就返回，未加密样本走完整校验与读取。
		r, err := Open(bytes.NewReader(data), nil)
		if err != nil {
			if !isDetected(err) && !errors.Is(err, ErrPasswordRequired) {
				t.Fatalf("Open 返回了非哨兵错误：%v", err)
			}
			return
		}
		if _, err := io.Copy(io.Discard, r); err != nil {
			t.Fatalf("校验通过后读取失败：%v", err)
		}
	})
}

func benchPayload() []byte { return randomBytes(1<<20, 42) }

func BenchmarkWrite(b *testing.B) {
	for _, enc := range []bool{false, true} {
		h, pw, name := plainHeader(), []byte(nil), "plain"
		if enc {
			h, pw, name = encHeader(), testPassword, "aes256"
			h.Iterations = 1 // 只测流式吞吐，不测密钥派生
		}
		b.Run(name, func(b *testing.B) {
			p := benchPayload()
			b.SetBytes(int64(len(p)))
			for b.Loop() {
				w, _ := NewWriter(io.Discard, h, pw)
				w.Write(p)
				w.Finish(Summary{})
			}
		})
	}
}

func BenchmarkOpenRead(b *testing.B) {
	for _, enc := range []bool{false, true} {
		h, pw, name := plainHeader(), []byte(nil), "plain"
		if enc {
			h, pw, name = encHeader(), testPassword, "aes256"
			h.Iterations = 1
		}
		b.Run(name, func(b *testing.B) {
			p := benchPayload()
			data := build(b, h, pw, p, Summary{})
			b.SetBytes(int64(len(p)))
			for b.Loop() {
				r, err := Open(bytes.NewReader(data), pw)
				if err != nil {
					b.Fatal(err)
				}
				io.Copy(io.Discard, r)
			}
		})
	}
}
