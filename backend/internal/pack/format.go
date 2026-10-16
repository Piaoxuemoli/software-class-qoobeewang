package pack

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
)

// 头块布局，见 docs/architecture/mypack-format.md §2。
const (
	// Magic 是每个头块开头的魔数。
	Magic = "MYPK"
	// Version 是当前格式版本。
	Version = 1
	// MaxNameLen 是路径与链接目标各自的最大字节数。
	MaxNameLen = 1 << 16

	offVersion   = 4
	offType      = 5
	offFlags     = 6
	offReserved1 = 7
	offChecksum  = 8
	offExtBlocks = 12
	offPathLen   = 16
	offLinkLen   = 20
	offMode      = 24
	offUID       = 28
	offGID       = 32
	offDevMajor  = 36
	offDevMinor  = 40
	offReserved2 = 44
	offSize      = 48
	offMtimeSec  = 56
	offAtimeSec  = 64
	offCtimeSec  = 72
	offBtimeSec  = 80
	offMtimeNsec = 88
	offAtimeNsec = 92
	offCtimeNsec = 96
	offBtimeNsec = 100
	offName      = 104

	// inlineNameLen 是头块内联名字区的长度。
	inlineNameLen = BlockSize - offName

	flagBtime = 1 << 0
)

// 类型码（ASCII），沿用 tar；'7' 套接字为 MyPack 扩展。
const (
	typeRegular  = '0'
	typeHardlink = '1'
	typeSymlink  = '2'
	typeChar     = '3'
	typeBlock    = '4'
	typeDir      = '5'
	typeFIFO     = '6'
	typeSocket   = '7'
)

var typeflagOf = map[fsmeta.FileType]byte{
	fsmeta.TypeRegular:     typeRegular,
	fsmeta.TypeSymlink:     typeSymlink,
	fsmeta.TypeCharDevice:  typeChar,
	fsmeta.TypeBlockDevice: typeBlock,
	fsmeta.TypeDir:         typeDir,
	fsmeta.TypeFIFO:        typeFIFO,
	fsmeta.TypeSocket:      typeSocket,
}

var fileTypeOf = map[byte]fsmeta.FileType{
	typeRegular:  fsmeta.TypeRegular,
	typeHardlink: fsmeta.TypeRegular,
	typeSymlink:  fsmeta.TypeSymlink,
	typeChar:     fsmeta.TypeCharDevice,
	typeBlock:    fsmeta.TypeBlockDevice,
	typeDir:      fsmeta.TypeDir,
	typeFIFO:     fsmeta.TypeFIFO,
	typeSocket:   fsmeta.TypeSocket,
}

// extBlocksFor 返回名字总长 n 所需的扩展块个数。
func extBlocksFor(n int) int {
	if n <= inlineNameLen {
		return 0
	}
	return (n - inlineNameLen + BlockSize - 1) / BlockSize
}

// checkPath 校验 FileInfo.Path 格式（同时用于硬链接目标），见格式文档 §4。
func checkPath(p string) error {
	if p == "" {
		return fmt.Errorf("路径为空: %w", ErrUnsafePath)
	}
	if strings.IndexByte(p, 0) >= 0 {
		return fmt.Errorf("路径 %q 含 NUL: %w", p, ErrUnsafePath)
	}
	if p[0] == '/' {
		return fmt.Errorf("路径 %q 是绝对路径: %w", p, ErrUnsafePath)
	}
	for seg := range strings.SplitSeq(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("路径 %q 含非法段 %q: %w", p, seg, ErrUnsafePath)
		}
	}
	return nil
}

