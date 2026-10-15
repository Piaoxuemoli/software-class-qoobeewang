// Package huffman 实现分块流式 Huffman 压缩（FR-12）：输入按 BlockSize 分块，
// 每块统计频率、构造规范 Huffman 码表并编码，块头记录码长表。
// 编码输出与解码输入均为流，不整体读入内存。
//
// 流的字节级格式（整数均为小端序）：
//
//	流     = { 数据块 } 结束块
//	数据块 = u32 origLen | u32 compLen | u8 codeLen[256] | compLen 字节位流
//	结束块 = u32 0
//
// origLen 是块原始长度（1..BlockSize）；codeLen[i] 是字节值 i 的码长
// （0 表示未出现，只允许一个码长为 1 的单符号块或其他完整规范码）。
// 编码为规范 Huffman 码：按（码长, 符号值）升序连续分配码字；
// 位流先写码字最高位，按位填入字节的高位到低位，块末字节以 0 补齐。
// compLen 满足 ceil(origLen/8) <= compLen <= 8*origLen。
package huffman

import (
	"bufio"
	"container/heap"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// BlockSize 是每个压缩块的最大原始数据长度（字节）。
const BlockSize = 1 << 20

// maxCodeLen 是允许的最大码长。1MiB 输入的 Huffman 树深度必 ≤44
// （Fibonacci 频率上界 F(46)>1MiB），实现取 64 作为硬上限。
const maxCodeLen = 64

// headerLen 是数据块头长度：origLen + compLen + codeLen 表。
const headerLen = 4 + 4 + 256

// ErrCorrupt 表示压缩流格式错误（码表非法、截断等）。cli 映射为 E_CORRUPT。
var ErrCorrupt = errors.New("压缩数据已损坏")

// ---- 编码 -----------------------------------------------------------------

// huffNode 是 Huffman 树的节点。minSym 是该子树中最小的符号值，
// 用作同频时的平局裁决，保证编码结果确定。
type huffNode struct {
	freq   uint64
	minSym int
	sym    int // 叶子为字节值，内部节点为 -1
	l, r   *huffNode
}

// nodeHeap 按 (freq, minSym) 升序的最小堆。
type nodeHeap []*huffNode

func (h nodeHeap) Len() int { return len(h) }
func (h nodeHeap) Less(i, j int) bool {
	if h[i].freq != h[j].freq {
		return h[i].freq < h[j].freq
	}
	return h[i].minSym < h[j].minSym
}
func (h nodeHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *nodeHeap) Push(x any)   { *h = append(*h, x.(*huffNode)) }
func (h *nodeHeap) Pop() any     { old := *h; n := old[len(old)-1]; *h = old[:len(old)-1]; return n }

// codeLengths 统计 block 的字节频率并返回每个符号的码长（0 = 未出现）。
func codeLengths(block []byte) [256]uint8 {
	var freq [256]uint64
	for _, b := range block {
		freq[b]++
	}
	h := &nodeHeap{}
	for sym := 0; sym < 256; sym++ {
		if freq[sym] > 0 {
			heap.Push(h, &huffNode{freq: freq[sym], minSym: sym, sym: sym})
		}
	}
	var lengths [256]uint8
	if h.Len() == 1 {
		// 单符号块没有分叉，规定码长为 1（码字为 0），否则无法编码。
		lengths[(*h)[0].sym] = 1
		return lengths
	}
	for h.Len() > 1 {
		a, b := heap.Pop(h).(*huffNode), heap.Pop(h).(*huffNode)
		min := a.minSym
		if b.minSym < min {
			min = b.minSym
		}
		heap.Push(h, &huffNode{freq: a.freq + b.freq, minSym: min, sym: -1, l: a, r: b})
	}
	// DFS 量取每个叶子的深度。
	type item struct {
		n     *huffNode
		depth int
	}
	stack := []item{{(*h)[0], 0}}
	for len(stack) > 0 {
		it := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if it.n.sym >= 0 {
			if it.depth < 1 || it.depth > maxCodeLen {
				// 对 ≤BlockSize 的输入不可能发生；保险起见按 64 截断。
				it.depth = maxCodeLen
			}
			lengths[it.n.sym] = uint8(it.depth)
			continue
		}
		stack = append(stack, item{it.n.l, it.depth + 1}, item{it.n.r, it.depth + 1})
	}
	return lengths
}

// canonicalCodes 把码长表转换为规范码字：按（码长, 符号值）升序，
// 同码长连续编号。返回每个符号的码字（低位对齐）。
func canonicalCodes(lengths [256]uint8) [256]uint64 {
	var blCount [maxCodeLen + 1]uint64
	for _, l := range lengths {
		blCount[l]++
	}
	var next [maxCodeLen + 1]uint64
	var code uint64
	for bits := 1; bits <= maxCodeLen; bits++ {
		code = (code + blCount[bits-1]) << 1
		next[bits] = code
	}
	var codes [256]uint64
	for sym := 0; sym < 256; sym++ {
		l := lengths[sym]
		if l > 0 {
			codes[sym] = next[l]
			next[l]++
		}
	}
	return codes
}

// bitWriter 把位按"字节高位到低位"的顺序填入缓冲。
type bitWriter struct {
	buf   []byte
	acc   uint8
	nbits int
}

func (w *bitWriter) writeCode(code uint64, length int) {
	for i := length - 1; i >= 0; i-- {
		w.acc = w.acc<<1 | uint8(code>>i)&1
		if w.nbits++; w.nbits == 8 {
			w.buf = append(w.buf, w.acc)
			w.acc, w.nbits = 0, 0
		}
	}
}

// finish 把不足一字节的剩余位以 0 补齐写入。
func (w *bitWriter) finish() {
	if w.nbits > 0 {
		w.buf = append(w.buf, w.acc<<(8-w.nbits))
		w.acc, w.nbits = 0, 0
	}
}

// encodeBlock 编码一块原始数据，返回完整的"数据块"字节串。
func encodeBlock(block []byte) []byte {
	lengths := codeLengths(block)
	codes := canonicalCodes(lengths)
	bw := &bitWriter{buf: make([]byte, 0, len(block)/2)}
	for _, b := range block {
		bw.writeCode(codes[b], int(lengths[b]))
	}
	bw.finish()

	out := make([]byte, headerLen, headerLen+len(bw.buf))
	binary.LittleEndian.PutUint32(out[0:4], uint32(len(block)))
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(bw.buf)))
	copy(out[8:], lengths[:])
	return append(out, bw.buf...)
}

