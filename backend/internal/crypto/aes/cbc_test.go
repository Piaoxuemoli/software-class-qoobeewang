package aes

import (
	"bytes"
	stdaes "crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"testing"
	"testing/iotest"
)

// NIST SP 800-38A 附录 F.2.5 / F.2.6 CBC-AES256 向量。
const (
	sp80038aKey = "603deb1015ca71be2b73aef0857d77811f352c073b6108d72d9810a30914dff4"
	sp80038aIV  = "000102030405060708090a0b0c0d0e0f"
	sp80038aPT  = "6bc1bee22e409f96e93d7e117393172a ae2d8a571e03ac9c9eb76fac45af8e51" +
		" 30c81c46a35ce411e5fbc1191a0a52ef f69f2445df4f9b17ad2b417be66c3710"
	sp80038aCT = "f58c4c04d6e5f1ba779eabfb5f7bfbd6 9cfc4e967edb808d679f777bc6702c7d" +
		" 39f23369a9d9bacfa530e26304231461 b2eb05e2c39be9fcda6c19078c6a9d1b"
)

// stdPad 按 PKCS#7 填充（对照组用，与被测实现无关）。
func stdPad(pt []byte) []byte {
	n := BlockSize - len(pt)%BlockSize
	return append(append([]byte(nil), pt...), bytes.Repeat([]byte{byte(n)}, n)...)
}

// stdCBCEncrypt 用标准库 CBC 加密原始分组（不做填充）。
func stdCBCEncrypt(t testing.TB, key, iv, blocks []byte) []byte {
	t.Helper()
	b, err := stdaes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, len(blocks))
	cipher.NewCBCEncrypter(b, iv).CryptBlocks(out, blocks)
	return out
}

