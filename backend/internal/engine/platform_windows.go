//go:build windows

package engine

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/pack"
)

// ownershipSupported 为 false：Windows 无 uid/gid/chown 概念，
// extractor 完全跳过属主恢复，不发 warn、不计入 Warnings（C4b 验收）。
const ownershipSupported = false

// currentOwnership 返回零值：Windows 上 uid/gid/root 均无意义。
func currentOwnership() (uid, gid int, root bool) {
	return 0, 0, false
}

// createSpecial 静默跳过 FIFO/套接字/设备节点：Windows 无法创建它们，
// 只记录本地日志文件，不发 warn 事件、不打扰界面。
func (x *extractor) createSpecial(p string, fi fsmeta.FileInfo) (skip bool, err error) {
	logSkipped(fi.Path, "平台不支持的文件类型 %s", fi.Type)
	return true, nil
}

// symlinkEntry 创建软链接；Windows 上需要开发者模式或管理员权限，
// 失败时静默跳过并记录本地日志。
func (x *extractor) symlinkEntry(p string, fi fsmeta.FileInfo) (skip bool, err error) {
	if err := os.Symlink(fi.LinkTarget, p); err != nil {
		logSkipped(fi.Path, "软链接创建失败: %v", err)
		return true, nil
	}
	return false, nil
}

// linkEntry 还原硬链接；归档损坏/不安全（ErrCorrupt/ErrUnsafePath）与
// 路径冲突（ErrExist）仍是错误，仅 os.Link 失败（如非 NTFS 目标）时静默跳过。
func (x *extractor) linkEntry(fi fsmeta.FileInfo) (skip bool, err error) {
	if err := x.link(fi); err != nil {
		if errors.Is(err, pack.ErrCorrupt) || errors.Is(err, pack.ErrUnsafePath) || errors.Is(err, fs.ErrExist) {
			return false, err
		}
		logSkipped(fi.Path, "硬链接创建失败: %v", err)
		return true, nil
	}
	return false, nil
}

// windows-skip.log 记录"平台不支持被跳过"的条目。路径：
// $DATABACKUP_SKIPLOG 或 %LOCALAPPDATA%\databackup\windows-skip.log。
// 路径懒解析一次；每条记录打开-写入-关闭，不留常驻句柄（Windows 上
// 句柄占用会阻塞文件删除）。打不开时静默丢弃——跳过本身不能演变成错误。
var (
	skipLogOnce sync.Once
	skipLogPath string
)

func skipLogTarget() string {
	skipLogOnce.Do(func() {
		if p := os.Getenv("DATABACKUP_SKIPLOG"); p != "" {
			skipLogPath = p
			return
		}
		base, err := os.UserCacheDir()
		if err == nil {
			skipLogPath = filepath.Join(base, "databackup", "windows-skip.log")
		}
	})
	return skipLogPath
}

func logSkipped(rel, format string, args ...any) {
	path := skipLogTarget()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\t%s\t%s\n",
		time.Now().Format(time.RFC3339), rel, fmt.Sprintf(format, args...))
}
