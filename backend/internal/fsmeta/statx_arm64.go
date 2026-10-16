//go:build linux

package fsmeta

// sysStatx 是 arm64 上 statx(2) 的系统调用号
// （include/uapi/asm-generic/unistd.h: __NR3264_statx = 291）。syscall 包未导出。
const sysStatx = 291