// encryptChunks 用 CBCWriter 加密 pt，每次 Write 的长度依次取自 sizes（循环使用）。
func encryptChunks(t testing.TB, key, iv, pt []byte, sizes []int) []byte {
	t.Helper()
	var out bytes.Buffer
	w, err := NewCBCWriter(&out, key, iv)
	if err != nil {
		t.Fatalf("NewCBCWriter: %v", err)
	}
	for i := 0; len(pt) > 0; i++ {
		n := min(sizes[i%len(sizes)], len(pt))
		m, err := w.Write(pt[:n])
		if err != nil || m != n {
			t.Fatalf("Write(%d) = %d, %v", n, m, err)
		}
		pt = pt[n:]
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return out.Bytes()
}

// decryptWithBuf 用 CBCReader 解密 ct，每次 Read 传入 bufSize 字节的缓冲区。
func decryptWithBuf(key, iv, ct []byte, bufSize int) ([]byte, error) {
	r, err := NewCBCReader(bytes.NewReader(ct), key, iv)
	if err != nil {
		return nil, err
	}
	var out []byte
	buf := make([]byte, bufSize)
	for {
		n, err := r.Read(buf)
		out = append(out, buf[:n]...)
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
	}
}

func TestCBCVectors(t *testing.T) {
	key, iv := mustHex(t, sp80038aKey), mustHex(t, sp80038aIV)
	pt, ct := mustHex(t, sp80038aPT), mustHex(t, sp80038aCT)

	t.Run("TC-AES-009 SP 800-38A F.2.5 CBC-AES256 加密", func(t *testing.T) {
		c := mustCipher(t, key)
		prev := [BlockSize]byte(iv)
		got := make([]byte, len(pt))
		cbcEncryptBlocks(c, &prev, got, pt)
		if !bytes.Equal(got, ct) {
			t.Fatalf("cbcEncryptBlocks = %x, want %x", got, ct)
		}

		out := encryptChunks(t, key, iv, pt, []int{len(pt)})
		if len(out) != len(pt)+BlockSize {
			t.Fatalf("块对齐输入的密文长度 = %d, want %d", len(out), len(pt)+BlockSize)
		}
		if !bytes.Equal(out[:len(ct)], ct) {
			t.Fatalf("CBCWriter 前 64 字节 = %x, want %x", out[:len(ct)], ct)
		}
	})

	t.Run("TC-AES-010 SP 800-38A F.2.6 CBC-AES256 解密", func(t *testing.T) {
		c := mustCipher(t, key)
		prev := [BlockSize]byte(iv)
		got := make([]byte, len(ct))
		cbcDecryptBlocks(c, &prev, got, ct)
		if !bytes.Equal(got, pt) {
			t.Fatalf("cbcDecryptBlocks = %x, want %x", got, pt)
		}

		// 在向量末尾追加一个正确加密的整块填充（16×0x10），交给 CBCReader 解密。
		full := stdCBCEncrypt(t, key, iv, stdPad(pt))
		if !bytes.Equal(full[:len(ct)], ct) {
			t.Fatalf("对照组前 64 字节与向量不符")
		}
		plain, err := decryptWithBuf(key, iv, full, 4096)
		if err != nil {
			t.Fatalf("CBCReader: %v", err)
		}
		if !bytes.Equal(plain, pt) {
			t.Fatalf("CBCReader = %x, want %x", plain, pt)
		}
	})
}

func TestCBCDifferential(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	newKeyIV := func() ([]byte, []byte) {
		key, iv := make([]byte, KeySize), make([]byte, BlockSize)
		rng.Read(key)
		rng.Read(iv)
		return key, iv
	}

	t.Run("TC-AES-011 边界长度与标准库 CBC+PKCS#7 对照", func(t *testing.T) {
		for _, n := range []int{0, 1, 15, 16, 17, 31, 32, 33, 1000} {
			key, iv := newKeyIV()
			pt := make([]byte, n)
			rng.Read(pt)
			want := stdCBCEncrypt(t, key, iv, stdPad(pt))
			got := encryptChunks(t, key, iv, pt, []int{n + 1})
			if !bytes.Equal(got, want) {
				t.Fatalf("len=%d: 密文与标准库不一致", n)
			}
			for _, bs := range []int{1, 7, 16, 4096} {
				plain, err := decryptWithBuf(key, iv, want, bs)
				if err != nil {
					t.Fatalf("len=%d buf=%d: %v", n, bs, err)
				}
				if !bytes.Equal(plain, pt) {
					t.Fatalf("len=%d buf=%d: 解密结果不一致", n, bs)
				}
			}
		}
	})

	t.Run("TC-AES-012 1MiB 随机数据随机分块写与小缓冲读", func(t *testing.T) {
		key, iv := newKeyIV()
		pt := make([]byte, 1<<20)
		rng.Read(pt)
		sizes := make([]int, 64)
		for i := range sizes {
			sizes[i] = rng.Intn(5000) // 含 0 长度写
		}
		sizes[0] = 0
		want := stdCBCEncrypt(t, key, iv, stdPad(pt))
		got := encryptChunks(t, key, iv, pt, sizes)
		if !bytes.Equal(got, want) {
			t.Fatal("1MiB 密文与标准库不一致")
		}
		for _, bs := range []int{1, 7, 16, 4096} {
			plain, err := decryptWithBuf(key, iv, want, bs)
			if err != nil {
				t.Fatalf("buf=%d: %v", bs, err)
			}
			if !bytes.Equal(plain, pt) {
				t.Fatalf("buf=%d: 解密结果不一致", bs)
			}
		}
	})

	t.Run("TC-AES-013 块对齐输入追加整块 0x10 填充", func(t *testing.T) {
		key, iv := newKeyIV()
		c := mustCipher(t, key)
		for _, n := range []int{0, 16, 48} {
			pt := make([]byte, n)
			ct := encryptChunks(t, key, iv, pt, []int{7})
			if len(ct) != n+BlockSize {
				t.Fatalf("len=%d: 密文长度 = %d, want %d", n, len(ct), n+BlockSize)
			}
			last := make([]byte, BlockSize)
			c.Decrypt(last, ct[len(ct)-BlockSize:])
			prev := iv
			if n > 0 {
				prev = ct[len(ct)-2*BlockSize : len(ct)-BlockSize]
			}
			for i := range last {
				last[i] ^= prev[i]
			}
			if !bytes.Equal(last, bytes.Repeat([]byte{0x10}, BlockSize)) {
				t.Fatalf("len=%d: 最后一组明文 = %x，应为 16 个 0x10", n, last)
			}
		}
	})
}

func TestCBCErrors(t *testing.T) {
	key := make([]byte, KeySize)
	iv := make([]byte, BlockSize)

	t.Run("TC-AES-014 密钥或 IV 长度非法", func(t *testing.T) {
		for _, n := range []int{16, 24, 31, 33} {
			if _, err := NewCBCWriter(io.Discard, make([]byte, n), iv); !errors.Is(err, ErrKeySize) {
				t.Errorf("NewCBCWriter key=%d err = %v, want ErrKeySize", n, err)
			}
			if _, err := NewCBCReader(bytes.NewReader(nil), make([]byte, n), iv); !errors.Is(err, ErrKeySize) {
				t.Errorf("NewCBCReader key=%d err = %v, want ErrKeySize", n, err)
			}
		}
		for _, n := range []int{0, 15, 17} {
			if _, err := NewCBCWriter(io.Discard, key, make([]byte, n)); !errors.Is(err, ErrIVSize) {
				t.Errorf("NewCBCWriter iv=%d err = %v, want ErrIVSize", n, err)
			}
			if _, err := NewCBCReader(bytes.NewReader(nil), key, make([]byte, n)); !errors.Is(err, ErrIVSize) {
				t.Errorf("NewCBCReader iv=%d err = %v, want ErrIVSize", n, err)
			}
		}
	})

	t.Run("TC-AES-015 密文长度不是正的分组整数倍", func(t *testing.T) {
		for _, n := range []int{0, 1, 15, 17, 31, 33} {
			for _, bs := range []int{1, 16, 4096} {
				if _, err := decryptWithBuf(key, iv, make([]byte, n), bs); !errors.Is(err, ErrPadding) {
					t.Errorf("密文 %d 字节 buf=%d: err = %v, want ErrPadding", n, bs, err)
				}
			}
		}
	})

	t.Run("TC-AES-016 篡改末组填充返回 ErrPadding", func(t *testing.T) {
		good := bytes.Repeat([]byte{0xAA}, BlockSize)
		cases := map[string][]byte{
			"填充字节为 0":    append(bytes.Repeat([]byte{0xAA}, 15), 0x00),
			"填充字节为 17":   append(bytes.Repeat([]byte{0xAA}, 15), 0x11),
			"填充字节为 0xff": append(bytes.Repeat([]byte{0xAA}, 15), 0xff),
			"填充字节不一致":    append(bytes.Repeat([]byte{0xAA}, 12), 0x04, 0x03, 0x04, 0x04),
			"整块填充不一致":    append([]byte{0x0f}, bytes.Repeat([]byte{0x10}, 15)...),
		}
		for name, last := range cases {
			ct := stdCBCEncrypt(t, key, iv, append(append([]byte(nil), good...), last...))
			for _, bs := range []int{1, 16, 4096} {
				if _, err := decryptWithBuf(key, iv, ct, bs); !errors.Is(err, ErrPadding) {
					t.Errorf("%s buf=%d: err = %v, want ErrPadding", name, bs, err)
				}
			}
		}
	})

	t.Run("TC-AES-017 错误与 EOF 粘滞", func(t *testing.T) {
		r, err := NewCBCReader(bytes.NewReader(make([]byte, 17)), key, iv)
		if err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 64)
		var first error
		for first == nil {
			_, first = r.Read(buf)
		}
		if !errors.Is(first, ErrPadding) {
			t.Fatalf("err = %v, want ErrPadding", first)
		}
		for range 3 {
			if n, err := r.Read(buf); n != 0 || err != first {
				t.Fatalf("出错后 Read = %d, %v, want 0, %v", n, err, first)
			}
		}

		ct := stdCBCEncrypt(t, key, iv, stdPad([]byte("hello")))
		r, err = NewCBCReader(bytes.NewReader(ct), key, iv)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := io.ReadAll(r); err != nil || string(got) != "hello" {
			t.Fatalf("ReadAll = %q, %v", got, err)
		}
		for range 3 {
			if n, err := r.Read(buf); n != 0 || err != io.EOF {
				t.Fatalf("EOF 后 Read = %d, %v, want 0, EOF", n, err)
			}
		}
	})
}

