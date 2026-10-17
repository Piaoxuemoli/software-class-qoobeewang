//go:build windows

package fsmeta

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Windows 平台差异（整体设计见 docs/plans/C4b-windows-portable.md）：
//   - UID/GID 恒为 0：Windows 无属主/属组概念，属主过滤与属主恢复无意义。
//   - Ctime 退化为 Mtime（NTFS 无 change time）；Btime 取 NTFS 创建时间，
//     比 Linux 的 statx btime 覆盖更稳。
//   - Mode 只剩只读位语义：普通文件 0o666（只读 0o444）、目录 0o755、软链接 0o777，
//     保证 Windows→Windows 往返能恢复只读属性，跨平台归档落到 Linux 也是合理权限。
//   - 目录型 reparse point（junction 等）按软链接处理：Go 未必对其标 ModeSymlink，
//     若当目录递归，自指 junction（如 %USERPROFILE%\AppData\Local\Application Data）
//     会造成遍历死循环。取不到目标时 LinkTarget 置空，不进入、不报错。
//   - Dev/Ino/Nlink 经 GetFileInformationByHandle 尽力获取（硬链接识别）；
//     打开失败则 Nlink=1，同 inode 的多个硬链接退化为各存一份完整数据（正确性不受影响）。

// Lstat 读取 root/rel 的信息（不跟随软链接），返回的 FileInfo.Path 为 rel。
// rel 必须满足 FileInfo.Path 的格式要求。HardlinkTo 不由本函数填写。
func Lstat(root, rel string) (FileInfo, error) {
	p := filepath.Join(root, filepath.FromSlash(rel))
	st, err := os.Lstat(p)
	if err != nil {
		return FileInfo{}, fmt.Errorf("lstat %s: %w", rel, err)
	}
	m := st.Mode()
	fi := FileInfo{Path: rel}
	attrs, hasAttrs := st.Sys().(*syscall.Win32FileAttributeData)
	if hasAttrs {
		fi.Atime = filetime(attrs.LastAccessTime)
		fi.Mtime = filetime(attrs.LastWriteTime)
		fi.Btime = filetime(attrs.CreationTime)
		fi.Ctime = fi.Mtime
	}
	switch {
	case m.IsRegular():
		fi.Type = TypeRegular
		fi.Size = st.Size()
		fi.Mode = 0o666
		if hasAttrs && attrs.FileAttributes&syscall.FILE_ATTRIBUTE_READONLY != 0 {
			fi.Mode = 0o444
		}
		fillByHandle(p, &fi)
	case m&os.ModeSymlink != 0:
		fi.Type = TypeSymlink
		fi.Mode = 0o777
		target, err := os.Readlink(p)
		if err != nil {
			return FileInfo{}, fmt.Errorf("readlink %s: %w", rel, err)
		}
		fi.LinkTarget = target
	case hasAttrs && attrs.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0:
		// junction/挂载点/其他目录型或非标准的 reparse point（Go 可能标 ModeIrregular
		// 而非 ModeSymlink）：优先按软链接存——当目录递归会因自指 junction 死循环。
		// Readlink 成功的（junction、挂载点）带目标；失败且原本像目录的仍存为
		// 空目标链接（不进入、不报错）；失败且不像目录的（如 OneDrive 占位文件）
		// 回退为普通文件，照常备份内容。
		if target, err := os.Readlink(p); err == nil {
			fi.Type = TypeSymlink
			fi.Mode = 0o777
			fi.LinkTarget = target
		} else if m.IsDir() {
			fi.Type = TypeSymlink
			fi.Mode = 0o777
		} else {
			fi.Type = TypeRegular
			fi.Size = st.Size()
			fi.Mode = 0o666
			fillByHandle(p, &fi)
		}
	case m.IsDir():
		fi.Type = TypeDir
		fi.Mode = 0o755
	case m&os.ModeNamedPipe != 0:
		fi.Type = TypeFIFO
		fi.Mode = 0o666
	case m&os.ModeSocket != 0:
		fi.Type = TypeSocket
		fi.Mode = 0o666
	case m&os.ModeDevice != 0:
		// Windows 不区分字符/块设备，统一记为字符设备。
		fi.Type = TypeCharDevice
		fi.Mode = 0o666
	default:
		return FileInfo{}, fmt.Errorf("lstat %s: 未知文件类型 %s", rel, m.String())
	}
	if fi.Nlink == 0 {
		fi.Nlink = 1
	}
	return fi, nil
}

// filetime 把 Windows FILETIME（100ns 间隔，1601 纪元）转为 time.Time。
func filetime(ft syscall.Filetime) time.Time {
	return time.Unix(0, ft.Nanoseconds())
}

// fillByHandle 用仅查询元数据的方式打开 p（不读数据、不跟随），读取
// 卷序列号/文件索引/链接数填进 Dev/Ino/Nlink。任何失败都保持零值，
// 调用方把 Nlink 当作 1：硬链接退化为完整存储，不算错误。
func fillByHandle(path string, fi *FileInfo) {
	p16, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	h, err := syscall.CreateFile(p16, 0,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return
	}
	defer syscall.CloseHandle(h)
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &info); err != nil {
		return
	}
	fi.Dev = uint64(info.VolumeSerialNumber)
	fi.Ino = uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow)
	fi.Nlink = uint64(info.NumberOfLinks)
}

// CreateNode 在 Windows 上不受支持（无 FIFO/设备节点概念）。
// 正常路径不可达：engine 还原特殊文件时已按平台静默跳过。
func CreateNode(path string, fi FileInfo) error {
	return errors.ErrUnsupported
}

// ApplyMetadata 尽力恢复元数据：普通文件与目录恢复只读位（os.Chmod）与
// atime/mtime（os.Chtimes）；软链接直接跳过（Windows 无 lutimes 等价物，
// Chtimes 会作用于链接目标）。UID/GID、Ctime、Btime 在 Windows 无对应物，
// 忽略；opts.Chown 恒被忽略（engine 在 Windows 上不会传 true）。
func ApplyMetadata(path string, fi FileInfo, opts ApplyOptions) error {
	_ = opts
	if fi.Type != TypeRegular && fi.Type != TypeDir {
		return nil
	}
	if err := os.Chmod(path, os.FileMode(fi.Mode).Perm()); err != nil {
		return &os.PathError{Op: "chmod", Path: path, Err: err}
	}
	if err := os.Chtimes(path, fi.Atime, fi.Mtime); err != nil {
		return &os.PathError{Op: "chtimes", Path: path, Err: err}
	}
	return nil
}
