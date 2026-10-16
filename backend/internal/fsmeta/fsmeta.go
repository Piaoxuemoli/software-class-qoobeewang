// Package fsmeta 定义备份中"一个文件系统对象"的信息模型 FileInfo，
// 并提供读取（Lstat）、创建特殊节点（CreateNode）与恢复元数据（ApplyMetadata）。
//
// FileInfo 已在 M1-3 冻结：之后只能新增字段，不能修改或删除已有字段。
package fsmeta

import (
	"fmt"
	"time"
)

// FileType 是文件类型。取值与 CLI 的 --types 字母一一对应（见 String）。
// 硬链接不是独立类型：它是 Type 为 TypeRegular 且 HardlinkTo 非空的条目。
type FileType uint8

// 文件类型。零值不是合法类型。
const (
	TypeRegular     FileType = iota + 1 // f 普通文件
	TypeDir                             // d 目录
	TypeSymlink                         // l 软链接
	TypeCharDevice                      // c 字符设备
	TypeBlockDevice                     // b 块设备
	TypeSocket                          // s 套接字
	TypeFIFO                            // p 命名管道
)

// AllTypes 按 --types 字母顺序列出全部类型。
var AllTypes = []FileType{
	TypeRegular, TypeDir, TypeSymlink, TypeCharDevice, TypeBlockDevice, TypeSocket, TypeFIFO,
}

const typeLetters = "fdlcbsp"

// String 返回类型对应的 --types 字母（f d l c b s p），非法值返回 "?"。
func (t FileType) String() string {
	if t < TypeRegular || t > TypeFIFO {
		return "?"
	}
	return typeLetters[t-1 : t]
}

// ParseFileType 把 --types 中的一个字母解析为 FileType。
func ParseFileType(letter string) (FileType, error) {
	for i := range len(typeLetters) {
		if typeLetters[i:i+1] == letter {
			return FileType(i + 1), nil
		}
	}
	return 0, fmt.Errorf("未知文件类型 %q（可选 f d l c b s p）", letter)
}

// FileInfo 描述一个文件系统对象，是 walk → filter → pack → 还原 全链路共用的数据模型。
type FileInfo struct {
	// Path 是相对源目录的路径，以 "/" 分隔，不含 "."、".." 段，不以 "/" 开头或结尾。
	Path string
	// Type 是文件类型。
	Type FileType
	// Mode 是权限位及 setuid/setgid/sticky 位（低 12 位，即 0o7777），不含类型位。
	Mode uint32
	// UID、GID 是数字属主与属组。
	UID, GID uint32
	// Size 是普通文件的数据长度（字节）；其他类型以及硬链接条目为 0。
	Size int64
	// Atime、Mtime、Ctime 是访问、修改、状态变更时间（纳秒精度）。
	Atime, Mtime, Ctime time.Time
	// Btime 是创建时间（statx 的 btime）；文件系统不支持时为零值。
	Btime time.Time
	// LinkTarget 是软链接的内容（原样保存，不解析）；其他类型为空。
	LinkTarget string
	// HardlinkTo 非空表示本条目是硬链接，指向归档中更早出现的、同一 inode 的条目的 Path；
	// 此时不携带数据。由 walk 根据 Dev+Ino 识别。
	HardlinkTo string
	// DevMajor、DevMinor 是字符/块设备的设备号；其他类型为 0。
	DevMajor, DevMinor uint32

	// Dev、Ino、Nlink 是源文件系统上的标识，只用于遍历时识别硬链接，不写入归档。
	Dev, Ino, Nlink uint64
}

// CreateTime 返回"创建时间"过滤所用的时间：有 Btime 时用 Btime，否则退化为 Ctime。
func (fi FileInfo) CreateTime() time.Time {
	if !fi.Btime.IsZero() {
		return fi.Btime
	}
	return fi.Ctime
}

// IsHardlink 报告本条目是否为指向更早条目的硬链接。
func (fi FileInfo) IsHardlink() bool {
	return fi.HardlinkTo != ""
}

// ApplyOptions 控制 ApplyMetadata 恢复哪些元数据。
type ApplyOptions struct {
	// Chown 为 true 时恢复 UID/GID（需要 root）。非 root 时调用方应置为 false 并发 warn。
	Chown bool
}
