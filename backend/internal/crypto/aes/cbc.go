package aes

import (
	"errors"
	"fmt"
	"io"
)

// 本文件实现 CBC 工作模式（NIST SP 800-38A §6.2）与 PKCS#7 填充（RFC 5652 §6.3）
// 的流式封装：
//
//	加密：C_1 = CIPH(P_1 ⊕ IV)，C_j = CIPH(P_j ⊕ C_(j-1))
//	解密：P_1 = CIPH^-1(C_1) ⊕ IV，P_j = CIPH^-1(C_j) ⊕ C_(j-1)
//
// PKCS#7 总是追加 n 个值为 n 的字节（1 ≤ n ≤ 16），输入恰为分组整数倍时追加一整组 0x10，
// 因此密文长度一定是正的分组整数倍，解密端总能无歧义地去除填充。
//
// 填充校验失败只返回 ErrPadding，不区分具体原因；容器层采用 encrypt-then-MAC，
// 在解密前已校验 HMAC，篡改的密文到不了这里，因此不存在填充预言攻击面。

// streamBufSize 是 CBCWriter / CBCReader 每次批量处理的密文字节数（256 个分组）。
// 读写过程中只使用这一固定大小的缓冲，内存占用与数据总量无关。
const streamBufSize = 256 * BlockSize

// maxEmptyReads 是底层 Reader 连续返回 (0, nil) 的容忍次数，超过即返回
// io.ErrNoProgress，避免死循环（与 bufio 的约定相同）。
const maxEmptyReads = 100

// errWriterClosed 表示 Close 之后又调用了 Write。
var errWriterClosed = errors.New("aes: CBCWriter 已关闭")

// cbcEncryptBlocks 以 CBC 模式加密 src 中的整组数据写入 dst（SP 800-38A §6.2）。
// prev 传入上一个密文分组（首次为 IV），返回时更新为本批最后一个密文分组。
// len(src) 必须是 BlockSize 的整数倍，dst 至少与 src 等长，二者可以是同一切片。
func cbcEncryptBlocks(c *Cipher, prev *[BlockSize]byte, dst, src []byte) {
	for i := 0; i < len(src); i += BlockSize {
		var x [BlockSize]byte
		for j := range x {
			x[j] = src[i+j] ^ prev[j] // P_j ⊕ C_(j-1)
		}
		c.Encrypt(prev[:], x[:]) // C_j，同时作为下一组的"上一个密文"
		copy(dst[i:i+BlockSize], prev[:])
	}
}

// cbcDecryptBlocks 以 CBC 模式解密 src 中的整组数据写入 dst（SP 800-38A §6.2）。
// prev 的含义与 cbcEncryptBlocks 相同。dst 与 src 可以是同一切片。
func cbcDecryptBlocks(c *Cipher, prev *[BlockSize]byte, dst, src []byte) {
	for i := 0; i < len(src); i += BlockSize {
		var cur, x [BlockSize]byte
		copy(cur[:], src[i:i+BlockSize]) // 先保存 C_j：原地解密时 src 会被覆盖
		c.Decrypt(x[:], cur[:])
		for j := range x {
			dst[i+j] = x[j] ^ prev[j] // CIPH^-1(C_j) ⊕ C_(j-1)
		}
		*prev = cur
	}
}

// newCBCState 校验密钥与 IV 长度，完成密钥扩展并复制 IV（不与调用者共享内存）。
func newCBCState(key, iv []byte) (*Cipher, [BlockSize]byte, error) {
	var prev [BlockSize]byte
	c, err := NewCipher(key)
	if err != nil {
		return nil, prev, err
	}
	if len(iv) != BlockSize {
		return nil, prev, ErrIVSize
	}
	copy(prev[:], iv)
	return c, prev, nil
}

// ---------------------------------------------------------------------------
// CBCWriter
// ---------------------------------------------------------------------------

// CBCWriter 以 CBC 模式加密写入的明文，Close 时追加 PKCS#7 填充。
type CBCWriter struct {
	w       io.Writer
	c       *Cipher
	prev    [BlockSize]byte     // 上一个密文分组，初始为 IV
	pending [BlockSize]byte     // 尚不足一组的明文
	npend   int                 // pending 中的有效字节数，0 ≤ npend < BlockSize
	out     [streamBufSize]byte // 待写出的密文
	err     error               // 底层写入错误，出现后一直返回
	closed  bool
}

// NewCBCWriter 返回把密文写入 w 的 CBCWriter。
// key 不是 32 字节返回 ErrKeySize，iv 不是 16 字节返回 ErrIVSize。
func NewCBCWriter(w io.Writer, key, iv []byte) (*CBCWriter, error) {
	c, prev, err := newCBCState(key, iv)
	if err != nil {
		return nil, err
	}
	return &CBCWriter{w: w, c: c, prev: prev}, nil
}

// Write 加密并写出所有完整分组，不足一组的部分留待后续。
// 底层写入出错后，本次及之后的 Write、Close 都返回该错误；Close 之后调用 Write 返回错误。
func (w *CBCWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.closed {
		return 0, errWriterClosed
	}
	written := 0
	for len(p) > 0 {
		// 把明文逐组凑满并加密进 out，out 满或 p 用完即写出一次。
		nout, consumed := 0, 0
		for nout < len(w.out) && len(p) > 0 {
			k := copy(w.pending[w.npend:], p)
			w.npend += k
			p = p[k:]
			consumed += k
			if w.npend == BlockSize {
				cbcEncryptBlocks(w.c, &w.prev, w.out[nout:nout+BlockSize], w.pending[:])
				nout += BlockSize
				w.npend = 0
			}
		}
		if err := w.flush(nout); err != nil {
			return written, err
		}
		written += consumed
	}
	return written, nil
}

