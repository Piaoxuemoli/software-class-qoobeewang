// Package hmac 手写实现 HMAC（RFC 2104），配合本项目的 sha256 使用。
// 接口与标准库 crypto/hmac 相同。
//
// 定义：HMAC(K, m) = H((K' ⊕ opad) ‖ H((K' ⊕ ipad) ‖ m))，其中 K' 是把密钥补零到
// 底层哈希分组长度的结果（密钥长于分组时先取 H(K)），ipad 为字节 0x36 重复、
// opad 为字节 0x5c 重复（RFC 2104 §2）。
package hmac

import "hash"

// RFC 2104 §2 规定的内外填充字节。
const (
	ipadByte = 0x36
	opadByte = 0x5c
)

// digest 是 HMAC 的流式计算状态：inner 持续吸收消息，outer 只在 Sum 时使用。
type digest struct {
	inner, outer hash.Hash
	ipad, opad   []byte // K' ⊕ ipad、K' ⊕ opad，长度为底层分组长度
}

// New 返回以 h 为底层哈希、key 为密钥的 HMAC。
func New(h func() hash.Hash, key []byte) hash.Hash {
	d := &digest{inner: h(), outer: h()}
	blockSize := d.inner.BlockSize()
	if len(key) > blockSize {
		// RFC 2104 §3：长于分组的密钥先哈希为 L 字节。
		d.outer.Write(key)
		key = d.outer.Sum(nil)
	}
	d.ipad = make([]byte, blockSize)
	d.opad = make([]byte, blockSize)
	copy(d.ipad, key)
	copy(d.opad, key)
	for i := range d.ipad {
		d.ipad[i] ^= ipadByte
		d.opad[i] ^= opadByte
	}
	d.Reset()
	return d
}

// Reset 恢复到刚创建时（已吸收 K' ⊕ ipad）的状态。
func (d *digest) Reset() {
	d.inner.Reset()
	d.inner.Write(d.ipad)
}

func (d *digest) Write(p []byte) (int, error) { return d.inner.Write(p) }

func (d *digest) Size() int { return d.outer.Size() }

func (d *digest) BlockSize() int { return d.inner.BlockSize() }

// Sum 把 MAC 追加到 b 后返回，不改变已吸收的消息状态（可继续 Write）。
func (d *digest) Sum(b []byte) []byte {
	origLen := len(b)
	b = d.inner.Sum(b) // 暂借 b 的尾部存放内层摘要
	d.outer.Reset()
	d.outer.Write(d.opad)
	d.outer.Write(b[origLen:])
	return d.outer.Sum(b[:origLen])
}

// Equal 以常量时间比较两个 MAC，长度不同直接返回 false。
//
// 对所有字节的异或结果做按位或累积，执行时间只取决于长度而与内容无关，
// 避免逐字节提前返回泄露"第几个字节开始不同"（MAC 长度本身不是秘密）。
func Equal(mac1, mac2 []byte) bool {
	if len(mac1) != len(mac2) {
		return false
	}
	var diff byte
	for i := range mac1 {
		diff |= mac1[i] ^ mac2[i]
	}
	return diff == 0
}
