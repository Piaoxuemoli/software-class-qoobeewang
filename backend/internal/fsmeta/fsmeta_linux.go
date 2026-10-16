//go:build linux

package fsmeta

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
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

// mkdev 按 glibc 的 gnu_dev_makedev 把主/次设备号组合为 dev_t，与 devMajor/devMinor 互逆。
func mkdev(major, minor uint32) uint64 {
	return uint64(major&0xfff)<<8 | uint64(major&^0xfff)<<32 |
		uint64(minor&0xff) | uint64(minor&^0xff)<<12
}

// nodePerm 是 CreateNode 创建节点时的临时权限（只有属主可读写）；
// 最终权限由 ApplyMetadata 设置，与 umask 无关。
const nodePerm = 0o600

// CreateNode 在 path 处创建 fi 描述的特殊节点：FIFO（mkfifo）、字符/块设备与套接字（mknod）。
// 设备节点需要 root。普通文件、目录、软/硬链接不由本函数创建。不设置元数据。
func CreateNode(path string, fi FileInfo) error {
	var err error
	switch fi.Type {
	case TypeFIFO:
		err = syscall.Mkfifo(path, nodePerm)
	case TypeSocket:
		// 套接字节点只是一个文件系统条目（没有监听进程），mknod 即可还原。
		err = syscall.Mknod(path, syscall.S_IFSOCK|nodePerm, 0)
	case TypeCharDevice:
		err = syscall.Mknod(path, syscall.S_IFCHR|nodePerm, int(mkdev(fi.DevMajor, fi.DevMinor)))
	case TypeBlockDevice:
		err = syscall.Mknod(path, syscall.S_IFBLK|nodePerm, int(mkdev(fi.DevMajor, fi.DevMinor)))
	default:
		return fmt.Errorf("CreateNode %s: 类型 %s 不是特殊节点", path, fi.Type)
	}
	if err != nil {
		return &os.PathError{Op: "mknod", Path: path, Err: err}
	}
	return nil
}

// ApplyMetadata 把 fi 中的元数据恢复到 path：按需 lchown，然后 chmod（软链接跳过），
// 最后设置 atime/mtime（软链接设置链接本身，不跟随）。Ctime、Btime 无法设置，忽略。
// 目录应在其子项全部还原后再调用。
func ApplyMetadata(path string, fi FileInfo, opts ApplyOptions) error {
	if opts.Chown {
		if err := os.Lchown(path, int(fi.UID), int(fi.GID)); err != nil {
			return err
		}
	}
	if fi.Type != TypeSymlink {
		// chown 会清除 setuid/setgid，所以 chmod 在其后；syscall.Chmod 原样传递
		// 0o7777 内的全部位（os.Chmod 需要先转换为 os.FileMode）。
		if err := syscall.Chmod(path, fi.Mode&0o7777); err != nil {
			return &os.PathError{Op: "chmod", Path: path, Err: err}
		}
	}
	return lutimes(path, fi.Atime, fi.Mtime)
}

// lutimes 用 utimensat(AT_FDCWD, path, times, AT_SYMLINK_NOFOLLOW) 设置纳秒精度的
// atime/mtime；对软链接作用于链接本身。直接构造 Timespec 而不经 UnixNano，
// 避免 1678 年以前或 2262 年以后的时间溢出。
func lutimes(path string, atime, mtime time.Time) error {
	ts := [2]syscall.Timespec{
		{Sec: atime.Unix(), Nsec: int64(atime.Nanosecond())},
		{Sec: mtime.Unix(), Nsec: int64(mtime.Nanosecond())},
	}
	p, err := syscall.BytePtrFromString(path)
	if err != nil {
		return &os.PathError{Op: "utimensat", Path: path, Err: err}
	}
	dirfd := atFDCWD
	_, _, errno := syscall.Syscall6(syscall.SYS_UTIMENSAT, uintptr(dirfd), uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&ts[0])), atSymlinkNofollow, 0, 0)
	if errno != 0 {
		return &os.PathError{Op: "utimensat", Path: path, Err: errno}
	}
	return nil
}
