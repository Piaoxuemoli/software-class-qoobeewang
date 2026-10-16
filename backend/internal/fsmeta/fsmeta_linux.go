//go:build linux

package fsmeta

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Lstat 读取 root/rel 的信息（不跟随软链接），返回的 FileInfo.Path 为 rel。
// rel 必须满足 FileInfo.Path 的格式要求。HardlinkTo 不由本函数填写。
// 创建时间优先通过 statx 读取 btime，不支持时 Btime 留空。
func Lstat(root, rel string) (FileInfo, error) {
	p := filepath.Join(root, filepath.FromSlash(rel))
	var st syscall.Stat_t
	if err := syscall.Lstat(p, &st); err != nil {
		return FileInfo{}, fmt.Errorf("lstat %s: %w", rel, err)
	}
	fi := FileInfo{
		Path:  rel,
		Mode:  st.Mode & 0o7777, // 只保留权限位与 setuid/setgid/sticky
		UID:   st.Uid,
		GID:   st.Gid,
		Atime: time.Unix(st.Atim.Sec, st.Atim.Nsec),
		Mtime: time.Unix(st.Mtim.Sec, st.Mtim.Nsec),
		Ctime: time.Unix(st.Ctim.Sec, st.Ctim.Nsec),
		Dev:   uint64(st.Dev),
		Ino:   st.Ino,
		Nlink: uint64(st.Nlink),
	}
	switch st.Mode & syscall.S_IFMT {
	case syscall.S_IFREG:
		fi.Type = TypeRegular
		fi.Size = st.Size // Size 只对普通文件填写
	case syscall.S_IFDIR:
		fi.Type = TypeDir
	case syscall.S_IFLNK:
		fi.Type = TypeSymlink
		target, err := os.Readlink(p)
		if err != nil {
			return FileInfo{}, fmt.Errorf("readlink %s: %w", rel, err)
		}
		fi.LinkTarget = target
	case syscall.S_IFCHR:
		fi.Type = TypeCharDevice
		fi.DevMajor, fi.DevMinor = devMajor(uint64(st.Rdev)), devMinor(uint64(st.Rdev))
	case syscall.S_IFBLK:
		fi.Type = TypeBlockDevice
		fi.DevMajor, fi.DevMinor = devMajor(uint64(st.Rdev)), devMinor(uint64(st.Rdev))
	case syscall.S_IFSOCK:
		fi.Type = TypeSocket
	case syscall.S_IFIFO:
		fi.Type = TypeFIFO
	default:
		return FileInfo{}, fmt.Errorf("lstat %s: 未知文件类型 0%o", rel, st.Mode&syscall.S_IFMT)
	}
	if t, ok := statxBtime(p); ok {
		fi.Btime = t
	}
	return fi, nil
}

// devMajor/devMinor 按 glibc 的 dev_t 编码（gnu_dev_major/gnu_dev_minor，
// sysmacros.h）把 Rdev 拆成主/次设备号；与 testutil 中的编码互逆。
func devMajor(dev uint64) uint32 {
	return uint32((dev>>8)&0xfff) | uint32((dev>>32)&^0xfff)
}

func devMinor(dev uint64) uint32 {
	return uint32(dev&0xff) | uint32((dev>>12)&^0xff)
}

// CreateNode 在 path 处创建 fi 描述的特殊节点：FIFO（mkfifo）、字符/块设备与套接字（mknod）。
// 设备节点需要 root。普通文件、目录、软/硬链接不由本函数创建。不设置元数据。
func CreateNode(path string, fi FileInfo) error {
	return errNotImplemented
}

// ApplyMetadata 把 fi 中的元数据恢复到 path：按需 lchown，然后 chmod（软链接跳过），
// 最后设置 atime/mtime（软链接设置链接本身，不跟随）。Ctime、Btime 无法设置，忽略。
// 目录应在其子项全部还原后再调用。
func ApplyMetadata(path string, fi FileInfo, opts ApplyOptions) error {
	return errNotImplemented
}
