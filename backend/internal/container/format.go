package container

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/crypto/aes"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/crypto/sha256"
)

// 字节布局常量，定义见 docs/architecture/container-format.md。
const (
	magic        = "DBAK" // §2 头部魔数
	trailerMagic = "DBKE" // §4 尾部魔数

	fixedLen   = 32          // §2 头部固定部分
	trailerLen = 40          // §4
	sumLen     = sha256.Size // §5 HMAC-SHA256 与 SHA-256 都是 32 字节

	// SaltLen 是 PBKDF2 盐的字节数。
	SaltLen = 16
	// IVLen 是 CBC 初始向量的字节数（AES 分组长度）。
	IVLen = aes.BlockSize
	// MaxSourceLen 是源目录路径的最大字节数。
	MaxSourceLen = 1 << 16
	// MaxIterations 是读写时接受的最大 PBKDF2 迭代次数，防止伪造文件拖住还原。
	MaxIterations = 1_000_000

	keyLen = 64 // PBKDF2 输出：前 32 字节 AES-256 密钥，后 32 字节 HMAC 密钥
)

// 头部固定部分各字段偏移（§2）。
const (
	offVersion     = 4
	offCompression = 6
	offEncryption  = 7
	offCreatedAt   = 8
	offIterations  = 16
	offSaltLen     = 20
	offIVLen       = 21
	offReserved1   = 22
	offSourceLen   = 24
	offReserved2   = 28
)

// 尾部各字段偏移（§4）。
const (
	offTrailerReserved = 4
	offPayloadLen      = 8
	offFiles           = 16
	offDirs            = 24
	offBytes           = 32
)

var le = binary.LittleEndian

func headerLen(h *Header) int64 {
	return fixedLen + int64(len(h.Salt)+len(h.IV)+len(h.Source))
}

// encodeHeader 按 §2 编码头部；调用方已保证 h 合法且 Salt/IV 与加密标识一致。
func encodeHeader(h *Header) []byte {
	b := make([]byte, headerLen(h))
	copy(b, magic)
	le.PutUint16(b[offVersion:], FormatVersion)
	b[offCompression] = byte(h.Compression)
	b[offEncryption] = byte(h.Encryption)
	le.PutUint64(b[offCreatedAt:], uint64(h.CreatedAt.Unix()))
	le.PutUint32(b[offIterations:], h.Iterations)
	b[offSaltLen] = byte(len(h.Salt))
	b[offIVLen] = byte(len(h.IV))
	le.PutUint32(b[offSourceLen:], uint32(len(h.Source)))
	n := copy(b[fixedLen:], h.Salt)
	n += copy(b[fixedLen+n:], h.IV)
	copy(b[fixedLen+n:], h.Source)
	return b
}

// parseFixed 解析头部固定部分，返回各变长字段长度之和。
func parseFixed(b []byte, h *Header) (varLen int, err error) {
	if string(b[:len(magic)]) != magic {
		return 0, fmt.Errorf("%w: 魔数不符", ErrCorrupt)
	}
	h.Version = le.Uint16(b[offVersion:])
	switch {
	case h.Version == 0:
		return 0, fmt.Errorf("%w: 版本为 0", ErrCorrupt)
	case h.Version > FormatVersion:
		return 0, fmt.Errorf("%w: %d", ErrUnsupportedVersion, h.Version)
	}
	h.Compression = Compression(b[offCompression])
	h.Encryption = Encryption(b[offEncryption])
	h.CreatedAt = time.Unix(int64(le.Uint64(b[offCreatedAt:])), 0).UTC()
	h.Iterations = le.Uint32(b[offIterations:])
	saltLen, ivLen := int(b[offSaltLen]), int(b[offIVLen])
	sourceLen := le.Uint32(b[offSourceLen:])
	if le.Uint16(b[offReserved1:]) != 0 || le.Uint32(b[offReserved2:]) != 0 {
		return 0, fmt.Errorf("%w: 头部保留字段非 0", ErrCorrupt)
	}
	if h.Compression > CompressionHuffman {
		return 0, fmt.Errorf("%w: 未知压缩算法 %d", ErrCorrupt, h.Compression)
	}
	switch h.Encryption {
	case EncryptionNone:
		if h.Iterations != 0 || saltLen != 0 || ivLen != 0 {
			return 0, fmt.Errorf("%w: 未加密备份带有 KDF 参数", ErrCorrupt)
		}
	case EncryptionAES256CBC:
		if h.Iterations == 0 || h.Iterations > MaxIterations || saltLen != SaltLen || ivLen != IVLen {
			return 0, fmt.Errorf("%w: KDF 参数非法", ErrCorrupt)
		}
	default:
		return 0, fmt.Errorf("%w: 未知加密算法 %d", ErrCorrupt, h.Encryption)
	}
	if sourceLen > MaxSourceLen {
		return 0, fmt.Errorf("%w: 源目录过长", ErrCorrupt)
	}
	return saltLen + ivLen + int(sourceLen), nil
}

