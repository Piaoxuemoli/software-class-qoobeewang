// Package container 读写备份文件 .dbk（FR-13、FR-17）。
//
// 布局：头部（魔数、格式版本、压缩/加密标识、创建时间、源目录、KDF 参数、盐、IV）
// → 负载（engine 产生的打包流，可能已压缩；加密时为 AES-256-CBC 密文）
// → 尾部（Summary 统计）→ 校验值。
// 加密时：PBKDF2-HMAC-SHA256(口令, 盐, 迭代次数) 派生 64 字节，前 32 字节为 AES 密钥、
// 后 32 字节为 HMAC 密钥；校验值 = HMAC-SHA256(头部‖密文‖尾部)（encrypt-then-MAC）。
// 未加密时：校验值 = SHA-256(头部‖负载‖尾部)。
// 头部与尾部始终是明文，list/info 无需口令即可读取。字节级格式见
// docs/architecture/container-format.md。
package container

import (
	"crypto/rand"
	"errors"
	"fmt"
	"hash"
	"io"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/crypto/aes"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/crypto/hmac"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/crypto/pbkdf2"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/crypto/sha256"
)

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

// errFinished 表示 Finish 之后又调用了 Write 或 Finish。
var errFinished = errors.New("container: 已调用 Finish")

// randRead 生成盐与 IV；测试替换它以得到可复现的 golden 文件。
var randRead = rand.Read

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

// deriveKeys 按 §3 从口令派生 AES 密钥与 HMAC 密钥。
func deriveKeys(password, salt []byte, iter uint32) (aesKey, macKey []byte) {
	k := pbkdf2.Key(password, salt, int(iter), keyLen, sha256.New)
	return k[:aes.KeySize], k[aes.KeySize:]
}

// Writer 把负载写成容器。
type Writer struct {
	raw     io.Writer      // 底层 Writer，只用来写最后的校验值
	out     *hashingWriter // 头部、负载、尾部都经过它，同时计入校验值
	payload io.Writer      // 负载入口：out，或加密时包在 out 外层的 CBC
	cbc     *aes.CBCWriter // 未加密时为 nil
	hlen    int64
	err     error
	done    bool
}

// NewWriter 写出头部并返回负载写入器。h.Encryption 不为 None 时 password 必须非空，
// 由本函数用 crypto/rand 生成盐与 IV 并派生密钥；为 None 时 password 必须为 nil。
func NewWriter(w io.Writer, h Header, password []byte) (*Writer, error) {
	if h.Compression > CompressionHuffman {
		return nil, fmt.Errorf("container: 未知压缩算法 %d", h.Compression)
	}
	if len(h.Source) > MaxSourceLen {
		return nil, fmt.Errorf("container: 源目录路径超过 %d 字节", MaxSourceLen)
	}
	h.Version = FormatVersion

	var sum hash.Hash
	var aesKey []byte
	switch h.Encryption {
	case EncryptionNone:
		if password != nil {
			return nil, errors.New("container: 未加密时不得提供口令")
		}
		h.Iterations, h.Salt, h.IV = 0, nil, nil
		sum = sha256.New()
	case EncryptionAES256CBC:
		if len(password) == 0 {
			return nil, ErrPasswordRequired
		}
		if h.Iterations == 0 {
			h.Iterations = DefaultIterations
		}
		if h.Iterations > MaxIterations {
			return nil, fmt.Errorf("container: 迭代次数超过 %d", MaxIterations)
		}
		h.Salt, h.IV = make([]byte, SaltLen), make([]byte, IVLen)
		if _, err := randRead(h.Salt); err != nil {
			return nil, err
		}
		if _, err := randRead(h.IV); err != nil {
			return nil, err
		}
		var macKey []byte
		aesKey, macKey = deriveKeys(password, h.Salt, h.Iterations)
		defer clear(aesKey)
		defer clear(macKey)
		sum = hmac.New(sha256.New, macKey)
	default:
		return nil, fmt.Errorf("container: 未知加密算法 %d", h.Encryption)
	}

	cw := &Writer{raw: w, out: &hashingWriter{w: w, sum: sum}}
	cw.payload = cw.out
	if aesKey != nil {
		cbc, err := aes.NewCBCWriter(cw.out, aesKey, h.IV)
		if err != nil {
			return nil, err
		}
		cw.cbc, cw.payload = cbc, cbc
	}
	hb := encodeHeader(&h)
	if _, err := cw.out.Write(hb); err != nil {
		return nil, err
	}
	cw.hlen = int64(len(hb))
	return cw, nil
}

// Write 写入负载（加密时边写边加密、边计算 HMAC）。
func (w *Writer) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.done {
		return 0, errFinished
	}
	n, err := w.payload.Write(p)
	if err != nil {
		w.err = err
	}
	return n, err
}

// Finish 结束负载，写出尾部 s 与校验值。不关闭底层 Writer。
func (w *Writer) Finish(s Summary) error {
	if w.err != nil {
		return w.err
	}
	if w.done {
		return errFinished
	}
	if s.Files < 0 || s.Dirs < 0 || s.Bytes < 0 {
		return errors.New("container: 统计不能为负")
	}
	w.done = true
	w.err = w.finish(s)
	return w.err
}

