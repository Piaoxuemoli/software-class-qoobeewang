package chacha20

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"math/rand"
	"strings"
	"testing"
)

// mustHex 把十六进制字符串（允许空白）解码为字节，测试向量专用。
func mustHex(t testing.TB, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.Join(strings.Fields(s), ""))
	if err != nil {
		t.Fatalf("解析十六进制 %q: %v", s, err)
	}
	return b
}

// rfc8439Key 是 RFC 8439 §2.3.2 / §2.4.2 共用的 256 位测试密钥 00:01:…:1f。
var rfc8439Key = "000102030405060708090a0b0c0d0e0f" +
	"101112131415161718191a1b1c1d1e1f"

func TestBlockVector(t *testing.T) {
	// RFC 8439 §2.3.2：块函数测试向量（key=00:01:…:1f，nonce=000000090000004a00000000，
	// counter=1）的 64 字节密钥流输出。
	key := mustHex(t, rfc8439Key)
	nonce := mustHex(t, "000000090000004a00000000")
	want := mustHex(t, `
		10f1e7e4d13b5915500fdd1fa32071c4
		c7d1f4c733c068030422aa9ac3d46c4e
		d2826446079faa0914c2d705d98b02a2
		b5129cd1de164eb9cbd083e8a2503c4e`)

	t.Run("TC-CHACHA20-001 RFC 8439 §2.3.2 块函数", func(t *testing.T) {
		c, err := NewCipher(key, nonce)
		if err != nil {
			t.Fatalf("NewCipher: %v", err)
		}
		// 加密从 1 开始：异或 64 个零字节得到的"密文"就是第 1 块密钥流本身。
		got := make([]byte, 64)
		c.XORKeyStream(got, make([]byte, 64))
		if !bytes.Equal(got, want) {
			t.Fatalf("keystream = %x\nwant         %x", got, want)
		}
	})

	t.Run("TC-CHACHA20-002 分段异或与一次异或结果一致", func(t *testing.T) {
		c1, _ := NewCipher(key, nonce)
		c2, _ := NewCipher(key, nonce)
		src := make([]byte, 200)
		for i := range src {
			src[i] = byte(i * 7)
		}
		whole := make([]byte, len(src))
		c1.XORKeyStream(whole, src)
		parts := append([]byte(nil), src...)
		for off := 0; off < len(parts); {
			n := min(65, len(parts)-off) // 覆盖 64 字节块边界
			c2.XORKeyStream(parts[off:off+n], parts[off:off+n])
			off += n
		}
		if !bytes.Equal(whole, parts) {
			t.Fatalf("分段异或结果与一次异或不一致")
		}
	})
}

func TestRFC8439EncryptVector(t *testing.T) {
	// RFC 8439 §2.4.2：加密测试向量。明文 114 字节，counter 从 1 开始。
	key := mustHex(t, rfc8439Key)
	nonce := mustHex(t, "000000000000004a00000000")
	plain := []byte("Ladies and Gentlemen of the class of '99: If I could offer you only one tip for the future, sunscreen would be it.")
	want := mustHex(t, `
		6e2e359a2568f98041ba0728dd0d6981
		e97e7aec1d4360c20a27afccfd9fae0b
		f91b65c5524733ab8f593dabcd62b357
		1639d624e65152ab8f530c359f0861d8
		07ca0dbf500d6a6156a38e088a22b65e
		52bc514d16ccf806818ce91ab7793736
		5af90bbf74a35be6b40b8eedf2785e42
		874d`)

	t.Run("TC-CHACHA20-003 RFC 8439 §2.4.2 加密", func(t *testing.T) {
		var buf bytes.Buffer
		w, err := NewWriter(&buf, key, nonce)
		if err != nil {
			t.Fatalf("NewWriter: %v", err)
		}
		if _, err := w.Write(plain); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		if !bytes.Equal(buf.Bytes(), want) {
			t.Fatalf("ciphertext = %x\nwant           %x", buf.Bytes(), want)
		}
	})

	t.Run("TC-CHACHA20-004 RFC 8439 §2.4.2 解密", func(t *testing.T) {
		r, err := NewReader(bytes.NewReader(want), key, nonce)
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}
		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if !bytes.Equal(got, plain) {
			t.Fatalf("plaintext = %q, want %q", got, plain)
		}
	})
}

