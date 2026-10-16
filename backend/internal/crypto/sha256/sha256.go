// Package sha256 手写实现 SHA-256（FIPS 180-4）。接口与标准库 crypto/sha256 相同，
// 以便在测试中直接与标准库对照（标准库只允许出现在 _test.go 中）。
//
// 算法概要：消息按 64 字节分组，每组经 64 轮压缩函数更新 8 个 32 位工作变量；
// 结束时按 §5.1.1 填充并追加消息比特长度，最终 8 个字（大端）即 32 字节摘要。
// 实现流式处理：未满一组的尾部缓存在 digest.x 中，内存占用恒定。
package sha256

import (
	"encoding/binary"
	"hash"
	"math/bits"
)

// Size 是摘要长度（字节）。
const Size = 32

// BlockSize 是分组长度（字节）。
const BlockSize = 64

// lenFieldSize 是填充末尾"消息比特长度"字段的字节数（FIPS 180-4 §5.1.1：64 位大端）。
const lenFieldSize = 8

// initH 是初始哈希值 H(0)：前 8 个素数平方根小数部分的前 32 位（FIPS 180-4 §5.3.3）。
var initH = [8]uint32{
	0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a,
	0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19,
}

// roundK 是 64 个轮常量：前 64 个素数立方根小数部分的前 32 位（FIPS 180-4 §4.2.2）。
var roundK = [64]uint32{
	0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
	0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
	0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
	0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
	0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
	0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
	0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
	0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
}

// digest 是 SHA-256 的流式计算状态。
type digest struct {
	h   [8]uint32       // 当前哈希值 H(i)
	x   [BlockSize]byte // 尚未凑满一组的输入
	nx  int             // x 中有效字节数
	len uint64          // 已写入的消息总字节数
}

// New 返回一个计算 SHA-256 的 hash.Hash。
func New() hash.Hash {
	d := new(digest)
	d.Reset()
	return d
}

// Sum256 返回 data 的 SHA-256 摘要。
func Sum256(data []byte) [Size]byte {
	var d digest
	d.Reset()
	d.Write(data)
	return d.checkSum()
}

func (d *digest) Reset() {
	d.h = initH
	d.nx = 0
	d.len = 0
}

func (d *digest) Size() int { return Size }

func (d *digest) BlockSize() int { return BlockSize }

// Write 吸收 p；按 hash.Hash 约定永不返回错误。
func (d *digest) Write(p []byte) (int, error) {
	n := len(p)
	d.len += uint64(n)
	if d.nx > 0 {
		c := copy(d.x[d.nx:], p)
		d.nx += c
		p = p[c:]
		if d.nx < BlockSize {
			return n, nil
		}
		block(&d.h, d.x[:])
		d.nx = 0
	}
	if full := len(p) &^ (BlockSize - 1); full > 0 {
		block(&d.h, p[:full])
		p = p[full:]
	}
	d.nx = copy(d.x[:], p)
	return n, nil
}

// Sum 把当前摘要追加到 b 后返回；在副本上完成填充，不改变 d 的状态。
func (d *digest) Sum(b []byte) []byte {
	d0 := *d
	sum := d0.checkSum()
	return append(b, sum[:]...)
}

// checkSum 完成填充并输出摘要（会改变 d，调用方应在副本上调用）。
//
// 填充规则（FIPS 180-4 §5.1.1）：追加 1 个比特 1（字节 0x80），再追加若干 0，
// 使长度 ≡ 56 (mod 64)，最后追加 8 字节大端的消息比特长度，总长恰为 64 的整数倍。
func (d *digest) checkSum() [Size]byte {
	bitLen := d.len << 3
	var pad [BlockSize + lenFieldSize]byte
	pad[0] = 0x80
	padLen := (BlockSize - lenFieldSize) - int(d.len%BlockSize)
	if padLen <= 0 {
		padLen += BlockSize
	}
	binary.BigEndian.PutUint64(pad[padLen:], bitLen)
	d.Write(pad[:padLen+lenFieldSize])

	var out [Size]byte
	for i, v := range d.h {
		binary.BigEndian.PutUint32(out[i*4:], v)
	}
	return out
}

// block 对 p 中每个 64 字节分组执行压缩函数（FIPS 180-4 §6.2.2），len(p) 必须是 64 的倍数。
func block(h *[8]uint32, p []byte) {
	var w [64]uint32
	for ; len(p) >= BlockSize; p = p[BlockSize:] {
		// 消息调度：W0..W15 为分组的 16 个大端字，
		// Wt = σ1(Wt-2) + Wt-7 + σ0(Wt-15) + Wt-16（§4.1.2 式 4.6、4.7）。
		for t := 0; t < 16; t++ {
			w[t] = binary.BigEndian.Uint32(p[t*4:])
		}
		for t := 16; t < 64; t++ {
			v1, v2 := w[t-2], w[t-15]
			s1 := bits.RotateLeft32(v1, -17) ^ bits.RotateLeft32(v1, -19) ^ (v1 >> 10)
			s0 := bits.RotateLeft32(v2, -7) ^ bits.RotateLeft32(v2, -18) ^ (v2 >> 3)
			w[t] = s1 + w[t-7] + s0 + w[t-16]
		}

		a, b, c, dd, e, f, g, hh := h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7]
		for t := 0; t < 64; t++ {
			// Σ1、Ch、Σ0、Maj 见 §4.1.2 式 4.2–4.5；ROTR^n 用循环左移 -n 实现。
			bigS1 := bits.RotateLeft32(e, -6) ^ bits.RotateLeft32(e, -11) ^ bits.RotateLeft32(e, -25)
			ch := (e & f) ^ (^e & g)
			t1 := hh + bigS1 + ch + roundK[t] + w[t]
			bigS0 := bits.RotateLeft32(a, -2) ^ bits.RotateLeft32(a, -13) ^ bits.RotateLeft32(a, -22)
			maj := (a & b) ^ (a & c) ^ (b & c)
			t2 := bigS0 + maj
			hh, g, f, e, dd, c, b, a = g, f, e, dd+t1, c, b, a, t1+t2
		}
		h[0] += a
		h[1] += b
		h[2] += c
		h[3] += dd
		h[4] += e
		h[5] += f
		h[6] += g
		h[7] += hh
	}
}
