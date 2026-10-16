// Package aes 手写实现 AES-256 分组密码（FIPS-197）以及 CBC 模式 + PKCS#7 填充的
// 流式加解密（NIST SP 800-38A）。不提供其他密钥长度与模式。
//
// 本文件是分组密码本体：S 盒与 GF(2^8) 乘法表在包初始化时按 FIPS-197 §4、§5.1.1
// 的数学定义生成（不内嵌常量表），然后按 §5.1（加密）、§5.2（密钥扩展）、
// §5.3（解密）逐步实现轮函数。实现以可读性优先，没有使用 T 表等合并优化。
//
// 状态（State）的排列遵循 FIPS-197 §3.4：16 字节输入 in[0..15] 按列装入 4×4 矩阵，
// s[r][c] = in[r+4c]。因此代码里 state[r+4*c] 就是第 r 行第 c 列。
package aes

import "errors"

// BlockSize 是 AES 分组长度（字节）。
const BlockSize = 16

// KeySize 是 AES-256 密钥长度（字节）。
const KeySize = 32

// FIPS-197 §2.2 / 图 4 中 AES-256 的参数。
const (
	nb = 4  // Nb：状态的列数（每列 4 字节，32 位字）
	nk = 8  // Nk：密钥的字数（256 位 / 32）
	nr = 14 // Nr：轮数

	// expandedWords 是扩展密钥的字数 Nb×(Nr+1)，每轮 4 个字（§5.2）。
	expandedWords = nb * (nr + 1)
)

var (
	// ErrKeySize 表示密钥不是 KeySize 字节。
	ErrKeySize = errors.New("aes: 密钥长度必须为 32 字节")
	// ErrIVSize 表示 IV 不是 BlockSize 字节。
	ErrIVSize = errors.New("aes: IV 长度必须为 16 字节")
	// ErrPadding 表示解密后 PKCS#7 填充非法或密文长度不是分组整数倍。
	ErrPadding = errors.New("aes: 填充非法")
)

// ---------------------------------------------------------------------------
// GF(2^8) 运算与查找表（FIPS-197 §4）
// ---------------------------------------------------------------------------

// xtime 计算 b·{02}（FIPS-197 §4.2.1）：左移一位，若最高位溢出则异或 {1b}，
// 即对不可约多项式 m(x) = x^8 + x^4 + x^3 + x + 1 取模。
func xtime(b byte) byte {
	if b&0x80 != 0 {
		return b<<1 ^ 0x1b
	}
	return b << 1
}

// gfMul 计算 GF(2^8) 中的乘法 a·b（FIPS-197 §4.2）：把 b 按二进制位展开，
// 对每个为 1 的位累加（异或）相应次数 xtime 后的 a。只在初始化建表时使用。
func gfMul(a, b byte) byte {
	var p byte
	for b != 0 {
		if b&1 != 0 {
			p ^= a
		}
		a = xtime(a)
		b >>= 1
	}
	return p
}

// 查找表。全部在 init 中由数学定义生成，之后只读，因此可被多个协程并发读取。
var (
	sbox    [256]byte // SubBytes 的 S 盒（§5.1.1 图 7）
	invSbox [256]byte // InvSubBytes 的逆 S 盒（§5.3.2 图 14）

	// mulN[x] = x·{N}，供 MixColumns（§5.1.3）与 InvMixColumns（§5.3.3）查表。
	mul2, mul3, mul9, mul11, mul13, mul14 [256]byte

	// rcon[i] 是轮常量字 Rcon[i] = [x^(i-1), 00, 00, 00]（§5.2），i 从 1 开始。
	rcon [nr/2 + 1]uint32
)

func init() {
	buildSBoxes()
	for i := range 256 {
		x := byte(i)
		mul2[i], mul3[i] = gfMul(x, 0x02), gfMul(x, 0x03)
		mul9[i], mul11[i] = gfMul(x, 0x09), gfMul(x, 0x0b)
		mul13[i], mul14[i] = gfMul(x, 0x0d), gfMul(x, 0x0e)
	}
	r := byte(0x01) // x^0
	for i := 1; i < len(rcon); i++ {
		rcon[i] = uint32(r) << 24
		r = xtime(r)
	}
}

