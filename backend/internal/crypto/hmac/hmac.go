// Package hmac 手写实现 HMAC（RFC 2104），配合本项目的 sha256 使用。
// 接口与标准库 crypto/hmac 相同。
package hmac

import "hash"

// New 返回以 h 为底层哈希、key 为密钥的 HMAC。
func New(h func() hash.Hash, key []byte) hash.Hash {
	panic("hmac: 尚未实现")
}

// Equal 以常量时间比较两个 MAC，长度不同直接返回 false。
func Equal(mac1, mac2 []byte) bool {
	panic("hmac: 尚未实现")
}
