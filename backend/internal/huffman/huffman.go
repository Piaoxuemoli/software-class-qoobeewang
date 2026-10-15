// Package huffman 实现分块流式 Huffman 压缩（FR-12）：输入按 BlockSize 分块，
// 每块统计频率、构造规范 Huffman 码表并编码，块头记录码长表。
// 编码输出与解码输入均为流，不整体读入内存。
package huffman

import (
	"errors"
	"io"
)

var errNotImplemented = errors.New("huffman: 尚未实现")

// BlockSize 是每个压缩块的最大原始数据长度（字节）。
const BlockSize = 1 << 20

// ErrCorrupt 表示压缩流格式错误（码表非法、截断等）。cli 映射为 E_CORRUPT。
var ErrCorrupt = errors.New("压缩数据已损坏")

// Writer 是压缩写入器。
type Writer struct{}

// NewWriter 返回把压缩结果写入 w 的 Writer。
func NewWriter(w io.Writer) *Writer {
	return &Writer{}
}

// Write 缓冲数据，每满一块编码输出一次。
func (w *Writer) Write(p []byte) (int, error) {
	return 0, errNotImplemented
}

// Close 编码剩余数据并写入流结束标记。不关闭底层 Writer。
func (w *Writer) Close() error {
	return errNotImplemented
}

// Reader 是解压读取器。
type Reader struct{}

// NewReader 返回从 r 读取压缩流并输出原始数据的 Reader。
func NewReader(r io.Reader) *Reader {
	return &Reader{}
}

// Read 读取解压后的数据；流结束返回 io.EOF，格式错误返回 ErrCorrupt。
func (r *Reader) Read(p []byte) (int, error) {
	return 0, errNotImplemented
}