// buildSBoxes 按 FIPS-197 §5.1.1 生成 S 盒：
//  1. 取字节在 GF(2^8) 中的乘法逆元（{00} 映射到自身）；
//  2. 做仿射变换 b'_i = b_i ⊕ b_(i+4) ⊕ b_(i+5) ⊕ b_(i+6) ⊕ b_(i+7) ⊕ c_i，c = {63}，
//     下标对 8 取模。等价于 b ⊕ rotl(b,1) ⊕ rotl(b,2) ⊕ rotl(b,3) ⊕ rotl(b,4) ⊕ {63}。
//
// 逆元借助生成元 {03} 的幂表 / 对数表求得：a = 3^k ⇒ a^-1 = 3^(255-k)。
func buildSBoxes() {
	var exp, log [256]byte
	x := byte(1)
	for i := range 255 {
		exp[i] = x
		log[x] = byte(i)
		x ^= xtime(x) // x·{03} = x·{02} ⊕ x
	}
	for i := range 256 {
		var inv byte
		if i != 0 {
			inv = exp[(255-int(log[i]))%255]
		}
		s := inv ^ rotl8(inv, 1) ^ rotl8(inv, 2) ^ rotl8(inv, 3) ^ rotl8(inv, 4) ^ 0x63
		sbox[i] = s
		invSbox[s] = byte(i)
	}
}

// rotl8 把字节循环左移 n 位。
func rotl8(b byte, n uint) byte {
	return b<<n | b>>(8-n)
}

// ---------------------------------------------------------------------------
// 密钥扩展（FIPS-197 §5.2）
// ---------------------------------------------------------------------------

// Cipher 是一个已完成密钥扩展的 AES-256 实例，可并发使用。
type Cipher struct {
	// w 是扩展密钥 w[0..Nb(Nr+1)-1]；第 round 轮的轮密钥为 w[4·round .. 4·round+3]。
	// 构造完成后只读，因此同一个 Cipher 可被多个协程同时使用。
	w [expandedWords]uint32
}

// NewCipher 对 key 做密钥扩展。key 不是 32 字节时返回 ErrKeySize。
// 返回的 Cipher 不引用 key 的底层数组。
func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != KeySize {
		return nil, ErrKeySize
	}
	c := &Cipher{}
	c.expandKey(key)
	return c, nil
}

// expandKey 实现 FIPS-197 §5.2 图 11 的 KeyExpansion（Nk = 8）：
//   - 前 Nk 个字直接取自密钥（大端拼接 4 字节为一个字）；
//   - 之后 w[i] = w[i-Nk] ⊕ temp，其中 temp = w[i-1]，并且
//     i mod Nk = 0 时 temp = SubWord(RotWord(temp)) ⊕ Rcon[i/Nk]，
//     i mod Nk = 4 时（仅 Nk > 6，即 AES-256）temp = SubWord(temp)。
func (c *Cipher) expandKey(key []byte) {
	for i := range nk {
		c.w[i] = uint32(key[4*i])<<24 | uint32(key[4*i+1])<<16 |
			uint32(key[4*i+2])<<8 | uint32(key[4*i+3])
	}
	for i := nk; i < expandedWords; i++ {
		temp := c.w[i-1]
		switch i % nk {
		case 0:
			temp = subWord(rotWord(temp)) ^ rcon[i/nk]
		case 4:
			temp = subWord(temp)
		}
		c.w[i] = c.w[i-nk] ^ temp
	}
}

// rotWord 把字 [a0,a1,a2,a3] 循环左移一个字节为 [a1,a2,a3,a0]（§5.2）。
func rotWord(w uint32) uint32 {
	return w<<8 | w>>24
}

// subWord 对字的 4 个字节分别查 S 盒（§5.2）。
func subWord(w uint32) uint32 {
	return uint32(sbox[w>>24])<<24 | uint32(sbox[w>>16&0xff])<<16 |
		uint32(sbox[w>>8&0xff])<<8 | uint32(sbox[w&0xff])
}

// ---------------------------------------------------------------------------
// 加密（FIPS-197 §5.1 Cipher）
// ---------------------------------------------------------------------------

// Encrypt 加密 src 的第一个分组写入 dst（可原地）。
// src 或 dst 不足 BlockSize 字节属于调用方编程错误，会 panic（与标准库一致）。
func (c *Cipher) Encrypt(dst, src []byte) {
	checkBlock(dst, src)
	var s [BlockSize]byte
	copy(s[:], src[:BlockSize]) // 先复制到局部状态，因此 dst 与 src 可以重叠

	// §5.1 图 5：初始轮密钥加 → Nr-1 个完整轮 → 最后一轮（无 MixColumns）。
	addRoundKey(&s, c.w[0:nb])
	for round := 1; round < nr; round++ {
		subBytes(&s)
		shiftRows(&s)
		mixColumns(&s)
		addRoundKey(&s, c.w[round*nb:(round+1)*nb])
	}
	subBytes(&s)
	shiftRows(&s)
	addRoundKey(&s, c.w[nr*nb:(nr+1)*nb])

	copy(dst[:BlockSize], s[:])
}

// subBytes 用 S 盒逐字节替换状态（§5.1.1）。
func subBytes(s *[BlockSize]byte) {
	for i := range s {
		s[i] = sbox[s[i]]
	}
}

