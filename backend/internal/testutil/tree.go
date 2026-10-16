// Package testutil 是测试专用夹具（借鉴 restic internal/archiver/testing.go）：
// 用声明式目录树描述测试输入，CreateTree 在磁盘上生成它，AssertTreeEqual 按
// 备份正确性五要素（类型、路径、数据、元数据、链接方式）比较两棵目录树，
// Golden 读写黄金文件。
//
// 比较器直接用 syscall 读取 inode 信息，不调用 fsmeta，以免用被测代码验证被测代码。
// CreateTree 与 AssertTreeEqual 只在 Linux 上可用。
package testutil

import "time"

// DefaultTime 是 CreateTree 对未指定时间的条目设置的 atime/mtime。
// 固定时间使同一份声明生成的两棵树完全一致，也使"未恢复时间"的缺陷必然暴露。
var DefaultTime = time.Date(2026, 1, 2, 3, 4, 5, 123456789, time.UTC)

// 未指定 Mode 时使用的默认权限。CreateTree 总是显式 chmod，结果与 umask 无关。
const (
	DefaultFileMode uint32 = 0o644
	DefaultDirMode  uint32 = 0o755
	DefaultNodeMode uint32 = 0o600
)

// Entry 是目录树中的一个条目：File、Dir、MetaDir、Symlink、Hardlink、FIFO、Socket、Device。
type Entry interface {
	isEntry()
}

// Owner 是数字属主与属组。设置了 Owner 的条目需要 root，非 root 时 CreateTree 跳过测试。
type Owner struct {
	UID, GID uint32
}

// File 是普通文件。Content 非空时写入 Content；否则写入 Size 字节由路径决定的伪随机数据。
// Mode 是低 12 位权限（含 setuid/setgid/sticky），0 表示 DefaultFileMode；
// Mtime/Atime 为零值时取 DefaultTime（Atime 缺省跟随 Mtime）。
type File struct {
	Content string
	Size    int64
	Mode    uint32
	Owner   *Owner
	Mtime   time.Time
	Atime   time.Time
}

// Dir 是目录，键为条目名（不含 "/"）。元数据取默认值；需要指定元数据时用 MetaDir。
type Dir map[string]Entry

// MetaDir 是带元数据的目录。
type MetaDir struct {
	Entries Dir
	Mode    uint32
	Owner   *Owner
	Mtime   time.Time
	Atime   time.Time
}

// Symlink 是软链接，Target 原样写入（可以悬空、可以是绝对路径）。软链接没有独立权限位。
type Symlink struct {
	Target string
	Owner  *Owner
	Mtime  time.Time
	Atime  time.Time
}

// Hardlink 是指向树内另一个非目录条目的硬链接，Target 是相对树根、以 "/" 分隔的路径。
// 硬链接与目标共享 inode，元数据随目标，本身不携带元数据。
type Hardlink struct {
	Target string
}

// FIFO 是命名管道。
type FIFO struct {
	Mode  uint32
	Owner *Owner
	Mtime time.Time
	Atime time.Time
}

// Socket 是 Unix 域套接字节点（mknod 创建，无监听进程）。
type Socket struct {
	Mode  uint32
	Owner *Owner
	Mtime time.Time
	Atime time.Time
}

// Device 是字符设备（Char 为 true）或块设备。创建设备需要 root，非 root 时 CreateTree 跳过测试。
type Device struct {
	Char         bool
	Major, Minor uint32
	Mode         uint32
	Owner        *Owner
	Mtime        time.Time
	Atime        time.Time
}

func (File) isEntry()     {}
func (Dir) isEntry()      {}
func (MetaDir) isEntry()  {}
func (Symlink) isEntry()  {}
func (Hardlink) isEntry() {}
func (FIFO) isEntry()     {}
func (Socket) isEntry()   {}
func (Device) isEntry()   {}
