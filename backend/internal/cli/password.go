package cli

import (
	"bytes"
	"fmt"
	"io"
)

// readPassword 从 stdin 读取口令：读到第一个 '\n' 或 EOF 为止，去掉结尾的 '\r'；
// 长度 1~1024 字节（契约 §1）。口令不写入任何输出。
func readPassword(stdin io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	one := make([]byte, 1)
	for {
		n, err := stdin.Read(one)
		if n == 1 {
			if one[0] == '\n' {
				break
			}
			buf.WriteByte(one[0])
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("读取口令: %w", err)
		}
		if buf.Len() > 1024 {
			break // 已超出上限，不必再读
		}
	}
	b := bytes.TrimSuffix(buf.Bytes(), []byte{'\r'})
	if len(b) == 0 {
		return nil, fmt.Errorf("口令为空: %w", ErrUsage)
	}
	if len(b) > 1024 {
		return nil, fmt.Errorf("口令超过 1024 字节: %w", ErrUsage)
	}
	return b, nil
}
