// Package pbkdf2 手写实现 PBKDF2（RFC 8018 §5.2），用于从用户口令派生 AES 与 HMAC 密钥。
//
// 定义：DK = T1 ‖ T2 ‖ … 截取前 dkLen 字节，其中
// Ti = U1 ⊕ U2 ⊕ … ⊕ Uc，U1 = PRF(P, S ‖ INT(i))，Uj = PRF(P, Uj-1)，
// INT(i) 为 4 字节大端块序号（从 1 开始），PRF 为 HMAC(h)。
package pbkdf2

import (
	"encoding/binary"
	"hash"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/crypto/hmac"
)

// maxBlocks 是 RFC 8018 §5.2 步骤 1 允许的最大块数 2^32 - 1（块序号为 32 位）。
const maxBlocks = 1<<32 - 1

// Key 用伪随机函数 HMAC(h) 从 password、salt 派生 keyLen 字节密钥，迭代 iter 次。
// iter、keyLen 必须为正。
//
// iter ≤ 0、keyLen ≤ 0 或 keyLen 超过 (2^32-1)·h().Size() 属于调用方编程错误，
// 直接 panic（信息以 "pbkdf2: " 开头），不作为可恢复错误返回。
func Key(password, salt []byte, iter, keyLen int, h func() hash.Hash) []byte {
	if iter <= 0 {
		panic("pbkdf2: 迭代次数 iter 必须为正")
	}
	if keyLen <= 0 {
		panic("pbkdf2: 密钥长度 keyLen 必须为正")
	}
	prf := hmac.New(h, password)
	hLen := prf.Size()
	numBlocks := (keyLen + hLen - 1) / hLen
	if uint64(numBlocks) > maxBlocks {
		panic("pbkdf2: 密钥长度 keyLen 超过 (2^32-1)·hLen")
	}

	dk := make([]byte, 0, numBlocks*hLen)
	u := make([]byte, 0, hLen)
	var blockIndex [4]byte
	for i := 1; i <= numBlocks; i++ {
		// U1 = PRF(P, S ‖ INT(i))，T 初值为 U1。
		prf.Reset()
		prf.Write(salt)
		binary.BigEndian.PutUint32(blockIndex[:], uint32(i))
		prf.Write(blockIndex[:])
		dk = prf.Sum(dk)
		t := dk[len(dk)-hLen:]
		u = append(u[:0], t...)

		// Uj = PRF(P, Uj-1)，T ^= Uj。复用同一个 HMAC 实例与缓冲区，循环内不分配内存。
		for j := 2; j <= iter; j++ {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(u[:0])
			for k := range t {
				t[k] ^= u[k]
			}
		}
	}
	return dk[:keyLen]
}
