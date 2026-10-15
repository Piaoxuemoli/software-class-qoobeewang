// Package pbkdf2 手写实现 PBKDF2（RFC 8018 §5.2），用于从用户口令派生 AES 与 HMAC 密钥。
package pbkdf2

import "hash"

// Key 用伪随机函数 HMAC(h) 从 password、salt 派生 keyLen 字节密钥，迭代 iter 次。
// iter、keyLen 必须为正。
func Key(password, salt []byte, iter, keyLen int, h func() hash.Hash) []byte {
	panic("pbkdf2: 尚未实现")
}
