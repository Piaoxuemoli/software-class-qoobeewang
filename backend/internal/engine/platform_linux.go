//go:build linux

package engine

import (
	"errors"
	"os"
	"syscall"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
)

// ownershipSupported 报告平台是否有属主语义（uid/gid + chown）。
// 为 false 的平台（Windows）不恢复属主、不发"跳过属主"warn。
const ownershipSupported = true

// currentOwnership 返回当前进程 uid/gid 与是否 root。
func currentOwnership() (uid, gid int, root bool) {
	euid := os.Geteuid()
	return euid, os.Getegid(), euid == 0
}

// linkEntry 还原硬链接；skip 恒为 false，失败原样报错。
func (x *extractor) linkEntry(fi fsmeta.FileInfo) (skip bool, err error) {
	return false, x.link(fi)
}

// symlinkEntry 还原软链接；skip 恒为 false，失败原样报错。
func (x *extractor) symlinkEntry(p string, fi fsmeta.FileInfo) (skip bool, err error) {
	return false, os.Symlink(fi.LinkTarget, p)
}

// createSpecial 还原 FIFO/套接字/设备节点；非 root 的设备创建被 EPERM
// 拒绝时按 warn 跳过（skip=true 且 err=nil），其余错误原样返回。
func (x *extractor) createSpecial(p string, fi fsmeta.FileInfo) (skip bool, err error) {
	err = fsmeta.CreateNode(p, fi)
	if isDevice(fi.Type) && !x.root && errors.Is(err, syscall.EPERM) {
		x.warn("非 root 用户无法创建设备文件，已跳过 %s", fi.Path)
		return true, nil
	}
	return false, err
}
