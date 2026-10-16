package pbkdf2

import (
	"bytes"
	stdpbkdf2 "crypto/pbkdf2"
	stdsha256 "crypto/sha256"
	"encoding/hex"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/crypto/sha256"
)

// benchIterations 与 container.DefaultIterations 保持一致（原语包不依赖 container）。
const benchIterations = 100_000

func mustHex(t testing.TB, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		t.Fatalf("解析十六进制 %q: %v", s, err)
	}
	return b
}

// RFC 7914 §11 给出的 PBKDF2-HMAC-SHA256 测试向量。
func TestRFC7914(t *testing.T) {
	cases := []struct {
		name     string
		password string
		salt     string
		iter     int
		want     string
	}{
		{"TC-KDF-001 RFC 7914 §11 passwd/salt c=1", "passwd", "salt", 1,
			"55 ac 04 6e 56 e3 08 9f ec 16 91 c2 25 44 b6 05" +
				"f9 41 85 21 6d de 04 65 e6 8b 9d 57 c2 0d ac bc" +
				"49 ca 9c cc f1 79 b6 45 99 16 64 b3 9d 77 ef 31" +
				"7c 71 b8 45 b1 e3 0b d5 09 11 20 41 d3 a1 97 83"},
		{"TC-KDF-002 RFC 7914 §11 Password/NaCl c=80000", "Password", "NaCl", 80000,
			"4d dc d8 f6 0b 98 be 21 83 0c ee 5e f2 27 01 f9" +
				"64 1a 44 18 d0 4c 04 14 ae ff 08 87 6b 34 ab 56" +
				"a1 d4 25 a1 22 58 33 54 9a db 84 1b 51 c9 b3 17" +
				"6a 27 2b de bb a1 d0 78 47 8f 62 b3 97 f3 3c 8d"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := mustHex(t, tc.want)
			got := Key([]byte(tc.password), []byte(tc.salt), tc.iter, len(want), sha256.New)
			if !bytes.Equal(got, want) {
				t.Errorf("Key = %x，期望 %x", got, want)
			}
		})
	}
}

func TestDifferentialRandom(t *testing.T) {
	t.Run("TC-KDF-003 随机口令/盐、多种 keyLen 与 crypto/pbkdf2 对照", func(t *testing.T) {
		r := rand.New(rand.NewPCG(5, 6))
		randBytes := func(n int) []byte {
			b := make([]byte, n)
			for i := range b {
				b[i] = byte(r.Uint32())
			}
			return b
		}
		for _, keyLen := range []int{1, 32, 33, 64, 100} {
			for _, iter := range []int{1, 2, 3, 17, 100} {
				// 口令长度覆盖空口令与超过分组长度（需先哈希）的情形。
				password := randBytes(r.IntN(100))
				salt := randBytes(r.IntN(40))
				want, err := stdpbkdf2.Key(stdsha256.New, string(password), salt, iter, keyLen)
				if err != nil {
					t.Fatalf("对照组 crypto/pbkdf2: %v", err)
				}
				got := Key(password, salt, iter, keyLen, sha256.New)
				if !bytes.Equal(got, want) {
					t.Fatalf("keyLen=%d iter=%d 口令 %d 字节 盐 %d 字节: %x，期望 %x",
						keyLen, iter, len(password), len(salt), got, want)
				}
			}
		}
	})

	t.Run("TC-KDF-004 短密钥是长密钥的前缀且长度精确", func(t *testing.T) {
		long := Key([]byte("pw"), []byte("salt"), 10, 100, sha256.New)
		for _, n := range []int{1, 31, 32, 33, 64, 99} {
			short := Key([]byte("pw"), []byte("salt"), 10, n, sha256.New)
			if len(short) != n || !bytes.Equal(short, long[:n]) {
				t.Fatalf("keyLen=%d: %x 不是 %x 的前缀", n, short, long)
			}
		}
	})
}

func TestInvalidArgsPanic(t *testing.T) {
	cases := []struct {
		name         string
		iter, keyLen int
	}{
		{"TC-KDF-005 iter 为 0 时 panic", 0, 32},
		{"TC-KDF-006 iter 为负时 panic", -1, 32},
		{"TC-KDF-007 keyLen 为 0 时 panic", 1, 0},
		{"TC-KDF-008 keyLen 为负时 panic", 1, -5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				msg, ok := recover().(string)
				if !ok || !strings.HasPrefix(msg, "pbkdf2: ") {
					t.Fatalf("期望以 \"pbkdf2: \" 开头的 panic 信息，得到 %q", msg)
				}
			}()
			Key([]byte("pw"), []byte("salt"), tc.iter, tc.keyLen, sha256.New)
		})
	}
}

func BenchmarkPBKDF2(b *testing.B) {
	password, salt := []byte("correct horse battery staple"), make([]byte, 16)
	for b.Loop() {
		Key(password, salt, benchIterations, 64, sha256.New)
	}
}