// closeRecorder 记录 Close 是否被调用，并可让 Write 失败或短写。
type closeRecorder struct {
	bytes.Buffer
	closed   bool
	failErr  error
	shortBy  int
	writeCnt int
}

func (c *closeRecorder) Write(p []byte) (int, error) {
	c.writeCnt++
	if c.failErr != nil {
		return 0, c.failErr
	}
	if c.shortBy > 0 && len(p) > c.shortBy {
		n, _ := c.Buffer.Write(p[:len(p)-c.shortBy])
		return n, nil
	}
	return c.Buffer.Write(p)
}

func (c *closeRecorder) Close() error {
	c.closed = true
	return nil
}

func TestCBCWriterLifecycle(t *testing.T) {
	key := make([]byte, KeySize)
	iv := make([]byte, BlockSize)

	t.Run("TC-AES-018 Close 不关闭底层且之后 Write 报错", func(t *testing.T) {
		rec := &closeRecorder{}
		w, err := NewCBCWriter(rec, key, iv)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("abc")); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		if rec.closed {
			t.Fatal("Close 不应关闭底层 Writer")
		}
		if rec.Len() != BlockSize {
			t.Fatalf("密文长度 = %d, want %d", rec.Len(), BlockSize)
		}
		if n, err := w.Write([]byte("x")); err == nil || n != 0 {
			t.Fatalf("Close 后 Write = %d, %v, want 0 与错误", n, err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("重复 Close = %v, want nil", err)
		}
		if rec.Len() != BlockSize {
			t.Fatal("重复 Close 不应再写出数据")
		}
	})

	t.Run("TC-AES-019 底层写失败或短写时错误粘滞", func(t *testing.T) {
		boom := errors.New("磁盘已满")
		rec := &closeRecorder{failErr: boom}
		w, _ := NewCBCWriter(rec, key, iv)
		if _, err := w.Write(make([]byte, 100)); !errors.Is(err, boom) {
			t.Fatalf("Write err = %v, want %v", err, boom)
		}
		if _, err := w.Write(make([]byte, 100)); !errors.Is(err, boom) {
			t.Fatalf("再次 Write err = %v, want %v", err, boom)
		}
		if err := w.Close(); !errors.Is(err, boom) {
			t.Fatalf("Close err = %v, want %v", err, boom)
		}

		short := &closeRecorder{shortBy: 1}
		w, _ = NewCBCWriter(short, key, iv)
		if _, err := w.Write(make([]byte, 64)); !errors.Is(err, io.ErrShortWrite) {
			t.Fatalf("短写 err = %v, want io.ErrShortWrite", err)
		}
	})

	t.Run("TC-AES-021 不与调用者的 IV 切片共享内存", func(t *testing.T) {
		pt := []byte("0123456789abcdef0123")
		myIV := bytes.Repeat([]byte{7}, BlockSize)
		want := stdCBCEncrypt(t, key, myIV, stdPad(pt))

		var out bytes.Buffer
		w, _ := NewCBCWriter(&out, key, myIV)
		myIV[0] ^= 0xff
		if _, err := w.Write(pt); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out.Bytes(), want) {
			t.Fatal("修改调用者 IV 影响了 CBCWriter")
		}

		readIV := bytes.Repeat([]byte{7}, BlockSize)
		r, _ := NewCBCReader(bytes.NewReader(want), key, readIV)
		readIV[0] ^= 0xff
		if got, err := io.ReadAll(r); err != nil || !bytes.Equal(got, pt) {
			t.Fatalf("修改调用者 IV 影响了 CBCReader: %q, %v", got, err)
		}
	})
}