// encodeHeader 把 fi 编码为头块加扩展块（含校验和）。调用方已校验 fi。
func encodeHeader(fi fsmeta.FileInfo) []byte {
	typeflag := typeflagOf[fi.Type]
	link := fi.LinkTarget
	if fi.IsHardlink() {
		typeflag, link = typeHardlink, fi.HardlinkTo
	}
	nameLen := len(fi.Path) + len(link)
	ext := extBlocksFor(nameLen)
	buf := make([]byte, BlockSize*(1+ext))
	h := buf[:BlockSize]

	le := binary.LittleEndian
	copy(h, Magic)
	h[offVersion] = Version
	h[offType] = typeflag
	if !fi.Btime.IsZero() {
		h[offFlags] = flagBtime
	}
	le.PutUint32(h[offExtBlocks:], uint32(ext))
	le.PutUint32(h[offPathLen:], uint32(len(fi.Path)))
	le.PutUint32(h[offLinkLen:], uint32(len(link)))
	le.PutUint32(h[offMode:], fi.Mode)
	le.PutUint32(h[offUID:], fi.UID)
	le.PutUint32(h[offGID:], fi.GID)
	le.PutUint32(h[offDevMajor:], fi.DevMajor)
	le.PutUint32(h[offDevMinor:], fi.DevMinor)
	if !fi.IsHardlink() {
		le.PutUint64(h[offSize:], uint64(fi.Size))
	}
	putTime(h, offMtimeSec, offMtimeNsec, fi.Mtime)
	putTime(h, offAtimeSec, offAtimeNsec, fi.Atime)
	putTime(h, offCtimeSec, offCtimeNsec, fi.Ctime)
	if !fi.Btime.IsZero() {
		putTime(h, offBtimeSec, offBtimeNsec, fi.Btime)
	}
	// 名字区从头块偏移 offName 开始，连续延伸到扩展块。
	n := copy(buf[offName:], fi.Path)
	copy(buf[offName+n:], link)

	le.PutUint32(h[offChecksum:], crc32(buf))
	return buf
}

func putTime(h []byte, offSec, offNsec int, t time.Time) {
	binary.LittleEndian.PutUint64(h[offSec:], uint64(t.Unix()))
	binary.LittleEndian.PutUint32(h[offNsec:], uint32(t.Nanosecond()))
}

// rawHeader 是头块中解析出的、读取扩展块之前就需要的字段。
type rawHeader struct {
	ext              int
	pathLen, linkLen int
}

// parseFixed 校验头块固定字段（魔数、版本、保留字节、长度），返回扩展块信息。
func parseFixed(h []byte) (rawHeader, error) {
	le := binary.LittleEndian
	if string(h[:len(Magic)]) != Magic {
		return rawHeader{}, fmt.Errorf("魔数不符 %q: %w", h[:len(Magic)], ErrCorrupt)
	}
	if h[offVersion] != Version {
		return rawHeader{}, fmt.Errorf("不支持的格式版本 %d: %w", h[offVersion], ErrCorrupt)
	}
	if h[offReserved1] != 0 || le.Uint32(h[offReserved2:]) != 0 || h[offFlags]&^flagBtime != 0 {
		return rawHeader{}, fmt.Errorf("保留字段非 0: %w", ErrCorrupt)
	}
	pathLen, linkLen := le.Uint32(h[offPathLen:]), le.Uint32(h[offLinkLen:])
	if pathLen > MaxNameLen || linkLen > MaxNameLen {
		return rawHeader{}, fmt.Errorf("名字长度 %d/%d 超过上限: %w", pathLen, linkLen, ErrCorrupt)
	}
	rh := rawHeader{pathLen: int(pathLen), linkLen: int(linkLen)}
	rh.ext = extBlocksFor(rh.pathLen + rh.linkLen)
	if le.Uint32(h[offExtBlocks:]) != uint32(rh.ext) {
		return rawHeader{}, fmt.Errorf("扩展块数 %d 与名字长度不符: %w", le.Uint32(h[offExtBlocks:]), ErrCorrupt)
	}
	return rh, nil
}

