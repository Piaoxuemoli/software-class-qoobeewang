// Package engine 编排备份与还原流水线（FR-01~04、FR-17、FR-18）：
//
//	备份：walk(+filter) → pack.Writer → [huffman.Writer] → container.Writer(→[AES+HMAC]) → <repo>/<name>.dbk.partial → 原子改名
//	还原：container.Open(先校验) → [huffman.Reader] → pack.Reader → 按类型重建 → 目录元数据最后设置
//
// 本包只属于 roadmap W3 集成泳道。进度节流（每阶段 ≤10 次/秒，阶段结束必发一次）在本包完成。
package engine

import (
	"context"
	"errors"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/container"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/filter"
)

var errNotImplemented = errors.New("engine: 尚未实现")

// 面向用户的错误类别；cli 用 errors.Is 映射为错误码（见契约 §4）。
// 下层包的哨兵错误（filter.ErrInvalidCriteria、pack.ErrCorrupt、container.ErrAuth 等）
// 原样包装向上传递，不在此重复定义。ctx 取消时返回 context.Canceled（→ E_CANCELLED）。
var (
	// ErrSourceNotFound：源目录不存在（E_NOT_FOUND）。
	ErrSourceNotFound = errors.New("源目录不存在")
	// ErrSourceNotDir：源路径不是目录（E_USAGE）。
	ErrSourceNotDir = errors.New("源路径不是目录")
	// ErrInvalidOptions：选项非法，如排除目录在源目录之外（E_USAGE）。
	ErrInvalidOptions = errors.New("选项非法")
	// ErrExists：同名备份已存在（E_EXISTS）。
	ErrExists = errors.New("同名备份已存在")
	// ErrBackupNotFound：备份文件不存在（E_NOT_FOUND）。
	ErrBackupNotFound = errors.New("备份不存在")
	// ErrTargetNotEmpty：还原目标已存在且不是空目录（E_TARGET_NOT_EMPTY）。
	ErrTargetNotEmpty = errors.New("还原目录不为空")
)

// Phase 是进度阶段。
type Phase string

// 进度阶段，取值与契约 progress.phase 相同。
const (
	PhaseScan    Phase = "scan"
	PhasePack    Phase = "pack"
	PhaseRestore Phase = "restore"
)

// Progress 是一次进度报告，口径见契约 §3.1。
type Progress struct {
	Phase        Phase
	Files, Bytes int64
	// HasTotal 为 true 时 TotalFiles/TotalBytes 有效（pack、restore 阶段）。
	HasTotal               bool
	TotalFiles, TotalBytes int64
	Current                string
}

// Level 是日志级别。
type Level int

// 日志级别。
const (
	LevelInfo Level = iota
	LevelWarn
	LevelError
)

// Reporter 接收进度与日志。engine 在同一个 goroutine 中调用它；Progress 已节流。
type Reporter interface {
	Progress(p Progress)
	Log(level Level, message string)
}

// BackupOptions 是一次备份（或预览）的选项。
type BackupOptions struct {
	// Source 是源目录（相对路径按当前目录解析）。
	Source string
	// Repo 是仓库目录（不存在则创建）。
	Repo string
	// Name 为空时使用 repo.DefaultName。
	Name string
	// Criteria 是过滤条件；ExcludeDirs 可以是源目录内的绝对路径，由 engine 换算为相对路径。
	Criteria filter.Criteria
	// Compression 是压缩算法。
	Compression container.Compression
	// Password 非 nil 表示用 AES-256-CBC 加密。engine 不保留其引用。
	Password []byte
}

// BackupResult 是备份结果，对应契约 backup result。
type BackupResult struct {
	Name, Path                      string
	Files, Dirs, Bytes, OutputBytes int64
	Compressed, Encrypted           bool
	Skipped, Warnings               int64
	Duration                        time.Duration
}

// DryRunResult 是预览结果，对应契约 dry-run result。
type DryRunResult struct {
	Files, Dirs, Bytes, Skipped int64
}

// RestoreOptions 是一次还原的选项。
type RestoreOptions struct {
	// Backup 是备份文件路径（名称已由调用方经 repo.Resolve 解析）。
	Backup string
	// Target 是还原目录：必须不存在或为空。
	Target string
	// Password 用于加密备份；备份未加密时提供则发 warn 并忽略。engine 不保留其引用。
	Password []byte
}

// RestoreResult 是还原结果，对应契约 restore result。
type RestoreResult struct {
	Target                       string
	Files, Dirs, Bytes, Warnings int64
	Duration                     time.Duration
}

// Backup 执行备份：先 scan 得到总量，再 pack 一次流式写出 .partial，成功后原子改名。
// 失败或取消时删除 .partial。
func Backup(ctx context.Context, opts BackupOptions, rep Reporter) (BackupResult, error) {
	return BackupResult{}, errNotImplemented
}

// DryRun 只遍历与过滤，不写任何文件；忽略 Name、Compression、Password。
func DryRun(ctx context.Context, opts BackupOptions, rep Reporter) (DryRunResult, error) {
	return DryRunResult{}, errNotImplemented
}

// Restore 执行还原：先完整校验备份，再写出文件；取消时不回滚，发 warn 说明目标不完整。
func Restore(ctx context.Context, opts RestoreOptions, rep Reporter) (RestoreResult, error) {
	return RestoreResult{}, errNotImplemented
}