// errReader 先给出 data，随后返回 err。
type errReader struct {
	data []byte
	err  error
}

func (e *errReader) Read(p []byte) (int, error) {
	if len(e.data) == 0 {
		return 0, e.err
	}
	n := copy(p, e.data)
	e.data = e.data[n:]
	return n, nil
}

// zeroReader 永远返回 0, nil，用来检验不会死循环。
type zeroReader struct{}

func (zeroReader) Read([]byte) (int, error) { return 0, nil }

func TestCBCReaderSource(t *testing.T) {
	key := make([]byte, KeySize)
	iv := make([]byte, BlockSize)

	t.Run("TC-AES-020 底层读错误原样透传", func(t *testing.T) {
		boom := errors.New("读盘失败")
		r, _ := NewCBCReader(&errReader{data: make([]byte, 40), err: boom}, key, iv)
		if _, err := io.ReadAll(r); !errors.Is(err, boom) {
			t.Fatalf("err = %v, want %v", err, boom)
		}
	})

	t.Run("TC-AES-022 底层持续返回 0 字节时不死循环", func(t *testing.T) {
		r, _ := NewCBCReader(zeroReader{}, key, iv)
		if _, err := r.Read(make([]byte, 16)); !errors.Is(err, io.ErrNoProgress) {
			t.Fatalf("err = %v, want io.ErrNoProgress", err)
		}
	})

	t.Run("TC-AES-023 流式：未到 EOF 已能读出前面的明文", func(t *testing.T) {
		pt := bytes.Repeat([]byte("streaming-block!"), 4) // 4 组，加填充共 5 组
		ct := stdCBCEncrypt(t, key, iv, stdPad(pt))
		pr, pw := io.Pipe()
		go func() {
			_, _ = pw.Write(ct[:3*BlockSize])
		}()
		r, _ := NewCBCReader(pr, key, iv)
		got := make([]byte, 2*BlockSize)
		if _, err := io.ReadFull(r, got); err != nil {
			t.Fatalf("ReadFull: %v", err)
		}
		if !bytes.Equal(got, pt[:2*BlockSize]) {
			t.Fatal("前两组明文不一致")
		}
		go func() {
			_, _ = pw.Write(ct[3*BlockSize:])
			_ = pw.Close()
		}()
		rest, err := io.ReadAll(r)
		if err != nil || !bytes.Equal(rest, pt[2*BlockSize:]) {
			t.Fatalf("剩余明文 = %q, %v", rest, err)
		}
	})

	t.Run("TC-AES-024 io.Reader 约定：单字节源、数据与 EOF 同时返回", func(t *testing.T) {
		pt := make([]byte, 5000)
		rand.New(rand.NewSource(4)).Read(pt)
		ct := stdCBCEncrypt(t, key, iv, stdPad(pt))
		wrappers := map[string]func(io.Reader) io.Reader{
			"OneByteReader": iotest.OneByteReader,
			"DataErrReader": iotest.DataErrReader,
			"HalfReader":    iotest.HalfReader,
		}
		for name, wrap := range wrappers {
			r, _ := NewCBCReader(wrap(bytes.NewReader(ct)), key, iv)
			if err := iotest.TestReader(r, pt); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}
	})
}

