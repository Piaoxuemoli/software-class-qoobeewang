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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/container"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/filter"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/pack"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/repo"
)

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
	start := time.Now()
	r := newRun(ctx, rep)
	res, err := r.backup(opts, start)
	if err = r.finish(err); err != nil {
		return BackupResult{}, err
	}
	res.Warnings, res.Duration = r.warnings, time.Since(start)
	return res, nil
}

func (r *run) backup(opts BackupOptions, start time.Time) (BackupResult, error) {
	src, err := prepare(opts)
	if err != nil {
		return BackupResult{}, err
	}
	name := opts.Name
	if name == "" {
		name = repo.DefaultName(src.Abs, start)
	}
	if err := repo.ValidateName(name); err != nil {
		return BackupResult{}, err
	}
	repoDir, err := filepath.Abs(opts.Repo)
	if err != nil {
		return BackupResult{}, fmt.Errorf("解析仓库目录 %s: %w", opts.Repo, err)
	}
	final := filepath.Join(repoDir, name+container.Ext)
	if err := checkNotExists(final); err != nil {
		return BackupResult{}, err
	}

	var entries []fsmeta.FileInfo
	stats, err := r.scan(src, func(fi fsmeta.FileInfo) { entries = append(entries, fi) })
	if err != nil {
		return BackupResult{}, err
	}

	if err := r.ensureRepo(repoDir); err != nil {
		return BackupResult{}, err
	}
	h := container.Header{Compression: opts.Compression, CreatedAt: start, Source: src.Abs}
	if opts.Password != nil {
		h.Encryption = container.EncryptionAES256CBC
	}
	partial := final + container.PartialExt
	sum, err := r.writeArchive(partial, src, h, opts.Password, entries, stats)
	if err == nil {
		err = r.publishBackup(partial, final)
	}
	if err != nil {
		if rmErr := os.Remove(partial); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			r.warn("删除未完成的备份文件 %s 失败：%v", partial, rmErr)
		}
		return BackupResult{}, err
	}
	syncDir(repoDir)

	st, err := os.Stat(final)
	if err != nil {
		return BackupResult{}, fmt.Errorf("读取备份文件: %w", err)
	}
	return BackupResult{
		Name: name, Path: final,
		Files: sum.Files, Dirs: sum.Dirs, Bytes: sum.Bytes, OutputBytes: st.Size(),
		Compressed: h.Compression != container.CompressionNone, Encrypted: h.Encryption != container.EncryptionNone,
		Skipped: stats.Skipped,
	}, nil
}

// publishBackup 把写好的 .partial 原子地发布为正式备份：先 os.Link 再删除临时文件。
// 不用 os.Rename：rename 对已存在目标是原子替换，并发同名备份中后完成者会静默覆盖
// 先完成者；link 遇已存在目标返回 EEXIST，映射为 ErrExists。
// link 成功后删除 .partial 失败只发警告——备份本身已生效，不算失败。
func (r *run) publishBackup(partial, final string) error {
	if err := os.Link(partial, final); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %s", ErrExists, final)
		}
		return fmt.Errorf("发布备份文件: %w", err)
	}
	if err := os.Remove(partial); err != nil {
		r.warn("删除临时文件 %s 失败：%v", partial, err)
	}
	return nil
}

// checkNotExists 在 path 已存在（任何类型）时返回 ErrExists。
func checkNotExists(path string) error {
	_, err := os.Lstat(path)
	switch {
	case err == nil:
		return fmt.Errorf("%w: %s", ErrExists, path)
	case errors.Is(err, fs.ErrNotExist):
		return nil
	default:
		return fmt.Errorf("访问 %s: %w", path, err)
	}
}

// ensureRepo 创建不存在的仓库目录，并发 info 日志。
func (r *run) ensureRepo(dir string) error {
	if _, err := os.Stat(dir); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建仓库目录: %w", err)
	}
	r.info("仓库目录不存在，已创建 %s", dir)
	return nil
}

// syncDir 尽力 fsync 目录，使改名在掉电后仍然生效；失败不影响结果。
func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}
}

// DryRun 只遍历与过滤，不写任何文件；忽略 Name、Compression、Password。
func DryRun(ctx context.Context, opts BackupOptions, rep Reporter) (DryRunResult, error) {
	r := newRun(ctx, rep)
	src, err := prepare(opts)
	if err != nil {
		return DryRunResult{}, err
	}
	stats, err := r.scan(src, nil)
	if err = r.finish(err); err != nil {
		return DryRunResult{}, err
	}
	return DryRunResult{Files: stats.Files, Dirs: stats.Dirs, Bytes: stats.Bytes, Skipped: stats.Skipped}, nil
}

// Restore 执行还原：先完整校验备份，再写出文件；取消时不回滚，发 warn 说明目标不完整。
func Restore(ctx context.Context, opts RestoreOptions, rep Reporter) (RestoreResult, error) {
	start := time.Now()
	r := newRun(ctx, rep)
	target, err := filepath.Abs(opts.Target)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("解析还原目录 %s: %w", opts.Target, err)
	}
	x, err := r.restore(opts, target)
	if err = r.finish(err); err != nil {
		return RestoreResult{}, err
	}
	return RestoreResult{
		Target: target, Files: x.files, Dirs: x.dirCount, Bytes: x.bytes,
		Warnings: r.warnings, Duration: time.Since(start),
	}, nil
}

func (r *run) restore(opts RestoreOptions, target string) (*extractor, error) {
	if err := checkTarget(target); err != nil {
		return nil, err
	}
	f, err := os.Open(opts.Backup)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrBackupNotFound, opts.Backup)
	}
	if err != nil {
		return nil, fmt.Errorf("打开备份: %w", err)
	}
	defer f.Close()
	if st, err := f.Stat(); err != nil {
		return nil, fmt.Errorf("读取备份状态: %w", err)
	} else if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s 不是普通文件", container.ErrCorrupt, opts.Backup)
	}
	payload, info, err := r.openBackup(f, opts.Password)
	if err != nil {
		return nil, err
	}

	// 校验已通过，从这里开始写文件。
	if err := os.MkdirAll(target, 0o755); err != nil {
		return nil, fmt.Errorf("创建还原目录: %w", err)
	}
	x := r.newExtractor(target, info.Summary)
	if err := x.extract(pack.NewReader(payload)); err != nil {
		if r.ctx.Err() != nil {
			r.warn("还原已取消，目标目录内容不完整：%s", target)
		} else {
			r.warn("还原失败，目标目录内容不完整：%s", target)
		}
		return nil, err
	}
	return x, nil
}