// decodeHeader 校验并解码完整的头块加扩展块 buf。路径规则由调用方另行检查。
func decodeHeader(buf []byte, rh rawHeader) (fsmeta.FileInfo, error) {
	le := binary.LittleEndian
	h := buf[:BlockSize]
	want := le.Uint32(h[offChecksum:])
	copy(h[offChecksum:offChecksum+4], []byte{0, 0, 0, 0})
	got := crc32(buf)
	le.PutUint32(h[offChecksum:], want)
	if got != want {
		return fsmeta.FileInfo{}, fmt.Errorf("头块校验和不符（记录 %08x，实际 %08x）: %w", want, got, ErrCorrupt)
	}

	names := buf[offName:]
	for _, b := range names[rh.pathLen+rh.linkLen:] {
		if b != 0 {
			return fsmeta.FileInfo{}, fmt.Errorf("名字区填充非 0: %w", ErrCorrupt)
		}
	}
	typeflag := h[offType]
	ft, ok := fileTypeOf[typeflag]
	if !ok {
		return fsmeta.FileInfo{}, fmt.Errorf("未知类型码 %q: %w", typeflag, ErrCorrupt)
	}
	fi := fsmeta.FileInfo{
		Path:     string(names[:rh.pathLen]),
		Type:     ft,
		Mode:     le.Uint32(h[offMode:]),
		UID:      le.Uint32(h[offUID:]),
		GID:      le.Uint32(h[offGID:]),
		Size:     int64(le.Uint64(h[offSize:])),
		DevMajor: le.Uint32(h[offDevMajor:]),
		DevMinor: le.Uint32(h[offDevMinor:]),
	}
	link := string(names[rh.pathLen : rh.pathLen+rh.linkLen])
	switch typeflag {
	case typeHardlink:
		fi.HardlinkTo = link
	case typeSymlink:
		if link == "" {
			return fsmeta.FileInfo{}, fmt.Errorf("软链接 %q 目标为空: %w", fi.Path, ErrCorrupt)
		}
		fi.LinkTarget = link
	default:
		if link != "" {
			return fsmeta.FileInfo{}, fmt.Errorf("非链接条目 %q 带链接目标: %w", fi.Path, ErrCorrupt)
		}
	}
	if fi.Mode&^0o7777 != 0 {
		return fsmeta.FileInfo{}, fmt.Errorf("权限位 %o 非法: %w", fi.Mode, ErrCorrupt)
	}
	if fi.Size < 0 || (fi.Size != 0 && typeflag != typeRegular) {
		return fsmeta.FileInfo{}, fmt.Errorf("条目 %q 的长度 %d 非法: %w", fi.Path, fi.Size, ErrCorrupt)
	}
	var err error
	if fi.Mtime, err = getTime(h, offMtimeSec, offMtimeNsec); err != nil {
		return fsmeta.FileInfo{}, err
	}
	if fi.Atime, err = getTime(h, offAtimeSec, offAtimeNsec); err != nil {
		return fsmeta.FileInfo{}, err
	}
	if fi.Ctime, err = getTime(h, offCtimeSec, offCtimeNsec); err != nil {
		return fsmeta.FileInfo{}, err
	}
	if h[offFlags]&flagBtime != 0 {
		if fi.Btime, err = getTime(h, offBtimeSec, offBtimeNsec); err != nil {
			return fsmeta.FileInfo{}, err
		}
	} else if le.Uint64(h[offBtimeSec:]) != 0 || le.Uint32(h[offBtimeNsec:]) != 0 {
		return fsmeta.FileInfo{}, fmt.Errorf("未标记 btime 却有非零 btime: %w", ErrCorrupt)
	}
	return fi, nil
}

func getTime(h []byte, offSec, offNsec int) (time.Time, error) {
	sec := int64(binary.LittleEndian.Uint64(h[offSec:]))
	nsec := binary.LittleEndian.Uint32(h[offNsec:])
	if nsec >= 1e9 {
		return time.Time{}, fmt.Errorf("纳秒字段 %d 超出范围: %w", nsec, ErrCorrupt)
	}
	return time.Unix(sec, int64(nsec)).UTC(), nil
}

// crcTable 是 CRC-32/IEEE（反射多项式 0xEDB88320）的查找表。
var crcTable = func() (t [256]uint32) {
	for i := range t {
		c := uint32(i)
		for range 8 {
			if c&1 != 0 {
				c = 0xEDB88320 ^ c>>1
			} else {
				c >>= 1
			}
		}
		t[i] = c
	}
	return t
}()

// crc32 计算 CRC-32/IEEE（与 zlib、PNG、以太网相同）。
func crc32(b []byte) uint32 {
	c := ^uint32(0)
	for _, x := range b {
		c = crcTable[byte(c)^x] ^ c>>8
	}
	return ^c
}
