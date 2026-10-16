//go:build linux

package engine

import (
	"fmt"
	"os"
	"syscall"
)

// openSource 只读打开待备份的普通文件。O_NOFOLLOW：扫描后被换成软链接时不跟随；
// O_NONBLOCK：被换成 FIFO 时不会阻塞（对普通文件无影响）。打开后再确认仍是普通文件。
func openSource(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !st.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("%s 在扫描后已不是普通文件", path)
	}
	return f, nil
}