// FuzzCBCReader：任意密文都不 panic、不死循环，且结果与标准库 CBC + PKCS#7 校验一致。
func FuzzCBCReader(f *testing.F) {
	key := bytes.Repeat([]byte{0x42}, KeySize)
	iv := bytes.Repeat([]byte{0x24}, BlockSize)
	f.Add([]byte{})
	f.Add(make([]byte, 15))
	f.Add(make([]byte, 32))
	f.Add(stdCBCEncrypt(f, key, iv, stdPad([]byte("fuzz seed"))))
	f.Add(stdCBCEncrypt(f, key, iv, stdPad(make([]byte, 16))))

	f.Fuzz(func(t *testing.T, ct []byte) {
		got, err := decryptWithBuf(key, iv, ct, 7)

		// 对照组：标准库解密 + 手工校验填充。
		var want []byte
		ok := len(ct) > 0 && len(ct)%BlockSize == 0
		if ok {
			plain := make([]byte, len(ct))
			b, _ := stdaes.NewCipher(key)
			cipher.NewCBCDecrypter(b, iv).CryptBlocks(plain, ct)
			pad := int(plain[len(plain)-1])
			ok = pad >= 1 && pad <= BlockSize
			for i := 0; ok && i < pad; i++ {
				ok = plain[len(plain)-1-i] == byte(pad)
			}
			if ok {
				want = plain[:len(plain)-pad]
			}
		}
		if !ok {
			if !errors.Is(err, ErrPadding) {
				t.Fatalf("非法密文 %x: err = %v, want ErrPadding", ct, err)
			}
			return
		}
		if err != nil {
			t.Fatalf("合法密文 %x: err = %v", ct, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("解密结果 %x, want %x", got, want)
		}
	})
}

// FuzzCBCRoundTrip：任意明文、密钥、分块方式下，加密结果与标准库一致且可解回原文。
func FuzzCBCRoundTrip(f *testing.F) {
	f.Add([]byte(""), []byte("k"), uint8(1), uint8(1))
	f.Add([]byte("hello, world"), []byte("another key"), uint8(5), uint8(7))
	f.Add(make([]byte, 64), make([]byte, 48), uint8(16), uint8(16))

	f.Fuzz(func(t *testing.T, pt, seed []byte, chunk, bufSize uint8) {
		keyIV := make([]byte, KeySize+BlockSize)
		copy(keyIV, seed)
		key, iv := keyIV[:KeySize], keyIV[KeySize:]
		ct := encryptChunks(t, key, iv, pt, []int{int(chunk)%33 + 1})
		if want := stdCBCEncrypt(t, key, iv, stdPad(pt)); !bytes.Equal(ct, want) {
			t.Fatalf("密文与标准库不一致")
		}
		got, err := decryptWithBuf(key, iv, ct, int(bufSize)%4097+1)
		if err != nil {
			t.Fatalf("解密: %v", err)
		}
		if !bytes.Equal(got, pt) {
			t.Fatalf("往返不一致")
		}
	})
}

func benchmarkPlaintext() []byte {
	pt := make([]byte, 1<<20)
	rand.New(rand.NewSource(3)).Read(pt)
	return pt
}

func BenchmarkCBCWriter(b *testing.B) {
	key, iv := make([]byte, KeySize), make([]byte, BlockSize)
	pt := benchmarkPlaintext()
	b.SetBytes(int64(len(pt)))
	for b.Loop() {
		w, _ := NewCBCWriter(io.Discard, key, iv)
		if _, err := w.Write(pt); err != nil {
			b.Fatal(err)
		}
		if err := w.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCBCReader(b *testing.B) {
	key, iv := make([]byte, KeySize), make([]byte, BlockSize)
	ct := stdCBCEncrypt(b, key, iv, stdPad(benchmarkPlaintext()))
	buf := make([]byte, 32*1024)
	b.SetBytes(int64(len(ct)))
	for b.Loop() {
		r, _ := NewCBCReader(bytes.NewReader(ct), key, iv)
		for {
			_, err := r.Read(buf)
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatal(fmt.Errorf("Read: %w", err))
			}
		}
	}
}