// parseVar 从变长部分取出盐、IV 与源目录，长度已由 parseFixed 校验。
func parseVar(fixed, v []byte, h *Header) {
	saltLen, ivLen := int(fixed[offSaltLen]), int(fixed[offIVLen])
	if saltLen > 0 {
		h.Salt = append([]byte(nil), v[:saltLen]...)
		h.IV = append([]byte(nil), v[saltLen:saltLen+ivLen]...)
	}
	h.Source = string(v[saltLen+ivLen:])
}

func encodeTrailer(payloadLen int64, s Summary) []byte {
	b := make([]byte, trailerLen)
	copy(b, trailerMagic)
	le.PutUint64(b[offPayloadLen:], uint64(payloadLen))
	le.PutUint64(b[offFiles:], uint64(s.Files))
	le.PutUint64(b[offDirs:], uint64(s.Dirs))
	le.PutUint64(b[offBytes:], uint64(s.Bytes))
	return b
}

func parseTrailer(b []byte) (payloadLen uint64, s Summary, err error) {
	if string(b[:len(trailerMagic)]) != trailerMagic || le.Uint32(b[offTrailerReserved:]) != 0 {
		return 0, s, fmt.Errorf("%w: 尾部不符", ErrCorrupt)
	}
	s = Summary{
		Files: int64(le.Uint64(b[offFiles:])),
		Dirs:  int64(le.Uint64(b[offDirs:])),
		Bytes: int64(le.Uint64(b[offBytes:])),
	}
	if s.Files < 0 || s.Dirs < 0 || s.Bytes < 0 {
		return 0, s, fmt.Errorf("%w: 统计为负", ErrCorrupt)
	}
	return le.Uint64(b[offPayloadLen:]), s, nil
}

// readInfo 实现 ReadInfo，另外返回头部长度供 Open 定位负载。
func readInfo(r io.ReadSeeker) (Info, int64, error) {
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return Info{}, 0, err
	}
	if size < fixedLen+trailerLen+sumLen {
		return Info{}, 0, fmt.Errorf("%w: 文件过短", ErrCorrupt)
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return Info{}, 0, err
	}
	info := Info{Size: size}
	fixed := make([]byte, fixedLen)
	if _, err := io.ReadFull(r, fixed); err != nil {
		return Info{}, 0, shortRead(err)
	}
	varLen, err := parseFixed(fixed, &info.Header)
	if err != nil {
		return Info{}, 0, err
	}
	hlen := int64(fixedLen + varLen)
	if hlen > size-trailerLen-sumLen {
		return Info{}, 0, fmt.Errorf("%w: 头部超出文件", ErrCorrupt)
	}
	v := make([]byte, varLen)
	if _, err := io.ReadFull(r, v); err != nil {
		return Info{}, 0, shortRead(err)
	}
	parseVar(fixed, v, &info.Header)

	if _, err := r.Seek(size-trailerLen-sumLen, io.SeekStart); err != nil {
		return Info{}, 0, err
	}
	tb := make([]byte, trailerLen)
	if _, err := io.ReadFull(r, tb); err != nil {
		return Info{}, 0, shortRead(err)
	}
	payloadLen, s, err := parseTrailer(tb)
	if err != nil {
		return Info{}, 0, err
	}
	if payloadLen != uint64(size-hlen-trailerLen-sumLen) {
		return Info{}, 0, fmt.Errorf("%w: 负载长度与文件长度不符", ErrCorrupt)
	}
	if info.Header.Encryption != EncryptionNone && (payloadLen == 0 || payloadLen%aes.BlockSize != 0) {
		return Info{}, 0, fmt.Errorf("%w: 密文长度不是分组的整数倍", ErrCorrupt)
	}
	info.Summary = s
	return info, hlen, nil
}

// shortRead 把读取中途遇到的文件结束（文件在 Seek 之后被截短）归为损坏。
func shortRead(err error) error {
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return fmt.Errorf("%w: 文件被截断", ErrCorrupt)
	}
	return err
}
