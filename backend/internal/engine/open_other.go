//go:build !linux

package engine

import "os"

// openSource 只读打开待备份的普通文件（非 Linux 平台无 O_NOFOLLOW，仅供编译）。
func openSource(path string) (*os.File, error) {
	return os.Open(path)
}
