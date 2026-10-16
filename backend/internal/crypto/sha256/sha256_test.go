package sha256

import (
	"bytes"
	stdsha256 "crypto/sha256"
	"encoding/hex"
	"io"
	"math/big"
	"math/rand/v2"
	"strings"
	"testing"
)

func mustHex(t testing.TB, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("解析十六进制 %q: %v", s, err)
	}
	return b
}

// writeChunked 把 data 按随机长度切块写入 h，覆盖缓冲区跨块拼接的各种情形。
func writeChunked(t testing.TB, h io.Writer, data []byte, r *rand.Rand) {
	t.Helper()
	for len(data) > 0 {
		n := r.IntN(min(len(data), 3*BlockSize) + 1)
		if wn, err := h.Write(data[:n]); wn != n || err != nil {
			t.Fatalf("Write(%d 字节) = %d, %v", n, wn, err)
		}
		data = data[n:]
	}
}

// 公开测试向量：abc、448 位消息、百万 'a' 见 FIPS 180-2 附录 B；空消息与 896 位消息见
// NIST CSRC 公布的 SHA-256 示例值。
func TestVectors(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want string
	}{
		{"TC-SHA-001 空消息", "",
			"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"TC-SHA-002 abc（单块）", "abc",
			"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{"TC-SHA-003 448 位消息（两块）", "abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq",
			"248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1"},
		{"TC-SHA-004 896 位消息",
			"abcdefghbcdefghicdefghijdefghijkefghijklfghijklmghijklmnhijklmnoijklmnopjklmnopqklmnopqrlmnopqrsmnopqrstnopqrstu",
			"cf5b16a778af8380036ce59e7b0492370b249b11e8f07a51afac45037afee9d1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := mustHex(t, tc.want)
			if got := Sum256([]byte(tc.msg)); !bytes.Equal(got[:], want) {
				t.Errorf("Sum256 = %x，期望 %x", got, want)
			}
			h := New()
			h.Write([]byte(tc.msg))
			if got := h.Sum(nil); !bytes.Equal(got, want) {
				t.Errorf("New().Sum = %x，期望 %x", got, want)
			}
		})
	}

	t.Run("TC-SHA-005 一百万个 a（分块流式写入）", func(t *testing.T) {
		want := mustHex(t, "cdc76e5c9914fb9281a1c7e284d73e67f1809a48a497200e046d39ccc7112cd0")
		// 997 是素数，块边界与 64 字节分组错开，验证流式缓冲。
		chunk := []byte(strings.Repeat("a", 997))
		h := New()
		for left := 1_000_000; left > 0; left -= len(chunk) {
			h.Write(chunk[:min(left, len(chunk))])
		}
		if got := h.Sum(nil); !bytes.Equal(got, want) {
			t.Errorf("Sum = %x，期望 %x", got, want)
		}
	})
}

func TestPaddingBoundaries(t *testing.T) {
	t.Run("TC-SHA-006 长度 0..130 与标准库一致", func(t *testing.T) {
		data := make([]byte, 130)
		for i := range data {
			data[i] = byte(i*7 + 3)
		}
		for n := 0; n <= len(data); n++ {
			msg := data[:n]
			want := stdsha256.Sum256(msg)
			if got := Sum256(msg); got != want {
				t.Fatalf("长度 %d: Sum256 = %x，期望 %x", n, got, want)
			}
			h := New()
			for i := range msg {
				h.Write(msg[i : i+1])
			}
			if got := h.Sum(nil); !bytes.Equal(got, want[:]) {
				t.Fatalf("长度 %d: 逐字节写入 Sum = %x，期望 %x", n, got, want)
			}
		}
	})
}

func TestDifferentialRandom(t *testing.T) {
	t.Run("TC-SHA-007 随机数据随机分块与标准库对照", func(t *testing.T) {
		r := rand.New(rand.NewPCG(1, 2))
		for i := 0; i < 300; i++ {
			data := make([]byte, r.IntN(5000))
			for j := range data {
				data[j] = byte(r.Uint32())
			}
			h := New()
			writeChunked(t, h, data, r)
			want := stdsha256.Sum256(data)
			if got := h.Sum(nil); !bytes.Equal(got, want[:]) {
				t.Fatalf("第 %d 组（%d 字节）: %x，期望 %x", i, len(data), got, want)
			}
		}
	})
}

