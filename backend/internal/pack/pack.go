// Package pack 实现自研归档格式 MyPack（FR-11）：仿 tar，每个对象是 512 字节的头记录
// 加上补齐到 512 字节倍数的数据，长文件名使用多个头块，归档以两个全零块结尾。
// 字节级格式见 docs/architecture/mypack-format.md。
//
// 用法与 archive/tar 相同：Writer 先 WriteHeader 再写数据；Reader 先 Next 再读数据。
package pack

import (
	"errors"
	"fmt"
	"io"
	"path"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
)

// BlockSize 是头记录与数据块的对齐单位（字节），来源：tar/UStar 规范。
const BlockSize = 512

var (
	// ErrCorrupt 表示归档格式错误：魔数/校验和不符、字段非法、意外截断等。cli 映射为 E_CORRUPT。
	ErrCorrupt = errors.New("归档已损坏")
	// ErrUnsafePath 表示条目路径是绝对路径、含 ".." 段或为空。cli 映射为 E_CORRUPT。
	ErrUnsafePath = errors.New("归档中含非法路径")
	// ErrWriteTooLong 表示写入的数据超过了 WriteHeader 声明的 Size。
	ErrWriteTooLong = errors.New("写入数据超过头记录声明的长度")
)

var (
	errClosed     = errors.New("pack: Writer 已关闭")
	errShortEntry = errors.New("pack: 上一个条目的数据少于头记录声明的长度")
)

var zeroBlock [BlockSize]byte

// padding 返回 size 字节数据补齐到 BlockSize 倍数所需的填充长度。
func padding(size int64) int64 {
	return -size & (BlockSize - 1)
}

// Writer 把条目顺序写成 MyPack 归档。
type Writer struct {
	w         io.Writer
	remaining int64 // 当前条目尚未写入的数据字节数
	pad       int64 // 当前条目数据之后需要的填充字节数
	closed    bool
	err       error // 底层写失败后保持
}

// NewWriter 返回写入 w 的 Writer。
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// WriteHeader 写入一个条目的头记录（必要时多个头块）。普通文件（非硬链接）之后必须
// 恰好写入 fi.Size 字节数据；其他类型没有数据。上一个条目数据不足时返回错误。
// fi.Path 不满足 FileInfo.Path 格式时返回 ErrUnsafePath。
func (w *Writer) WriteHeader(fi fsmeta.FileInfo) error {
	if err := w.finishEntry(); err != nil {
		return err
	}
	if err := validate(fi); err != nil {
		return err
	}
	if err := w.write(encodeHeader(fi)); err != nil {
		return err
	}
	if fi.Type == fsmeta.TypeRegular && !fi.IsHardlink() {
		w.remaining, w.pad = fi.Size, padding(fi.Size)
	}
	return nil
}

// validate 检查 fi 能否按格式写出。
func validate(fi fsmeta.FileInfo) error {
	if err := checkPath(fi.Path); err != nil {
		return err
	}
	if _, ok := typeflagOf[fi.Type]; !ok {
		return fmt.Errorf("pack: 条目 %q 的类型 %d 非法", fi.Path, fi.Type)
	}
	if fi.IsHardlink() {
		if fi.Type != fsmeta.TypeRegular {
			return fmt.Errorf("pack: 硬链接条目 %q 的类型必须是普通文件", fi.Path)
		}
		if err := checkPath(fi.HardlinkTo); err != nil {
			return fmt.Errorf("硬链接 %q 的目标: %w", fi.Path, err)
		}
	}
	if fi.Mode&^0o7777 != 0 {
		return fmt.Errorf("pack: 条目 %q 的权限位 %o 超出 0o7777", fi.Path, fi.Mode)
	}
	if fi.Size < 0 || (fi.Size != 0 && (fi.Type != fsmeta.TypeRegular || fi.IsHardlink())) {
		return fmt.Errorf("pack: 条目 %q 的长度 %d 非法（只有普通文件可以有数据）", fi.Path, fi.Size)
	}
	switch {
	case fi.Type == fsmeta.TypeSymlink && fi.LinkTarget == "":
		return fmt.Errorf("pack: 软链接 %q 的目标为空", fi.Path)
	case fi.Type != fsmeta.TypeSymlink && fi.LinkTarget != "":
		return fmt.Errorf("pack: 非软链接条目 %q 带链接目标", fi.Path)
	}
	if len(fi.Path) > MaxNameLen || len(fi.LinkTarget) > MaxNameLen || len(fi.HardlinkTo) > MaxNameLen {
		return fmt.Errorf("pack: 条目 %q 的路径或链接目标超过 %d 字节", fi.Path, MaxNameLen)
	}
	return nil
}

// Write 写入当前条目的数据。超过声明长度时只写到声明长度，并返回 ErrWriteTooLong。
func (w *Writer) Write(p []byte) (int, error) {
	if w.closed {
		return 0, errClosed
	}
	if w.err != nil {
		return 0, w.err
	}
	tooLong := int64(len(p)) > w.remaining
	if tooLong {
		p = p[:w.remaining]
	}
	n, err := w.w.Write(p)
	w.remaining -= int64(n)
	if err != nil {
		w.err = err
		return n, err
	}
	if tooLong {
		return n, ErrWriteTooLong
	}
	return n, nil
}

// Close 补齐最后一个条目、写入结尾的两个零块。不关闭底层 Writer。重复调用返回 nil。
func (w *Writer) Close() error {
	if w.closed {
		return nil
	}
	if err := w.finishEntry(); err != nil {
		return err
	}
	w.closed = true
	if err := w.write(zeroBlock[:]); err != nil {
		return err
	}
	return w.write(zeroBlock[:])
}

