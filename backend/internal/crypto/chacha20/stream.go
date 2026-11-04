package chacha20

import (
	"errors"
	"io"
)

// 本文件是 ChaCha20 的流式封装，接口形态与 aes 包的 CBCWriter / CBCReader
// 对齐，供容器层以相同方式接入两种算法。流密码无填充：密文与明文等长，
// Writer 的 Close 不追加任何字节，只收尾状态。

// streamBufSize 是 Writer / Reader 每次批量处理的字节数（64 个密钥流块）。
// 读写过程中只使用这一固定大小的缓冲，内存占用与数据总量无关（与 aes 包一致）。
const streamBufSize = 64 * 64

// maxEmptyReads 是底层 Reader 连续返回 (0, nil) 的容忍次数，超过即返回
// io.ErrNoProgress，避免死循环（与 bufio、aes 包的约定相同）。
const maxEmptyReads = 100

// errWriterClosed 表示 Close 之后又调用了 Write。
var errWriterClosed = errors.New("chacha20: Writer 已关闭")

// Writer 把写入的明文与密钥流异或后写出。
type Writer struct {
	w      io.Writer
	c      *Cipher
	err    error // 底层写入错误，出现后一直返回
	closed bool
}

// NewWriter 返回把密文写入 w 的 Writer。
// key 不是 32 字节返回 ErrKeySize，nonce 不是 12 字节返回 ErrNonceSize。
func NewWriter(w io.Writer, key, nonce []byte) (*Writer, error) {
	c, err := NewCipher(key, nonce)
	if err != nil {
		return nil, err
	}
	return &Writer{w: w, c: c}, nil
}

// Write 加密并写出 p 的全部字节。
// 底层写入出错后，本次及之后的 Write、Close 都返回该错误；Close 之后调用
// Write 返回错误。不修改 p（遵循 io.Writer 约定）。
func (w *Writer) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.closed {
		return 0, errWriterClosed
	}
	var out [streamBufSize]byte
	written := 0
	for written < len(p) {
		k := min(len(p)-written, len(out))
		w.c.XORKeyStream(out[:k], p[written:written+k])
		n, err := w.w.Write(out[:k])
		written += n
		if err != nil {
			w.err = err
			return written, err
		}
		if n < k {
			w.err = io.ErrShortWrite
			return written, w.err
		}
	}
	return written, nil
}

// Close 是流密码的收尾：无填充要追加，只清除密钥流残留并锁定状态。
// 不关闭底层 Writer。重复调用 Close 不再改动状态。
func (w *Writer) Close() error {
	if w.err != nil {
		return w.err
	}
	if w.closed {
		return nil
	}
	w.closed = true
	w.c.block = [64]byte{} // 不在内存中残留密钥流
	w.c.off = len(w.c.block)
	return nil
}

// Reader 从密文流读取并与密钥流异或，流式解密。密文与明文等长，读到
// 底层 io.EOF 即明文结束。
type Reader struct {
	r   io.Reader
	c   *Cipher
	buf [streamBufSize]byte // 已解密、尚未交给调用者的明文所在缓冲
	out []byte              // buf 中尚未交给调用者的部分
	err error               // 粘滞错误；正常结束时为 io.EOF
}

// NewReader 返回从 r 读取密文的 Reader。
// key 不是 32 字节返回 ErrKeySize，nonce 不是 12 字节返回 ErrNonceSize。
func NewReader(r io.Reader, key, nonce []byte) (*Reader, error) {
	c, err := NewCipher(key, nonce)
	if err != nil {
		return nil, err
	}
	return &Reader{r: r, c: c}, nil
}

// Read 读取明文；流结束返回 io.EOF。出错后之后的每次调用都返回同一个错误。
func (r *Reader) Read(p []byte) (int, error) {
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

// fill 从底层读一次密文并就地解密。结果放进 r.out 或 r.err，返回本次
// 读到的字节数。
func (r *Reader) fill() int {
	n, err := r.r.Read(r.buf[:])
	if n > 0 {
		r.c.XORKeyStream(r.buf[:n], r.buf[:n])
		r.out = r.buf[:n]
	}
	switch err {
	case nil:
	case io.EOF:
		r.err = io.EOF
	default:
		r.err = err
	}
	return n
}
