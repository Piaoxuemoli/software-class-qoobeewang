// Package aes 手写实现 AES-256 分组密码（FIPS-197）以及 CBC 模式 + PKCS#7 填充的
// 流式加解密（NIST SP 800-38A）。不提供其他密钥长度与模式。
package aes

import (
	"errors"
	"io"
)

var errNotImplemented = errors.New("aes: 尚未实现")

// BlockSize 是 AES 分组长度（字节）。
const BlockSize = 16

// KeySize 是 AES-256 密钥长度（字节）。
const KeySize = 32

var (
	// ErrKeySize 表示密钥不是 KeySize 字节。
	ErrKeySize = errors.New("aes: 密钥长度必须为 32 字节")
	// ErrIVSize 表示 IV 不是 BlockSize 字节。
	ErrIVSize = errors.New("aes: IV 长度必须为 16 字节")
	// ErrPadding 表示解密后 PKCS#7 填充非法或密文长度不是分组整数倍。
	ErrPadding = errors.New("aes: 填充非法")
)

// Cipher 是一个已完成密钥扩展的 AES-256 实例，可并发使用。
type Cipher struct{}

// NewCipher 对 key 做密钥扩展。key 不是 32 字节时返回 ErrKeySize。
func NewCipher(key []byte) (*Cipher, error) {
	return nil, errNotImplemented
}

// Encrypt 加密 src 的第一个分组写入 dst（可原地）。
func (c *Cipher) Encrypt(dst, src []byte) {
	panic("aes: 尚未实现")
}

// Decrypt 解密 src 的第一个分组写入 dst（可原地）。
func (c *Cipher) Decrypt(dst, src []byte) {
	panic("aes: 尚未实现")
}

// CBCWriter 以 CBC 模式加密写入的明文，Close 时追加 PKCS#7 填充。
type CBCWriter struct{}

// NewCBCWriter 返回把密文写入 w 的 CBCWriter。
func NewCBCWriter(w io.Writer, key, iv []byte) (*CBCWriter, error) {
	return nil, errNotImplemented
}

// Write 加密并写出所有完整分组，不足一组的部分留待后续。
func (w *CBCWriter) Write(p []byte) (int, error) {
	return 0, errNotImplemented
}

// Close 写出带填充的最后一个分组。不关闭底层 Writer。
func (w *CBCWriter) Close() error {
	return errNotImplemented
}

// CBCReader 从密文流读取并解密，在流末尾校验并去除 PKCS#7 填充。
type CBCReader struct{}

// NewCBCReader 返回从 r 读取密文的 CBCReader。
func NewCBCReader(r io.Reader, key, iv []byte) (*CBCReader, error) {
	return nil, errNotImplemented
}

// Read 读取明文；流结束返回 io.EOF，填充或长度非法返回 ErrPadding。
func (r *CBCReader) Read(p []byte) (int, error) {
	return 0, errNotImplemented
}