// Close 写出带填充的最后一个分组。不关闭底层 Writer。重复调用 Close 不再写出数据。
func (w *CBCWriter) Close() error {
	if w.err != nil {
		return w.err
	}
	if w.closed {
		return nil
	}
	w.closed = true
	// PKCS#7：补 n = 16 - npend 个值为 n 的字节；npend = 0 时补一整组 0x10。
	n := BlockSize - w.npend
	for i := w.npend; i < BlockSize; i++ {
		w.pending[i] = byte(n)
	}
	cbcEncryptBlocks(w.c, &w.prev, w.out[:BlockSize], w.pending[:])
	w.pending = [BlockSize]byte{} // 不在内存中残留明文
	w.npend = 0
	return w.flush(BlockSize)
}

// flush 把 out 的前 n 字节写入底层 Writer；短写视为 io.ErrShortWrite。
func (w *CBCWriter) flush(n int) error {
	if n == 0 {
		return nil
	}
	m, err := w.w.Write(w.out[:n])
	if err == nil && m < n {
		err = io.ErrShortWrite
	}
	if err != nil {
		w.err = err
	}
	return err
}

// ---------------------------------------------------------------------------
// CBCReader
// ---------------------------------------------------------------------------

// CBCReader 从密文流读取并解密，在流末尾校验并去除 PKCS#7 填充。
//
// 由于只有读到 EOF 才知道哪一组是最后一组（带填充），CBCReader 总是扣留最近读到的
// 一个密文分组，直到底层 Reader 返回 io.EOF 才解密它、校验并去除填充。
type CBCReader struct {
	r    io.Reader
	c    *Cipher
	prev [BlockSize]byte // 上一个密文分组，初始为 IV

	in  [streamBufSize]byte // 已读入、尚未解密的密文 in[:nin]
	nin int

	buf [streamBufSize]byte // 已解密、尚未交给调用者的明文所在缓冲
	out []byte              // buf 中尚未交给调用者的部分
	err error               // 粘滞错误；正常结束时为 io.EOF
}

// NewCBCReader 返回从 r 读取密文的 CBCReader。
// key 不是 32 字节返回 ErrKeySize，iv 不是 16 字节返回 ErrIVSize。
func NewCBCReader(r io.Reader, key, iv []byte) (*CBCReader, error) {
	c, prev, err := newCBCState(key, iv)
	if err != nil {
		return nil, err
	}
	return &CBCReader{r: r, c: c, prev: prev}, nil
}

// Read 读取明文；流结束返回 io.EOF，填充或长度非法返回 ErrPadding。
// 出错后之后的每次调用都返回同一个错误。
func (r *CBCReader) Read(p []byte) (int, error) {
	empty := 0
	for {
		if len(r.out) > 0 {
			n := copy(p, r.out)
			r.out = r.out[n:]
			return n, nil
		}
		if r.err != nil {
			return 0, r.err
		}
		if len(p) == 0 {
			return 0, nil
		}
		if r.fill() == 0 && r.err == nil {
			if empty++; empty >= maxEmptyReads {
				r.err = io.ErrNoProgress
			}
		}
	}
}

// fill 从底层读一次密文，解密除"可能是最后一组"之外的所有完整分组；
// 遇到 EOF 时处理最后一组的填充。结果放进 r.out 或 r.err，返回本次读到的字节数。
func (r *CBCReader) fill() int {
	n, err := r.r.Read(r.in[r.nin:])
	r.nin += n
	switch {
	case err == io.EOF:
		r.finish()
		return n
	case err != nil:
		r.err = err
		return n
	}
	// 扣留最后一个（可能不完整的）分组：nin 字节中只有前 (nin-1)/16 组确定不是最后一组。
	// 解密后 in 中最多剩 16 字节，而 in 的容量远大于一个分组，因此下一次读取总有空间。
	if r.nin > 0 {
		r.decrypt((r.nin - 1) / BlockSize * BlockSize)
	}
	return n
}

// decrypt 解密 in 的前 k 字节（k 为分组整数倍）到 out，并把剩余密文移到 in 开头。
func (r *CBCReader) decrypt(k int) {
	cbcDecryptBlocks(r.c, &r.prev, r.buf[:k], r.in[:k])
	r.out = r.buf[:k]
	r.nin = copy(r.in[:], r.in[k:r.nin])
}

// finish 在底层 EOF 时处理扣留的最后几组：校验总长度、解密、校验并去除 PKCS#7 填充。
func (r *CBCReader) finish() {
	if r.nin == 0 || r.nin%BlockSize != 0 {
		r.err = fmt.Errorf("密文长度不是正的分组整数倍: %w", ErrPadding)
		return
	}
	r.decrypt(r.nin)
	pad, ok := checkPadding(r.out)
	if !ok {
		r.out = nil
		r.err = fmt.Errorf("PKCS#7 填充校验失败: %w", ErrPadding)
		return
	}
	r.out = r.out[:len(r.out)-pad]
	r.err = io.EOF
}

// checkPadding 校验明文末尾的 PKCS#7 填充（RFC 5652 §6.3）：最后一个字节 n 必须在
// 1..16 之间，且最后 n 个字节都等于 n。返回填充长度。plain 至少有一个分组。
func checkPadding(plain []byte) (int, bool) {
	last := plain[len(plain)-BlockSize:]
	n := int(last[BlockSize-1])
	if n < 1 || n > BlockSize {
		return 0, false
	}
	var diff byte
	for _, b := range last[BlockSize-n:] {
		diff |= b ^ byte(n)
	}
	return n, diff == 0
}