// shiftRows 把第 r 行循环左移 r 个字节（§5.1.2）：s'[r][c] = s[r][(c+r) mod 4]。
func shiftRows(s *[BlockSize]byte) {
	t := *s
	for c := range nb {
		for r := 1; r < 4; r++ {
			s[r+4*c] = t[r+4*((c+r)%nb)]
		}
	}
}

// mixColumns 把每一列看作 GF(2^8) 上的多项式，乘以 a(x) = {03}x^3 + {01}x^2 + {01}x + {02}
// （§5.1.3），矩阵形式为：
//
//	[s0']   [02 03 01 01] [s0]
//	[s1'] = [01 02 03 01] [s1]
//	[s2']   [01 01 02 03] [s2]
//	[s3']   [03 01 01 02] [s3]
func mixColumns(s *[BlockSize]byte) {
	for c := range nb {
		a0, a1, a2, a3 := s[4*c], s[4*c+1], s[4*c+2], s[4*c+3]
		s[4*c] = mul2[a0] ^ mul3[a1] ^ a2 ^ a3
		s[4*c+1] = a0 ^ mul2[a1] ^ mul3[a2] ^ a3
		s[4*c+2] = a0 ^ a1 ^ mul2[a2] ^ mul3[a3]
		s[4*c+3] = mul3[a0] ^ a1 ^ a2 ^ mul2[a3]
	}
}

// addRoundKey 把第 c 列与轮密钥字 rk[c] 逐字节异或（§5.1.4）。
// 字的最高字节对应列的第 0 行。加密与解密共用（异或自逆，§5.3.4）。
func addRoundKey(s *[BlockSize]byte, rk []uint32) {
	for c := range nb {
		k := rk[c]
		s[4*c] ^= byte(k >> 24)
		s[4*c+1] ^= byte(k >> 16)
		s[4*c+2] ^= byte(k >> 8)
		s[4*c+3] ^= byte(k)
	}
}

// ---------------------------------------------------------------------------
// 解密（FIPS-197 §5.3 InvCipher）
// ---------------------------------------------------------------------------

// Decrypt 解密 src 的第一个分组写入 dst（可原地）。
// src 或 dst 不足 BlockSize 字节属于调用方编程错误，会 panic（与标准库一致）。
func (c *Cipher) Decrypt(dst, src []byte) {
	checkBlock(dst, src)
	var s [BlockSize]byte
	copy(s[:], src[:BlockSize])

	// §5.3 图 12：按与加密相反的顺序使用轮密钥，各步换成逆变换。
	addRoundKey(&s, c.w[nr*nb:(nr+1)*nb])
	for round := nr - 1; round >= 1; round-- {
		invShiftRows(&s)
		invSubBytes(&s)
		addRoundKey(&s, c.w[round*nb:(round+1)*nb])
		invMixColumns(&s)
	}
	invShiftRows(&s)
	invSubBytes(&s)
	addRoundKey(&s, c.w[0:nb])

	copy(dst[:BlockSize], s[:])
}

// invShiftRows 把第 r 行循环右移 r 个字节（§5.3.1）：s'[r][(c+r) mod 4] = s[r][c]。
func invShiftRows(s *[BlockSize]byte) {
	t := *s
	for c := range nb {
		for r := 1; r < 4; r++ {
			s[r+4*((c+r)%nb)] = t[r+4*c]
		}
	}
}

// invSubBytes 用逆 S 盒逐字节替换状态（§5.3.2）。
func invSubBytes(s *[BlockSize]byte) {
	for i := range s {
		s[i] = invSbox[s[i]]
	}
}

// invMixColumns 乘以 a(x) 的逆 a^-1(x) = {0b}x^3 + {0d}x^2 + {09}x + {0e}（§5.3.3）：
//
//	[s0']   [0e 0b 0d 09] [s0]
//	[s1'] = [09 0e 0b 0d] [s1]
//	[s2']   [0d 09 0e 0b] [s2]
//	[s3']   [0b 0d 09 0e] [s3]
func invMixColumns(s *[BlockSize]byte) {
	for c := range nb {
		a0, a1, a2, a3 := s[4*c], s[4*c+1], s[4*c+2], s[4*c+3]
		s[4*c] = mul14[a0] ^ mul11[a1] ^ mul13[a2] ^ mul9[a3]
		s[4*c+1] = mul9[a0] ^ mul14[a1] ^ mul11[a2] ^ mul13[a3]
		s[4*c+2] = mul13[a0] ^ mul9[a1] ^ mul14[a2] ^ mul11[a3]
		s[4*c+3] = mul11[a0] ^ mul13[a1] ^ mul9[a2] ^ mul14[a3]
	}
}

// checkBlock 确认 dst、src 至少各有一个分组。
func checkBlock(dst, src []byte) {
	if len(src) < BlockSize {
		panic("aes: 输入不足一个分组")
	}
	if len(dst) < BlockSize {
		panic("aes: 输出不足一个分组")
	}
}
