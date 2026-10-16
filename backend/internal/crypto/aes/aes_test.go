package aes

import (
	"bytes"
	stdaes "crypto/aes"
	"encoding/hex"
	"errors"
	"math/rand"
	"sync"
	"testing"
)

// mustHex 把十六进制字符串（允许空格）解码为字节，测试向量专用。
func mustHex(t testing.TB, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(string(bytes.ReplaceAll([]byte(s), []byte(" "), nil)))
	if err != nil {
		t.Fatalf("解析十六进制 %q: %v", s, err)
	}
	return b
}

// mustCipher 构造 Cipher，失败即终止测试。
func mustCipher(t testing.TB, key []byte) *Cipher {
	t.Helper()
	c, err := NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	return c
}

func TestBlockVectors(t *testing.T) {
	// FIPS-197 附录 C.3 AES-256 (Nk=8, Nr=14) 示例向量。
	key := mustHex(t, "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	plain := mustHex(t, "00112233445566778899aabbccddeeff")
	want := mustHex(t, "8ea2b7ca516745bfeafc49904b496089")

	t.Run("TC-AES-001 FIPS-197 C.3 分组加密", func(t *testing.T) {
		c := mustCipher(t, key)
		got := make([]byte, BlockSize)
		c.Encrypt(got, plain)
		if !bytes.Equal(got, want) {
			t.Fatalf("Encrypt = %x, want %x", got, want)
		}
	})

	t.Run("TC-AES-002 FIPS-197 C.3 分组解密", func(t *testing.T) {
		c := mustCipher(t, key)
		got := make([]byte, BlockSize)
		c.Decrypt(got, want)
		if !bytes.Equal(got, plain) {
			t.Fatalf("Decrypt = %x, want %x", got, plain)
		}
	})

	t.Run("TC-AES-005 原地加解密", func(t *testing.T) {
		c := mustCipher(t, key)
		buf := append([]byte(nil), plain...)
		c.Encrypt(buf, buf)
		if !bytes.Equal(buf, want) {
			t.Fatalf("原地 Encrypt = %x, want %x", buf, want)
		}
		c.Decrypt(buf, buf)
		if !bytes.Equal(buf, plain) {
			t.Fatalf("原地 Decrypt = %x, want %x", buf, plain)
		}
	})
}

func TestKeyExpansion(t *testing.T) {
	t.Run("TC-AES-003 FIPS-197 A.3 密钥扩展抽查", func(t *testing.T) {
		key := mustHex(t, "603deb1015ca71be2b73aef0857d77811f352c073b6108d72d9810a30914dff4")
		c := mustCipher(t, key)
		// 附录 A.3 表中 w[i] 一列的若干行。
		want := map[int]uint32{
			0:  0x603deb10,
			7:  0x0914dff4,
			8:  0x9ba35411,
			9:  0x8e6925af,
			12: 0xa8b09c1a,
			15: 0xb75d5b9a,
			56: 0xfe4890d1,
			57: 0xe6188d0b,
			58: 0x046df344,
			59: 0x706c631e,
		}
		for i, w := range want {
			if c.w[i] != w {
				t.Errorf("w[%d] = %08x, want %08x", i, c.w[i], w)
			}
		}
	})
}

func TestSBox(t *testing.T) {
	t.Run("TC-AES-004 S 盒抽查与逆 S 盒互逆", func(t *testing.T) {
		// FIPS-197 §5.1.1 图 7：S(00)=63，S(53)=ed（正文示例），S(ff)=16。
		for in, out := range map[byte]byte{0x00: 0x63, 0x53: 0xed, 0x01: 0x7c, 0xff: 0x16} {
			if sbox[in] != out {
				t.Errorf("sbox[%02x] = %02x, want %02x", in, sbox[in], out)
			}
		}
		for i := range 256 {
			if invSbox[sbox[i]] != byte(i) {
				t.Fatalf("invSbox[sbox[%02x]] = %02x", i, invSbox[sbox[i]])
			}
		}
	})
}

func TestBlockDifferential(t *testing.T) {
	t.Run("TC-AES-006 随机分组与 crypto/aes 对照", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1))
		key := make([]byte, KeySize)
		src := make([]byte, BlockSize)
		got := make([]byte, BlockSize)
		want := make([]byte, BlockSize)
		for range 200 {
			rng.Read(key)
			c := mustCipher(t, key)
			ref, err := stdaes.NewCipher(key)
			if err != nil {
				t.Fatal(err)
			}
			for range 10 {
				rng.Read(src)
				c.Encrypt(got, src)
				ref.Encrypt(want, src)
				if !bytes.Equal(got, want) {
					t.Fatalf("key=%x src=%x: Encrypt = %x, want %x", key, src, got, want)
				}
				c.Decrypt(got, src)
				ref.Decrypt(want, src)
				if !bytes.Equal(got, want) {
					t.Fatalf("key=%x src=%x: Decrypt = %x, want %x", key, src, got, want)
				}
			}
		}
	})
}

func TestNewCipherKeySize(t *testing.T) {
	t.Run("TC-AES-007 密钥长度非 32 字节返回 ErrKeySize", func(t *testing.T) {
		for _, n := range []int{0, 16, 24, 31, 33} {
			if _, err := NewCipher(make([]byte, n)); !errors.Is(err, ErrKeySize) {
				t.Errorf("NewCipher(%d 字节) err = %v, want ErrKeySize", n, err)
			}
		}
	})
}

func TestCipherConcurrent(t *testing.T) {
	t.Run("TC-AES-008 同一 Cipher 多协程并发使用", func(t *testing.T) {
		key := mustHex(t, "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
		c := mustCipher(t, key)
		ref, err := stdaes.NewCipher(key)
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for g := range 8 {
			wg.Add(1)
			go func(seed int64) {
				defer wg.Done()
				rng := rand.New(rand.NewSource(seed))
				src := make([]byte, BlockSize)
				got := make([]byte, BlockSize)
				want := make([]byte, BlockSize)
				for range 2000 {
					rng.Read(src)
					c.Encrypt(got, src)
					ref.Encrypt(want, src)
					if !bytes.Equal(got, want) {
						errs <- errors.New("并发加密结果不一致")
						return
					}
					c.Decrypt(got, got)
					if !bytes.Equal(got, src) {
						errs <- errors.New("并发解密结果不一致")
						return
					}
				}
			}(int64(g))
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Error(err)
		}
	})
}

func BenchmarkEncryptBlock(b *testing.B) {
	c := mustCipher(b, make([]byte, KeySize))
	buf := make([]byte, BlockSize)
	b.SetBytes(BlockSize)
	for b.Loop() {
		c.Encrypt(buf, buf)
	}
}

func BenchmarkDecryptBlock(b *testing.B) {
	c := mustCipher(b, make([]byte, KeySize))
	buf := make([]byte, BlockSize)
	b.SetBytes(BlockSize)
	for b.Loop() {
		c.Decrypt(buf, buf)
	}
}