// Writer 是压缩写入器。
type Writer struct {
	w     io.Writer
	buf   []byte // 未满一块的原始数据
	err   error
	ended bool
}

// NewWriter 返回把压缩结果写入 w 的 Writer。
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w, buf: make([]byte, 0, BlockSize)}
}

// Write 缓冲数据，每满一块编码输出一次。
func (w *Writer) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.ended {
		return 0, errors.New("huffman: Writer 已关闭")
	}
	n := len(p)
	for len(p) > 0 {
		room := BlockSize - len(w.buf)
		take := min(room, len(p))
		w.buf = append(w.buf, p[:take]...)
		p = p[take:]
		if len(w.buf) == BlockSize {
			if err := w.flush(); err != nil {
				return n - len(p), err
			}
		}
	}
	return n, nil
}

// flush 编码当前缓冲并写出数据块。
func (w *Writer) flush() error {
	if len(w.buf) == 0 {
		return nil
	}
	if _, w.err = w.w.Write(encodeBlock(w.buf)); w.err != nil {
		return w.err
	}
	w.buf = w.buf[:0]
	return nil
}

// Close 编码剩余数据并写入流结束标记。不关闭底层 Writer。
func (w *Writer) Close() error {
	if w.err != nil {
		return w.err
	}
	if w.ended {
		return nil
	}
	w.ended = true
	if err := w.flush(); err != nil {
		return err
	}
	var end [4]byte
	_, w.err = w.w.Write(end[:])
	return w.err
}

// ---- 解码 -----------------------------------------------------------------

// decodeTable 是按（码长, 符号值）排序的规范码解码表。
type decodeTable struct {
	count  [maxCodeLen + 1]uint64 // 各码长的码字个数
	first  [maxCodeLen + 1]uint64 // 各码长的首个码字
	idx    [maxCodeLen + 1]int    // 各码长在 syms 中的起始下标
	syms   [256]uint8             // 按（码长, 符号值）排序的符号
	maxLen int
}

