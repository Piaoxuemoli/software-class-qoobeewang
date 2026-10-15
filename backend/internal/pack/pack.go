// Package pack 实现自研归档格式 MyPack（FR-11）：仿 tar，每个对象是 512 字节的头记录
// 加上补齐到 512 字节倍数的数据，长文件名使用多个头块，归档以两个全零块结尾。
// 字节级格式见 docs/architecture/mypack-format.md（M2-3 编写）。
//
// 用法与 archive/tar 相同：Writer 先 WriteHeader 再写数据；Reader 先 Next 再读数据。
package pack

import (
	"errors"
	"io"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
)

var errNotImplemented = errors.New("pack: 尚未实现")

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

// Writer 把条目顺序写成 MyPack 归档。
type Writer struct{}

// NewWriter 返回写入 w 的 Writer。
func NewWriter(w io.Writer) *Writer {
	return &Writer{}
}

// WriteHeader 写入一个条目的头记录（必要时多个头块）。普通文件（非硬链接）之后必须
// 恰好写入 fi.Size 字节数据；其他类型没有数据。上一个条目数据不足时返回错误。
// fi.Path 不满足 FileInfo.Path 格式时返回 ErrUnsafePath。
func (w *Writer) WriteHeader(fi fsmeta.FileInfo) error {
	return errNotImplemented
}

// Write 写入当前条目的数据。
func (w *Writer) Write(p []byte) (int, error) {
	return 0, errNotImplemented
}

// Close 补齐最后一个条目、写入结尾的两个零块。不关闭底层 Writer。
func (w *Writer) Close() error {
	return errNotImplemented
}

// Reader 顺序读取 MyPack 归档。
type Reader struct{}

// NewReader 返回从 r 读取的 Reader。
func NewReader(r io.Reader) *Reader {
	return &Reader{}
}

// Next 跳过当前条目剩余数据，读取下一个条目的头记录。读到结尾零块时返回 io.EOF。
// 头记录校验失败返回 ErrCorrupt，路径不安全返回 ErrUnsafePath（均可用 errors.Is 判断）。
// 返回的 FileInfo 中 Dev、Ino、Nlink 为 0。
func (r *Reader) Next() (fsmeta.FileInfo, error) {
	return fsmeta.FileInfo{}, errNotImplemented
}

// Read 读取当前条目的数据，读完返回 io.EOF。
func (r *Reader) Read(p []byte) (int, error) {
	return 0, errNotImplemented
}
