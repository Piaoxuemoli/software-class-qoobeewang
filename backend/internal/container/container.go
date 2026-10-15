// Package container 读写备份文件 .dbk（FR-13、FR-17）。
//
// 布局：头部（魔数、格式版本、压缩/加密标识、创建时间、源目录、KDF 参数、盐、IV）
// → 负载（engine 产生的打包流，可能已压缩；加密时为 AES-256-CBC 密文）
// → 尾部（Summary 统计）→ 校验值。
// 加密时：PBKDF2-HMAC-SHA256(口令, 盐, 迭代次数) 派生 64 字节，前 32 字节为 AES 密钥、
// 后 32 字节为 HMAC 密钥；校验值 = HMAC-SHA256(头部‖密文‖尾部)（encrypt-then-MAC）。
// 未加密时：校验值 = SHA-256(头部‖负载‖尾部)。
// 头部与尾部始终是明文，list/info 无需口令即可读取。字节级格式见
// docs/architecture/container-format.md（M5-4 编写）。
package container

import (
	"errors"
	"io"
	"time"
)

var errNotImplemented = errors.New("container: 尚未实现")

// Ext 是备份文件扩展名；PartialExt 是写入中的临时文件追加的后缀（<名称>.dbk.partial）。
const (
	Ext        = ".dbk"
	PartialExt = ".partial"
)

// FormatVersion 是当前写入的容器格式版本。
const FormatVersion = 1

// DefaultIterations 是 PBKDF2 的默认迭代次数。
const DefaultIterations = 100_000

// Compression 是负载的压缩算法标识。
type Compression uint8

// 压缩算法。
const (
	CompressionNone Compression = iota
	CompressionHuffman
)

// Encryption 是负载的加密算法标识。
type Encryption uint8

// 加密算法。
const (
	EncryptionNone Encryption = iota
	EncryptionAES256CBC
)

var (
	// ErrAuth 表示 HMAC 校验失败：口令错误或数据被篡改。cli 映射为 E_AUTH。
	ErrAuth = errors.New("口令错误或备份已被篡改")
	// ErrPasswordRequired 表示备份已加密但未提供口令。cli 映射为 E_PASSWORD_REQUIRED。
	ErrPasswordRequired = errors.New("备份已加密，需要口令")
	// ErrCorrupt 表示不是合法的 .dbk 文件、被截断，或未加密备份的 SHA-256 不符。cli 映射为 E_CORRUPT。
	ErrCorrupt = errors.New("备份文件已损坏")
	// ErrUnsupportedVersion 表示格式版本高于本程序支持的版本。cli 映射为 E_CORRUPT。
	ErrUnsupportedVersion = errors.New("不支持的备份格式版本")
)

// Header 是容器头部。
type Header struct {
	// Version 是格式版本：读取时填写；写入时忽略，总是写 FormatVersion。
	Version uint16
	// Compression、Encryption 是负载使用的算法。
	Compression Compression
	Encryption  Encryption
	// CreatedAt 是备份创建时间（秒精度保存）。
	CreatedAt time.Time
	// Source 是备份时源目录的绝对路径。
	Source string
	// Iterations、Salt、IV 只在加密时有意义：写入时由 NewWriter 生成（Iterations 为 0 时
	// 使用 DefaultIterations），读取时填写。
	Iterations uint32
	Salt       []byte
	IV         []byte
}

// Summary 是写在尾部的统计信息，口径同契约 backup 结果。
type Summary struct {
	Files, Dirs, Bytes int64
}

// Info 是不校验、不解密即可读到的信息，用于 list/info。
type Info struct {
	Header  Header
	Summary Summary
	// Size 是整个备份文件的字节数。
	Size int64
}

// Writer 把负载写成容器。
type Writer struct{}

// NewWriter 写出头部并返回负载写入器。h.Encryption 不为 None 时 password 必须非空，
// 由本函数用 crypto/rand 生成盐与 IV 并派生密钥；为 None 时 password 必须为 nil。
func NewWriter(w io.Writer, h Header, password []byte) (*Writer, error) {
	return nil, errNotImplemented
}

// Write 写入负载（加密时边写边加密、边计算 HMAC）。
func (w *Writer) Write(p []byte) (int, error) {
	return 0, errNotImplemented
}

// Finish 结束负载，写出尾部 s 与校验值。不关闭底层 Writer。
func (w *Writer) Finish(s Summary) error {
	return errNotImplemented
}

// ReadInfo 只读取头部与尾部（不校验、不需要口令）。格式错误返回 ErrCorrupt 或
// ErrUnsupportedVersion。
func ReadInfo(r io.ReadSeeker) (Info, error) {
	return Info{}, errNotImplemented
}

// Reader 读取已校验的负载明文。
type Reader struct{}

// Open 先完整读取一遍 r 校验校验值（加密：HMAC，失败返回 ErrAuth；未加密：SHA-256，
// 失败返回 ErrCorrupt），通过后回到负载起点，返回负载明文读取器。
// 加密备份 password 为空时返回 ErrPasswordRequired；未加密备份忽略 password。
func Open(r io.ReadSeeker, password []byte) (*Reader, error) {
	return nil, errNotImplemented
}

// Info 返回容器的头部、尾部与文件大小。
func (r *Reader) Info() Info {
	return Info{}
}

// Read 读取负载明文（仍是打包流，可能已压缩），读完返回 io.EOF。
func (r *Reader) Read(p []byte) (int, error) {
	return 0, errNotImplemented
}
