package hmac

import (
	"bytes"
	stdhmac "crypto/hmac"
	stdsha256 "crypto/sha256"
	"encoding/hex"
	"hash"
	"math/rand/v2"
	"testing"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/crypto/sha256"
)

func mustHex(t testing.TB, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("解析十六进制 %q: %v", s, err)
	}
	return b
}

func macOf(h func() hash.Hash, key, data []byte) []byte {
	m := New(h, key)
	m.Write(data)
	return m.Sum(nil)
}

// RFC 4231 §4 中 HMAC-SHA-256 的测试用例 1–7。
func TestRFC4231(t *testing.T) {
	key131 := bytes.Repeat([]byte{0xaa}, 131)
	cases := []struct {
		name string
		key  []byte
		data []byte
		want string // 用例 5 只给出前 128 位
	}{
		{"TC-SHA-101 RFC 4231 用例 1", bytes.Repeat([]byte{0x0b}, 20), []byte("Hi There"),
			"b0344c61d8db38535ca8afceaf0bf12b881dc200c9833da726e9376c2e32cff7"},
		{"TC-SHA-102 RFC 4231 用例 2（短密钥）", []byte("Jefe"), []byte("what do ya want for nothing?"),
			"5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843"},
		{"TC-SHA-103 RFC 4231 用例 3", bytes.Repeat([]byte{0xaa}, 20), bytes.Repeat([]byte{0xdd}, 50),
			"773ea91e36800e46854db8ebd09181a72959098b3ef8c122d9635514ced565fe"},
		{"TC-SHA-104 RFC 4231 用例 4", mustHex(t, "0102030405060708090a0b0c0d0e0f10111213141516171819"),
			bytes.Repeat([]byte{0xcd}, 50),
			"82558a389a443c0ea4cc819899f2083a85f0faa3e578f8077a2e3ff46729665b"},
		{"TC-SHA-105 RFC 4231 用例 5（截断到 128 位）", bytes.Repeat([]byte{0x0c}, 20), []byte("Test With Truncation"),
			"a3b6167473100ee06e0c796c2955552b"},
		{"TC-SHA-106 RFC 4231 用例 6（131 字节密钥先哈希）", key131,
			[]byte("Test Using Larger Than Block-Size Key - Hash Key First"),
			"60e431591ee0b67f0d8a26aacbf5b77f8e0bc6213728c5140546040f0ee37f54"},
		{"TC-SHA-107 RFC 4231 用例 7（长密钥 + 长数据）", key131,
			[]byte("This is a test using a larger than block-size key and a larger than block-size data. The key needs to be hashed before being used by the HMAC algorithm."),
			"9b09ffa71b942fcb27635fbcd5b0e944bfdc63644f0713938a7f51535c3a35e2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := mustHex(t, tc.want)
			got := macOf(sha256.New, tc.key, tc.data)
			if len(got) != sha256.Size {
				t.Fatalf("MAC 长度 = %d，期望 %d", len(got), sha256.Size)
			}
			if !bytes.Equal(got[:len(want)], want) {
				t.Errorf("HMAC = %x，期望前缀 %x", got, want)
			}
		})
	}
}

func TestDifferentialRandom(t *testing.T) {
	t.Run("TC-SHA-108 随机密钥与数据和 crypto/hmac 对照", func(t *testing.T) {
		r := rand.New(rand.NewPCG(3, 4))
		randBytes := func(n int) []byte {
			b := make([]byte, n)
			for i := range b {
				b[i] = byte(r.Uint32())
			}
			return b
		}
		// 显式覆盖空密钥、恰为分组长度、比分组长 1 字节等边界。
		keyLens := []int{0, 1, 31, 32, 63, 64, 65, 128, 200}
		for i := 0; i < 200; i++ {
			kl := keyLens[i%len(keyLens)]
			if i >= len(keyLens) {
				kl = r.IntN(200)
			}
			key, data := randBytes(kl), randBytes(r.IntN(1000))

			want := stdhmac.New(stdsha256.New, key)
			want.Write(data)
			wantSum := want.Sum(nil)

			// 分别用本项目的 sha256 和标准库 sha256 作为底层哈希，隔离 HMAC 逻辑本身的错误。
			for _, under := range []struct {
				name string
				h    func() hash.Hash
			}{{"本项目 sha256", sha256.New}, {"标准库 sha256", stdsha256.New}} {
				m := New(under.h, key)
				for rest := data; len(rest) > 0; {
					n := r.IntN(len(rest) + 1)
					m.Write(rest[:n])
					rest = rest[n:]
				}
				if got := m.Sum(nil); !bytes.Equal(got, wantSum) {
					t.Fatalf("第 %d 组（%s，密钥 %d 字节，数据 %d 字节）: %x，期望 %x",
						i, under.name, len(key), len(data), got, wantSum)
				}
			}
		}
	})
}

