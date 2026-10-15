//go:build linux

package fsmeta

// Lstat 读取 root/rel 的信息（不跟随软链接），返回的 FileInfo.Path 为 rel。
// rel 必须满足 FileInfo.Path 的格式要求。HardlinkTo 不由本函数填写。
// 创建时间优先通过 statx 读取 btime，不支持时 Btime 留空。
func Lstat(root, rel string) (FileInfo, error) {
	return FileInfo{}, errNotImplemented
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