func (w *Writer) finish(s Summary) error {
	if w.cbc != nil {
		if err := w.cbc.Close(); err != nil {
			return err
		}
	}
	if _, err := w.out.Write(encodeTrailer(w.out.n-w.hlen, s)); err != nil {
		return err
	}
	return writeFull(w.raw, w.out.sum.Sum(nil))
}

// hashingWriter 把写入的字节同时送往 w 与 sum，并累计写出的字节数。
type hashingWriter struct {
	w   io.Writer
	sum hash.Hash
	n   int64
	err error
}

func (h *hashingWriter) Write(p []byte) (int, error) {
	if h.err != nil {
		return 0, h.err
	}
	n, err := h.w.Write(p)
	h.sum.Write(p[:n])
	h.n += int64(n)
	if err == nil && n < len(p) {
		err = io.ErrShortWrite
	}
	h.err = err
	return n, err
}

func writeFull(w io.Writer, p []byte) error {
	n, err := w.Write(p)
	if err == nil && n < len(p) {
		err = io.ErrShortWrite
	}
	return err
}

// ReadInfo 只读取头部与尾部（不校验、不需要口令）。格式错误返回 ErrCorrupt 或
// ErrUnsupportedVersion。
func ReadInfo(r io.ReadSeeker) (Info, error) {
	info, _, err := readInfo(r)
	return info, err
}

// Reader 读取已校验的负载明文。
type Reader struct {
	info Info
	src  io.Reader
	err  error
}

// Open 先完整读取一遍 r 校验校验值（加密：HMAC，失败返回 ErrAuth；未加密：SHA-256，
// 失败返回 ErrCorrupt），通过后回到负载起点，返回负载明文读取器。
// 加密备份 password 为空时返回 ErrPasswordRequired；未加密备份忽略 password。
func Open(r io.ReadSeeker, password []byte) (*Reader, error) {
	info, hlen, err := readInfo(r)
	if err != nil {
		return nil, err
	}
	encrypted := info.Header.Encryption != EncryptionNone
	if encrypted && len(password) == 0 {
		return nil, ErrPasswordRequired
	}

	var sum hash.Hash
	var aesKey []byte
	if encrypted {
		var macKey []byte
		aesKey, macKey = deriveKeys(password, info.Header.Salt, info.Header.Iterations)
		defer clear(aesKey)
		defer clear(macKey)
		sum = hmac.New(sha256.New, macKey)
	} else {
		sum = sha256.New()
	}
	if err := verify(r, info.Size, sum); err != nil {
		if errors.Is(err, errMismatch) {
			if encrypted {
				return nil, ErrAuth
			}
			return nil, fmt.Errorf("%w: SHA-256 不符", ErrCorrupt)
		}
		return nil, err
	}

	if _, err := r.Seek(hlen, io.SeekStart); err != nil {
		return nil, err
	}
	var src io.Reader = &exactReader{r: r, n: info.Size - hlen - trailerLen - sumLen}
	if encrypted {
		cbc, err := aes.NewCBCReader(src, aesKey, info.Header.IV)
		if err != nil {
			return nil, err
		}
		src = cbc
	}
	return &Reader{info: info, src: src}, nil
}

var errMismatch = errors.New("校验值不符")

// verify 从头读取 r，计算前 size-sumLen 字节的校验值并与末尾 sumLen 字节比较。
func verify(r io.ReadSeeker, size int64, sum hash.Hash) error {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := io.CopyN(sum, r, size-sumLen); err != nil {
		return shortRead(err)
	}
	stored := make([]byte, sumLen)
	if _, err := io.ReadFull(r, stored); err != nil {
		return shortRead(err)
	}
	if !hmac.Equal(sum.Sum(nil), stored) {
		return errMismatch
	}
	return nil
}

// Info 返回容器的头部、尾部与文件大小。
func (r *Reader) Info() Info {
	return r.info
}

// Read 读取负载明文（仍是打包流，可能已压缩），读完返回 io.EOF。
// 校验之后文件被改动导致负载变短或填充非法时返回 ErrCorrupt。
func (r *Reader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	n, err := r.src.Read(p)
	switch {
	case err == nil || err == io.EOF:
	case errors.Is(err, aes.ErrPadding), errors.Is(err, io.ErrUnexpectedEOF):
		err = fmt.Errorf("%w: 负载在校验后被改动", ErrCorrupt)
	}
	r.err = err
	return n, err
}

// exactReader 恰好读 n 字节；底层提前结束返回 io.ErrUnexpectedEOF。
type exactReader struct {
	r io.Reader
	n int64
}

func (e *exactReader) Read(p []byte) (int, error) {
	if e.n <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > e.n {
		p = p[:e.n]
	}
	k, err := e.r.Read(p)
	e.n -= int64(k)
	if err == io.EOF {
		if e.n > 0 {
			return k, io.ErrUnexpectedEOF
		}
		err = nil
	}
	return k, err
}
