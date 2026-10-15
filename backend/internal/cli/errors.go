package cli

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/config"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/container"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/crypto/aes"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/engine"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/filter"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/huffman"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/jsonl"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/pack"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/repo"
)

// ErrUsage 表示命令行用法错误（参数缺失、非法、多余）。参数解析代码用
// fmt.Errorf("...: %w", ErrUsage) 包装具体原因。
var ErrUsage = errors.New("用法错误")

// classes 按顺序列出哨兵错误到错误码的映射（契约 §4）。
var classes = []struct {
	code string
	errs []error
}{
	{jsonl.CodeCancelled, []error{context.Canceled}},
	{jsonl.CodeUsage, []error{
		ErrUsage, filter.ErrInvalidCriteria, engine.ErrSourceNotDir, engine.ErrInvalidOptions,
		repo.ErrInvalidName, config.ErrInvalid,
	}},
	{jsonl.CodeAuth, []error{container.ErrAuth}},
	{jsonl.CodePasswordRequired, []error{container.ErrPasswordRequired}},
	{jsonl.CodeNotFound, []error{engine.ErrSourceNotFound, engine.ErrBackupNotFound, repo.ErrNotFound}},
	{jsonl.CodeTargetNotEmpty, []error{engine.ErrTargetNotEmpty}},
	{jsonl.CodeExists, []error{engine.ErrExists}},
	{jsonl.CodeCorrupt, []error{
		pack.ErrCorrupt, pack.ErrUnsafePath, huffman.ErrCorrupt,
		container.ErrCorrupt, container.ErrUnsupportedVersion, aes.ErrPadding,
	}},
}

// Classify 把错误映射为契约错误码。nil 返回 ""；未识别的文件系统错误为 E_IO；
// 其余为 E_INTERNAL。
func Classify(err error) string {
	if err == nil {
		return ""
	}
	for _, c := range classes {
		for _, target := range c.errs {
			if errors.Is(err, target) {
				return c.code
			}
		}
	}
	var pathErr *fs.PathError
	var linkErr *os.LinkError
	var sysErr *os.SyscallError
	if errors.As(err, &pathErr) || errors.As(err, &linkErr) || errors.As(err, &sysErr) {
		return jsonl.CodeIO
	}
	return jsonl.CodeInternal
}

// ExitCode 返回错误码对应的进程退出码（契约 §4）。"" 为 0；未知错误码为 1。
func ExitCode(code string) int {
	switch code {
	case "":
		return 0
	case jsonl.CodeUsage:
		return 2
	case jsonl.CodeAuth, jsonl.CodePasswordRequired:
		return 3
	case jsonl.CodeCancelled:
		return 4
	default:
		return 1
	}
}
