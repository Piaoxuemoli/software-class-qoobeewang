//go:build !linux

package fsmeta

import "errors"

// Lstat 在非 Linux 平台上不受支持（目标平台为 Linux，见 AGENTS.md §3）。
func Lstat(root, rel string) (FileInfo, error) {
	return FileInfo{}, errors.ErrUnsupported
}

// CreateNode 在非 Linux 平台上不受支持。
func CreateNode(path string, fi FileInfo) error {
	return errors.ErrUnsupported
}

// ApplyMetadata 在非 Linux 平台上不受支持。
func ApplyMetadata(path string, fi FileInfo, opts ApplyOptions) error {
	return errors.ErrUnsupported
}