// buildDecode 校验码长表并构造解码表。Kraft 和不满足
// sum(2^-len) <= 1（过订阅）时返回 ErrCorrupt。
func buildDecode(lengths []byte) (*decodeTable, error) {
	t := &decodeTable{}
	used := 0
	for _, l := range lengths {
		if l > maxCodeLen {
			return nil, fmt.Errorf("码长 %d 超过上限: %w", l, ErrCorrupt)
		}
		if l > 0 {
			t.count[l]++
			used++
		}
	}
	if used == 0 {
		return nil, fmt.Errorf("码表为空: %w", ErrCorrupt)
	}
	for l := 1; l <= maxCodeLen; l++ {
		if t.count[l] > 0 {
			t.maxLen = l
		}
	}
	// Kraft 不等式：sum(2^(maxLen-l)) <= 2^maxLen。
	var kraft uint64
	for l := 1; l <= t.maxLen; l++ {
		kraft += t.count[l] << (t.maxLen - l)
	}
	if kraft > uint64(1)<<t.maxLen {
		return nil, fmt.Errorf("码表过订阅: %w", ErrCorrupt)
	}
	// 规范码首码与符号表。
	var code uint64
	pos := 0
	for l := 1; l <= t.maxLen; l++ {
		code = (code + t.count[l-1]) << 1
		t.first[l] = code
		t.idx[l] = pos
		pos += int(t.count[l])
	}
	for sym := 0; sym < 256; sym++ {
		l := lengths[sym]
		if l > 0 {
			t.syms[t.idx[l]] = uint8(sym)
			t.idx[l]++
		}
	}
	// 回填 idx 为起始下标。
	pos = 0
	for l := 1; l <= t.maxLen; l++ {
		t.idx[l] = pos
		pos += int(t.count[l])
	}
	return t, nil
}

// Reader 是解压读取器。
type Reader struct {
	r    *bufio.Reader
	out  []byte // 已解压、待读出的数据
	pos  int
	done bool // 已见结束块
	err  error
}

// NewReader 返回从 r 读取压缩流并输出原始数据的 Reader。
func NewReader(r io.Reader) *Reader {
	return &Reader{r: bufio.NewReader(r)}
}

// nextBlock 读取并解压下一个数据块到 r.out；读到结束块返回 io.EOF。
func (r *Reader) nextBlock() error {
	var hdr [headerLen]byte
	// 先读 origLen，其余字段只有数据块才有。
	if _, err := io.ReadFull(r.r, hdr[:4]); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return fmt.Errorf("读取块头: %v: %w", err, ErrCorrupt)
	}
	origLen := binary.LittleEndian.Uint32(hdr[:4])
	if origLen == 0 {
		r.done = true
		return io.EOF
	}
	if origLen > BlockSize {
		return fmt.Errorf("块长度 %d 超过上限: %w", origLen, ErrCorrupt)
	}
	if _, err := io.ReadFull(r.r, hdr[4:]); err != nil {
		return fmt.Errorf("读取块头: %v: %w", err, ErrCorrupt)
	}
	compLen := uint64(binary.LittleEndian.Uint32(hdr[4:8]))
	if compLen == 0 || compLen > 8*uint64(origLen) || compLen < uint64(origLen)/8 {
		return fmt.Errorf("压缩长度 %d 非法: %w", compLen, ErrCorrupt)
	}
	table, err := buildDecode(hdr[8:])
	if err != nil {
		return err
	}
	comp := make([]byte, compLen)
	if _, err := io.ReadFull(r.r, comp); err != nil {
		return fmt.Errorf("读取压缩数据: %v: %w", err, ErrCorrupt)
	}
	out, err := decodeBlock(table, comp, int(origLen))
	if err != nil {
		return err
	}
	r.out, r.pos = out, 0
	return nil
}

// decodeBlock 按规范码表解码 comp 中的位流，产出 origLen 字节。
func decodeBlock(t *decodeTable, comp []byte, origLen int) ([]byte, error) {
	out := make([]byte, origLen)
	bitLen := len(comp) * 8
	bitPos := 0
	readBit := func() (uint64, bool) {
		if bitPos >= bitLen {
			return 0, false
		}
		b := uint64(comp[bitPos>>3] >> (7 - bitPos&7) & 1)
		bitPos++
		return b, true
	}
	for i := 0; i < origLen; i++ {
		var code uint64
		matched := false
		for l := 1; l <= t.maxLen; l++ {
			bit, ok := readBit()
			if !ok {
				return nil, fmt.Errorf("位流意外结束: %w", ErrCorrupt)
			}
			code = code<<1 | bit
			if t.count[l] > 0 && code >= t.first[l] && code-t.first[l] < t.count[l] {
				out[i] = t.syms[t.idx[l]+int(code-t.first[l])]
				matched = true
				break
			}
		}
		if !matched {
			return nil, fmt.Errorf("位流中出现非法码字: %w", ErrCorrupt)
		}
	}
	return out, nil
}

// Read 读取解压后的数据；流结束返回 io.EOF，格式错误返回 ErrCorrupt。
func (r *Reader) Read(p []byte) (int, error) {
	for r.pos >= len(r.out) {
		if r.err != nil {
			return 0, r.err
		}
		if r.done {
			return 0, io.EOF
		}
		if err := r.nextBlock(); err != nil {
			if err == io.EOF {
				return 0, io.EOF
			}
			r.err = err
			return 0, err
		}
	}
	n := copy(p, r.out[r.pos:])
	r.pos += n
	return n, nil
}
