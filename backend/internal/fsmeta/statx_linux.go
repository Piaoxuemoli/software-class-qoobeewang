//go:build linux

package fsmeta

import (
	"encoding/binary"
	"syscall"
	"time"
	"unsafe"
)

// statx(2) 的调用号按架构定义在 statx_<GOARCH>.go 中；未定义调用号的架构
// sysStatx 为 0，statxBtime 直接返回不支持，不发起系统调用。

// struct statx 的字段偏移（include/uapi/linux/stat.h，总长 256 字节）。
// 只读取需要的字段，其余位置按偏移跳过。
const (
	statxBufLen = 256 // sizeof(struct statx)
	stxMaskOff  = 0   // stx_mask __u32：内核实际填充了哪些字段
	stxBtimeOff = 80  // stx_btime struct statx_timestamp
	// STATX_BTIME，include/uapi/linux/stat.h：请求/返回 btime（创建时间）。
	statxBtimeMask = 0x0800
)

// statx_timestamp 布局：{__s64 tv_sec; __u32 tv_nsec; __s32 __reserved}。
const (
	stxBtimeSecOff  = stxBtimeOff
	stxBtimeNsecOff = stxBtimeOff + 8
)

// statx(2) 参数常量，取自 include/uapi/linux/fcntl.h 与 include/uapi/linux/stat.h；
// syscall 包未导出。
const (
	atFDCWD           = -100  // AT_FDCWD：相对当前目录解析 path
	atSymlinkNofollow = 0x100 // AT_SYMLINK_NOFOLLOW：对软链接本身取信息（同 lstat）
)

// statxBtime 用原始 statx(2) 系统调用读取 path 的创建时间（btime），不跟随软链接。
// syscall 包没有封装 statx，这里用 Syscall6 直接发起并按内核头文件手工解析。
// 内核或文件系统不支持时（ENOSYS、EPERM、EINVAL，或返回的 stx_mask 不含
// STATX_BTIME）返回 ok=false，调用方把 Btime 留零值，不算错误。
func statxBtime(path string) (t time.Time, ok bool) {
	if sysStatx == 0 {
		return time.Time{}, false
	}
	p, err := syscall.BytePtrFromString(path)
	if err != nil {
		return time.Time{}, false
	}
	var buf [statxBufLen]byte
	dirfd := atFDCWD // uintptr(-100) 不能直接写成常量，先转变量
	_, _, errno := syscall.Syscall6(
		sysStatx,
		uintptr(dirfd),
		uintptr(unsafe.Pointer(p)),
		atSymlinkNofollow,
		statxBtimeMask,
		uintptr(unsafe.Pointer(&buf[0])),
		0,
	)
	if errno != 0 {
		return time.Time{}, false
	}
	if binary.LittleEndian.Uint32(buf[stxMaskOff:])&statxBtimeMask == 0 {
		return time.Time{}, false
	}
	sec := int64(binary.LittleEndian.Uint64(buf[stxBtimeSecOff:]))
	nsec := binary.LittleEndian.Uint32(buf[stxBtimeNsecOff:])
	return time.Unix(sec, int64(nsec)), true
}