func TestHashSemantics(t *testing.T) {
	t.Run("TC-SHA-008 Sum 不改变状态且可继续写入", func(t *testing.T) {
		a := []byte("hello, ")
		b := bytes.Repeat([]byte("world"), 30)
		h := New()
		h.Write(a)
		s1 := h.Sum(nil)
		if s1again := h.Sum(nil); !bytes.Equal(s1, s1again) {
			t.Fatalf("连续两次 Sum 不同: %x / %x", s1, s1again)
		}
		wantA := stdsha256.Sum256(a)
		if !bytes.Equal(s1, wantA[:]) {
			t.Fatalf("Sum(a) = %x，期望 %x", s1, wantA)
		}
		h.Write(b)
		prefix := []byte("prefix")
		s2 := h.Sum(prefix)
		wantAB := stdsha256.Sum256(append(append([]byte{}, a...), b...))
		if !bytes.HasPrefix(s2, prefix) || !bytes.Equal(s2[len(prefix):], wantAB[:]) {
			t.Fatalf("Sum(prefix) = %x，期望 prefix 后接 %x", s2, wantAB)
		}
	})

	t.Run("TC-SHA-009 Reset 恢复初始状态", func(t *testing.T) {
		h := New()
		h.Write(bytes.Repeat([]byte{0xff}, 100))
		h.Sum(nil)
		h.Reset()
		h.Write([]byte("abc"))
		want := mustHex(t, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad")
		if got := h.Sum(nil); !bytes.Equal(got, want) {
			t.Errorf("Reset 后 Sum = %x，期望 %x", got, want)
		}
	})

	t.Run("TC-SHA-010 Size/BlockSize 与 Write 返回值", func(t *testing.T) {
		h := New()
		if h.Size() != Size || Size != 32 {
			t.Errorf("Size = %d，期望 32", h.Size())
		}
		if h.BlockSize() != BlockSize || BlockSize != 64 {
			t.Errorf("BlockSize = %d，期望 64", h.BlockSize())
		}
		if n, err := h.Write(make([]byte, 123)); n != 123 || err != nil {
			t.Errorf("Write = %d, %v，期望 123, nil", n, err)
		}
	})
}

// firstPrimes 返回前 n 个素数。
func firstPrimes(n int) []int64 {
	var ps []int64
	for c := int64(2); len(ps) < n; c++ {
		if big.NewInt(c).ProbablyPrime(0) {
			ps = append(ps, c)
		}
	}
	return ps
}

// intRoot 返回 floor(x 的 k 次方根)（二分查找，精确整数运算）。
func intRoot(x *big.Int, k int64) *big.Int {
	lo, hi := big.NewInt(0), new(big.Int).Add(x, big.NewInt(1))
	kk := big.NewInt(k)
	for new(big.Int).Sub(hi, lo).Cmp(big.NewInt(1)) > 0 {
		mid := new(big.Int).Rsh(new(big.Int).Add(lo, hi), 1)
		if new(big.Int).Exp(mid, kk, nil).Cmp(x) <= 0 {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// fracBits32 返回 p 的 k 次方根小数部分的前 32 位：floor(p^(1/k) · 2^32) mod 2^32。
func fracBits32(p, k int64) uint32 {
	x := new(big.Int).Lsh(big.NewInt(p), uint(32*k))
	return uint32(intRoot(x, k).Uint64())
}

func TestConstants(t *testing.T) {
	t.Run("TC-SHA-011 初始值与轮常量可由素数方根推导（FIPS 180-4 §4.2.2、§5.3.3）", func(t *testing.T) {
		primes := firstPrimes(64)
		for i, p := range primes[:8] {
			if want := fracBits32(p, 2); initH[i] != want {
				t.Errorf("H0[%d] = %#08x，期望 %#08x（√%d）", i, initH[i], want, p)
			}
		}
		for i, p := range primes {
			if want := fracBits32(p, 3); roundK[i] != want {
				t.Errorf("K[%d] = %#08x，期望 %#08x（∛%d）", i, roundK[i], want, p)
			}
		}
	})
}

func benchmarkSum256(b *testing.B, size int) {
	data := make([]byte, size)
	b.SetBytes(int64(size))
	for b.Loop() {
		Sum256(data)
	}
}

func BenchmarkSum256(b *testing.B) {
	b.Run("1KiB", func(b *testing.B) { benchmarkSum256(b, 1<<10) })
	b.Run("1MiB", func(b *testing.B) { benchmarkSum256(b, 1<<20) })
}