// finishEntry 检查当前条目数据已写满并写入填充。
func (w *Writer) finishEntry() error {
	if w.closed {
		return errClosed
	}
	if w.err != nil {
		return w.err
	}
	if w.remaining > 0 {
		return errShortEntry
	}
	if w.pad > 0 {
		pad := w.pad
		w.pad = 0
		return w.write(zeroBlock[:pad])
	}
	return nil
}

func (w *Writer) write(b []byte) error {
	if w.err != nil {
		return w.err
	}
	if _, err := w.w.Write(b); err != nil {
		w.err = err
		return err
	}
	return nil
}

// Reader 顺序读取 MyPack 归档。
type Reader struct {
	r         io.Reader
	remaining int64 // 当前条目尚未读出的数据字节数
	pad       int64 // 当前条目数据之后的填充字节数
	symlinks  map[string]bool
	err       error // 粘滞错误；读到结尾后为 io.EOF
	block     [BlockSize]byte
}

// NewReader 返回从 r 读取的 Reader。
func NewReader(r io.Reader) *Reader {
	return &Reader{r: r, symlinks: map[string]bool{}}
}

// corrupt 把底层读错误转换为 ErrCorrupt（截断）或原样保留（I/O 错误）。
func corrupt(what string, err error) error {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf("%s时意外截断: %w", what, ErrCorrupt)
	}
	return err
}

// Next 跳过当前条目剩余数据，读取下一个条目的头记录。读到结尾零块时返回 io.EOF。
// 头记录校验失败返回 ErrCorrupt，路径不安全返回 ErrUnsafePath（均可用 errors.Is 判断）。
// 返回的 FileInfo 中 Dev、Ino、Nlink 为 0。
func (r *Reader) Next() (fsmeta.FileInfo, error) {
	if r.err != nil {
		return fsmeta.FileInfo{}, r.err
	}
	fi, err := r.next()
	if err != nil {
		r.err = err
		return fsmeta.FileInfo{}, err
	}
	return fi, nil
}

func (r *Reader) next() (fsmeta.FileInfo, error) {
	if skip := r.remaining + r.pad; skip > 0 {
		if _, err := io.CopyN(io.Discard, r.r, skip); err != nil {
			return fsmeta.FileInfo{}, corrupt("跳过数据", err)
		}
	}
	r.remaining, r.pad = 0, 0

	h := r.block[:]
	if _, err := io.ReadFull(r.r, h); err != nil {
		if err == io.EOF {
			return fsmeta.FileInfo{}, fmt.Errorf("缺少结尾零块: %w", ErrCorrupt)
		}
		return fsmeta.FileInfo{}, corrupt("读取头块", err)
	}
	if *(*[BlockSize]byte)(h) == zeroBlock {
		if _, err := io.ReadFull(r.r, h); err != nil {
			return fsmeta.FileInfo{}, corrupt("读取结尾零块", err)
		}
		if *(*[BlockSize]byte)(h) != zeroBlock {
			return fsmeta.FileInfo{}, fmt.Errorf("零块之后出现非零块: %w", ErrCorrupt)
		}
		return fsmeta.FileInfo{}, io.EOF
	}

	rh, err := parseFixed(h)
	if err != nil {
		return fsmeta.FileInfo{}, err
	}
	buf := make([]byte, BlockSize*(1+rh.ext))
	copy(buf, h)
	if _, err := io.ReadFull(r.r, buf[BlockSize:]); err != nil {
		return fsmeta.FileInfo{}, corrupt("读取扩展块", err)
	}
	fi, err := decodeHeader(buf, rh)
	if err != nil {
		return fsmeta.FileInfo{}, err
	}
	if err := r.checkSafe(fi.Path); err != nil {
		return fsmeta.FileInfo{}, err
	}
	if fi.IsHardlink() {
		if err := r.checkSafe(fi.HardlinkTo); err != nil {
			return fsmeta.FileInfo{}, fmt.Errorf("硬链接 %q 的目标: %w", fi.Path, err)
		}
	}
	if fi.Type == fsmeta.TypeSymlink {
		r.symlinks[fi.Path] = true
	}
	r.remaining, r.pad = fi.Size, padding(fi.Size)
	return fi, nil
}

// checkSafe 校验路径格式，并拒绝以已出现的软链接为祖先的路径（格式文档 §4）。
func (r *Reader) checkSafe(p string) error {
	if err := checkPath(p); err != nil {
		return err
	}
	if len(r.symlinks) == 0 {
		return nil
	}
	for dir := path.Dir(p); dir != "."; dir = path.Dir(dir) {
		if r.symlinks[dir] {
			return fmt.Errorf("路径 %q 位于软链接 %q 之下: %w", p, dir, ErrUnsafePath)
		}
	}
	return nil
}

// Read 读取当前条目的数据，读完返回 io.EOF。
func (r *Reader) Read(p []byte) (int, error) {
	if r.err != nil && r.err != io.EOF {
		return 0, r.err
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.r.Read(p)
	r.remaining -= int64(n)
	switch {
	case err == io.EOF && r.remaining > 0:
		r.err = fmt.Errorf("读取数据时意外截断: %w", ErrCorrupt)
		return n, r.err
	case err == io.EOF:
		return n, nil
	case err != nil:
		r.err = err
		return n, err
	}
	return n, nil
}