func TestCipherErrors(t *testing.T) {
	t.Run("TC-CHACHA20-005 非法密钥与 nonce 长度", func(t *testing.T) {
		if _, err := NewCipher(make([]byte, 16), make([]byte, NonceSize)); !errors.Is(err, ErrKeySize) {
			t.Fatalf("NewCipher(短密钥) err = %v, want ErrKeySize", err)
		}
		if _, err := NewCipher(make([]byte, KeySize), make([]byte, 8)); !errors.Is(err, ErrNonceSize) {
			t.Fatalf("NewCipher(短 nonce) err = %v, want ErrNonceSize", err)
		}
		if _, err := NewWriter(io.Discard, make([]byte, 8), make([]byte, NonceSize)); !errors.Is(err, ErrKeySize) {
			t.Fatalf("NewWriter(短密钥) err = %v, want ErrKeySize", err)
		}
		if _, err := NewReader(bytes.NewReader(nil), make([]byte, KeySize), make([]byte, 16)); !errors.Is(err, ErrNonceSize) {
			t.Fatalf("NewReader(长 nonce) err = %v, want ErrNonceSize", err)
		}
	})
}

func TestStreamRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	key, nonce := make([]byte, KeySize), make([]byte, NonceSize)
	rng.Read(key)
	rng.Read(nonce)

	for _, size := range []int{0, 1, 63, 64, 65, 4095, 4096, 4097, 1 << 20} {
		data := make([]byte, size)
		rng.Read(data)

		var enc bytes.Buffer
		w, _ := NewWriter(&enc, key, nonce)
		// 分段写入，覆盖 streamBufSize（4096）边界。
		for off := 0; off < len(data); {
			n := min(1500, len(data)-off)
			if _, err := w.Write(data[off : off+n]); err != nil {
				t.Fatalf("size=%d Write(%d): %v", size, n, err)
			}
			off += n
		}
		if err := w.Close(); err != nil {
			t.Fatalf("size=%d Close: %v", size, err)
		}
		if enc.Len() != size {
			t.Fatalf("size=%d 密文长度 %d != 明文长度", size, enc.Len())
		}

		r, _ := NewReader(bytes.NewReader(enc.Bytes()), key, nonce)
		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("size=%d ReadAll: %v", size, err)
		}
		if !bytes.Equal(got, data) {
			t.Fatalf("size=%d round-trip 不一致", size)
		}
	}
}

func TestStreamLifecycle(t *testing.T) {
	key, nonce := make([]byte, KeySize), make([]byte, NonceSize)
	for i := range key {
		key[i] = byte(i)
	}
	for i := range nonce {
		nonce[i] = byte(i)
	}

	t.Run("TC-CHACHA20-006 Close 后 Write 报错", func(t *testing.T) {
		w, _ := NewWriter(io.Discard, key, nonce)
		if err := w.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		if err := w.Close(); err != nil { // 重复 Close 幂等
			t.Fatalf("重复 Close: %v", err)
		}
		if _, err := w.Write([]byte("x")); err == nil {
			t.Fatal("Close 后 Write 应报错")
		}
	})

	t.Run("TC-CHACHA20-007 底层写错误粘滞", func(t *testing.T) {
		myErr := errors.New("boom")
		w, _ := NewWriter(&failWriter{err: myErr}, key, nonce)
		if _, err := w.Write([]byte("hello")); !errors.Is(err, myErr) {
			t.Fatalf("Write err = %v, want %v", err, myErr)
		}
		if _, err := w.Write([]byte("again")); !errors.Is(err, myErr) {
			t.Fatalf("第二次 Write err = %v, want %v", err, myErr)
		}
		if err := w.Close(); !errors.Is(err, myErr) {
			t.Fatalf("Close err = %v, want %v", err, myErr)
		}
	})

	t.Run("TC-CHACHA20-008 底层读错误粘滞", func(t *testing.T) {
		myErr := errors.New("boom")
		r, _ := NewReader(&failReader{err: myErr}, key, nonce)
		if _, err := r.Read(make([]byte, 10)); !errors.Is(err, myErr) {
			t.Fatalf("Read err = %v, want %v", err, myErr)
		}
		if _, err := r.Read(make([]byte, 10)); !errors.Is(err, myErr) {
			t.Fatalf("第二次 Read err = %v, want %v", err, myErr)
		}
	})
}

// failWriter 总是返回 err。
type failWriter struct{ err error }

func (w *failWriter) Write([]byte) (int, error) { return 0, w.err }

// failReader 总是返回 err。
type failReader struct{ err error }

func (r *failReader) Read([]byte) (int, error) { return 0, r.err }
