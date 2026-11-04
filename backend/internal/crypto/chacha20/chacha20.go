// Package chacha20 手写实现 ChaCha20 流密码（RFC 8439 §2.3）：256 位密钥、
// 96 位 nonce、32 位块计数器、20 轮。与 aes 包的 CBC 封装一样只提供容器层
// 需要的形态，此处是密码本体：把明文/密文与连续的密钥流块异或。流密码本身
// 无填充、无认证——完整性由容器层的 encrypt-then-MAC（HMAC-SHA256）保证，
// 因此密文被篡改时到不了解密端，也不存在密钥流重用的可利用面。
//
// 核心是 4×4 的 32 位字状态矩阵（RFC 8439 §2.3）：
//
//	c0 c1 c2 c3   常量 "expand 32-byte k"
//	k0 k1 k2 k3   密钥前半（小端字）
//	k4 k5 k6 k7   密钥后半
//	n0 n1 ctr     nonce（小端字）与块计数器
//
// 每个密钥流块 = 状态经过 20 轮（10 次"列轮 + 对角轮"）后加上初始状态，
// 按小端序序列化为 64 字节。实现只用加法、异或与循环左移（ARX），无查表，
// 不存在基于 S 盒的时序泄漏面。
package chacha20

import (
	"encoding/binary"
	"errors"
	"math/bits"
)

// KeySize 是密钥长度（字节），RFC 8439 只定义 256 位密钥。
const KeySize = 32

// NonceSize 是 nonce 长度（字节），RFC 8439 §2.3 采用 96 位 nonce。
const NonceSize = 12

var (
	// ErrKeySize 表示密钥不是 KeySize 字节。
	ErrKeySize = errors.New("chacha20: 密钥长度必须为 32 字节")
	// ErrNonceSize 表示 nonce 不是 NonceSize 字节。
	ErrNonceSize = errors.New("chacha20: nonce 长度必须为 12 字节")
)

// 状态矩阵前四个常量字 "expa" "nd 3" "2-by" "te k"（RFC 8439 §2.3）。
const (
	const0 = 0x61707865
	const1 = 0x3320646e
	const2 = 0x79622d32
	const3 = 0x6b206574
)

// quarterRound 是 RFC 8439 §2.1 的四分之一轮 QR(a, b, c, d)：
//
//	a += b; d ^= a; d <<<= 16;   c += d; b ^= c; b <<<= 12;
//	a += b; d ^= a; d <<<= 8;    c += d; b ^= c; b <<<= 7
func quarterRound(a, b, c, d uint32) (uint32, uint32, uint32, uint32) {
	a += b
	d ^= a
	d = bits.RotateLeft32(d, 16)
	c += d
	b ^= c
	b = bits.RotateLeft32(b, 12)
	a += b
	d ^= a
	d = bits.RotateLeft32(d, 8)
	c += d
	b ^= c
	b = bits.RotateLeft32(b, 7)
	return a, b, c, d
}

// Cipher 按 (key, nonce) 顺序生成密钥流，支持任意分段异或；同一 Cipher
// 不能并发使用。
type Cipher struct {
	key   [8]uint32
	nonce [3]uint32
	block [64]byte // 当前密钥流块
	off   int      // block 中已被消费的字节数；等于 64 表示需要生成下一块
	next  uint32   // 下一个待生成的块计数器，从 1 开始（RFC 8439 §2.4）
}

// NewCipher 用 key 与 nonce 构造密钥流生成器。key 不是 32 字节返回
// ErrKeySize，nonce 不是 12 字节返回 ErrNonceSize。
func NewCipher(key, nonce []byte) (*Cipher, error) {
	if len(key) != KeySize {
		return nil, ErrKeySize
	}
	if len(nonce) != NonceSize {
		return nil, ErrNonceSize
	}
	c := &Cipher{off: 64, next: 1} // off 满 64 迫使第一次异或先生成第 1 块
	for i := range c.key {
		c.key[i] = binary.LittleEndian.Uint32(key[4*i:])
	}
	for i := range c.nonce {
		c.nonce[i] = binary.LittleEndian.Uint32(nonce[4*i:])
	}
	return c, nil
}

// keystreamBlock 生成第 ctr 块密钥流写入 out（RFC 8439 §2.3 的块函数）。
func keystreamBlock(key *[8]uint32, nonce *[3]uint32, ctr uint32, out *[64]byte) {
	s := [16]uint32{
		const0, const1, const2, const3,
		key[0], key[1], key[2], key[3],
		key[4], key[5], key[6], key[7],
		ctr, nonce[0], nonce[1], nonce[2],
	}
	x := s
	for i := 0; i < 10; i++ { // 10 次双轮 = 20 轮
		// 列轮：QR 作用于每一列。
		x[0], x[4], x[8], x[12] = quarterRound(x[0], x[4], x[8], x[12])
		x[1], x[5], x[9], x[13] = quarterRound(x[1], x[5], x[9], x[13])
		x[2], x[6], x[10], x[14] = quarterRound(x[2], x[6], x[10], x[14])
		x[3], x[7], x[11], x[15] = quarterRound(x[3], x[7], x[11], x[15])
		// 对角轮：QR 作用于错开的对角线。
		x[0], x[5], x[10], x[15] = quarterRound(x[0], x[5], x[10], x[15])
		x[1], x[6], x[11], x[12] = quarterRound(x[1], x[6], x[11], x[12])
		x[2], x[7], x[8], x[13] = quarterRound(x[2], x[7], x[8], x[13])
		x[3], x[4], x[9], x[14] = quarterRound(x[3], x[4], x[9], x[14])
	}
	for i := range x {
		binary.LittleEndian.PutUint32(out[4*i:], x[i]+s[i])
	}
}

// XORKeyStream 把 src 与密钥流异或后写入 dst（dst 与 src 可以是同一切片，
// 原地异或）。一个 (key, nonce) 最多生成 2^32-1 块 ≈ 256 GiB 密钥流；
// 超出属调用方错误，直接 panic，绝不静默回绕计数器导致密钥流重用。
func (c *Cipher) XORKeyStream(dst, src []byte) {
	if len(dst) < len(src) {
		panic("chacha20: dst 长度小于 src")
	}
	for len(src) > 0 {
		if c.off == len(c.block) {
			if c.next == 0 { // 计数器已耗尽（0xFFFFFFFF 块之后回绕）
				panic("chacha20: 密钥流超过 256 GiB 上限")
			}
			keystreamBlock(&c.key, &c.nonce, c.next, &c.block)
			c.next++
			c.off = 0
		}
		n := min(len(src), len(c.block)-c.off)
		for i := 0; i < n; i++ {
			dst[i] = src[i] ^ c.block[c.off+i]
		}
		dst, src = dst[n:], src[n:]
		c.off += n
	}
}
