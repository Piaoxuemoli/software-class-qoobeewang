//go:build linux

package fsmeta

// sysStatx 是 amd64 上 statx(2) 的系统调用号
// （arch/x86/entry/syscalls/syscall_64.tbl: 332）。syscall 包未导出。
const sysStatx = 332