func TestHashSemantics(t *testing.T) {
	key := []byte("key")
	t.Run("TC-SHA-109 Sum 不改变状态、Reset 恢复带密钥的初始状态", func(t *testing.T) {
		m := New(sha256.New, key)
		m.Write([]byte("part1"))
		s1 := m.Sum(nil)
		if !bytes.Equal(s1, m.Sum(nil)) {
			t.Fatal("连续两次 Sum 结果不同")
		}
		m.Write([]byte("part2"))
		if got, want := m.Sum(nil), macOf(sha256.New, key, []byte("part1part2")); !bytes.Equal(got, want) {
			t.Fatalf("Sum 后继续写入: %x，期望 %x", got, want)
		}
		m.Reset()
		m.Write([]byte("fresh"))
		if got, want := m.Sum(nil), macOf(sha256.New, key, []byte("fresh")); !bytes.Equal(got, want) {
			t.Fatalf("Reset 后: %x，期望 %x", got, want)
		}
	})

	t.Run("TC-SHA-110 New 复制密钥，调用方随后修改密钥不影响结果", func(t *testing.T) {
		k := []byte("mutable-key")
		m := New(sha256.New, k)
		want := macOf(sha256.New, []byte("mutable-key"), []byte("msg"))
		for i := range k {
			k[i] = 0
		}
		m.Write([]byte("msg"))
		if got := m.Sum(nil); !bytes.Equal(got, want) {
			t.Fatalf("修改密钥切片后 MAC 改变: %x，期望 %x", got, want)
		}
	})

	t.Run("TC-SHA-111 Size/BlockSize 取自底层哈希", func(t *testing.T) {
		m := New(sha256.New, key)
		if m.Size() != sha256.Size || m.BlockSize() != sha256.BlockSize {
			t.Errorf("Size/BlockSize = %d/%d，期望 %d/%d", m.Size(), m.BlockSize(), sha256.Size, sha256.BlockSize)
		}
	})
}

func TestEqual(t *testing.T) {
	mac := macOf(sha256.New, []byte("k"), []byte("m"))
	flipped := append([]byte{}, mac...)
	flipped[len(flipped)-1] ^= 0x01
	cases := []struct {
		name string
		a, b []byte
		want bool
	}{
		{"TC-SHA-112 相同 MAC", mac, append([]byte{}, mac...), true},
		{"TC-SHA-113 末位比特不同", mac, flipped, false},
		{"TC-SHA-114 长度不同", mac, mac[:len(mac)-1], false},
		{"TC-SHA-115 均为空", nil, []byte{}, true},
		{"TC-SHA-116 一空一非空", nil, mac, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Equal(tc.a, tc.b); got != tc.want {
				t.Errorf("Equal = %v，期望 %v", got, tc.want)
			}
			if got := stdhmac.Equal(tc.a, tc.b); got != tc.want {
				t.Fatalf("对照组 crypto/hmac.Equal = %v，用例期望有误", got)
			}
		})
	}
}

func BenchmarkHMACSHA256(b *testing.B) {
	m := New(sha256.New, []byte("benchmark-key"))
	data := make([]byte, 1<<10)
	out := make([]byte, 0, sha256.Size)
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		m.Reset()
		m.Write(data)
		out = m.Sum(out[:0])
	}
}
