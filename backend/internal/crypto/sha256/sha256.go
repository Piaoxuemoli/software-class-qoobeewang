// Package sha256 手写实现 SHA-256（FIPS 180-4）。接口与标准库 crypto/sha256 相同，
// 以便在测试中直接与标准库对照（标准库只允许出现在 _test.go 中）。
package sha256

import "hash"

// Size 是摘要长度（字节）。
const Size = 32

// BlockSize 是分组长度（字节）。
const BlockSize = 64

// New 返回一个计算 SHA-256 的 hash.Hash。
func New() hash.Hash {
	panic("sha256: 尚未实现")
}

// Sum256 返回 data 的 SHA-256 摘要。
func Sum256(data []byte) [Size]byte {
	panic("sha256: 尚未实现")
}
