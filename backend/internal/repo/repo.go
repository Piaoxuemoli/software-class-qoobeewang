// Package repo 管理备份仓库目录（FR-14）：通过扫描目录、读取容器头尾列出备份，
// 不使用数据库或索引文件（ADR-005）。
package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/container"
)

var (
	// ErrNotFound 表示备份不存在。cli 映射为 E_NOT_FOUND。
	ErrNotFound = errors.New("备份不存在")
	// ErrInvalidName 表示备份名不合法（规则见契约 §2.1 --name）或删除目标不是 .dbk 文件。
	// cli 映射为 E_USAGE。
	ErrInvalidName = errors.New("备份名不合法")
)

// Backup 是仓库中的一个备份。
type Backup struct {
	// Name 是文件名去掉 .dbk。
	Name string
	// Path 是绝对路径。
	Path string
	// Info 是容器头尾信息；Corrupt 为 true 时只填 Size（文件 stat 大小），其余为零值。
	Info container.Info
	// ModTime 是文件修改时间（损坏时代替 CreatedAt 展示）。
	ModTime time.Time
	// Corrupt 表示头部或尾部无法解析。
	Corrupt bool
}

// Repo 是一个仓库目录。
type Repo struct {
	// Dir 是仓库目录的绝对路径。
	Dir string
}

// New 返回位于 dir 的仓库（dir 转为绝对路径；不要求已存在）。
func New(dir string) (*Repo, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("解析仓库路径 %s: %w", dir, err)
	}
	return &Repo{Dir: abs}, nil
}

// List 列出仓库中的全部 .dbk（忽略 .partial 与其他文件），损坏的文件以 Corrupt 列出。
// 按创建时间降序、名字升序排列。仓库目录不存在时返回空切片。
func (r *Repo) List() ([]Backup, error) {
	entries, err := os.ReadDir(r.Dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []Backup{}, nil
		}
		return nil, fmt.Errorf("读取仓库目录 %s: %w", r.Dir, err)
	}
	backups := make([]Backup, 0, len(entries))
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), container.Ext) {
			continue
		}
		regular := e.Type().IsRegular()
		if !regular && e.Type() == 0 {
			// 文件系统不报告条目类型（DT_UNKNOWN），退回 stat 判断。
			if st, err := e.Info(); err == nil {
				regular = st.Mode().IsRegular()
			}
		}
		if !regular {
			continue
		}
		b := Backup{
			Name: strings.TrimSuffix(e.Name(), container.Ext),
			Path: filepath.Join(r.Dir, e.Name()),
		}
		st, err := e.Info()
		if err != nil {
			// 连 stat 都失败：按损坏列出，ModTime 留零值。
			b.Corrupt = true
			backups = append(backups, b)
			continue
		}
		b.ModTime = st.ModTime()
		// 文件大小对损坏项同样有意义（契约 §2.3），先于解析填入。
		b.Info.Size = st.Size()
		if info, err := readInfoAt(b.Path); err != nil {
			b.Corrupt = true // 解析失败或无权限读取，不影响其他文件
		} else {
			b.Info = info
		}
		backups = append(backups, b)
	}
	// 按"展示用创建时间"降序：损坏文件没有头部，用文件修改时间代替；
	// 时间相同按名字升序，保证顺序稳定。
	sort.Slice(backups, func(i, j int) bool {
		ti, tj := displayTime(backups[i]), displayTime(backups[j])
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return backups[i].Name < backups[j].Name
	})
	return backups, nil
}

// displayTime 是列表排序与展示用的创建时间。
func displayTime(b Backup) time.Time {
	if b.Corrupt {
		return b.ModTime
	}
	return b.Info.Header.CreatedAt
}

// readInfoAt 打开 path 并只读取容器头尾（不校验、不需要口令）。
func readInfoAt(path string) (container.Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return container.Info{}, err
	}
	defer f.Close()
	return container.ReadInfo(f)
}

// looksLikePath 报告 nameOrPath 是否应视为文件系统路径而非仓库内备份名：
// 含 "/" 或平台路径分隔符（Windows 上 "\\"，兼容 C:\dir\a.dbk 这类输入）。
func looksLikePath(nameOrPath string) bool {
	return strings.ContainsAny(nameOrPath, "/"+string(filepath.Separator))
}

// Resolve 把"名称或路径"解析为备份文件的绝对路径：像路径的按路径解析，
// 否则视为仓库中的名称（可带或不带 .dbk 后缀）。不检查文件是否存在。
func (r *Repo) Resolve(nameOrPath string) (string, error) {
	if looksLikePath(nameOrPath) {
		return filepath.Abs(nameOrPath)
	}
	name := strings.TrimSuffix(nameOrPath, container.Ext)
	if err := ValidateName(name); err != nil {
		return "", err
	}
	return filepath.Join(r.Dir, name+container.Ext), nil
}

// Info 读取一个备份的信息。不存在返回 ErrNotFound，损坏返回 container.ErrCorrupt。
func (r *Repo) Info(nameOrPath string) (Backup, error) {
	path, err := r.Resolve(nameOrPath)
	if err != nil {
		return Backup{}, err
	}
	return r.readAt(path)
}

// readAt 读取 path 处备份的信息；不存在返回包装 ErrNotFound 的错误，
// 无法解析时返回 Corrupt 的 Backup 与包装 container.ErrCorrupt 的错误。
func (r *Repo) readAt(path string) (Backup, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Backup{}, fmt.Errorf("%s: %w", path, ErrNotFound)
		}
		return Backup{}, fmt.Errorf("打开备份 %s: %w", path, err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return Backup{}, fmt.Errorf("读取备份状态 %s: %w", path, err)
	}
	b := Backup{
		Name:    strings.TrimSuffix(filepath.Base(path), container.Ext),
		Path:    path,
		ModTime: st.ModTime(),
	}
	// 文件大小对损坏项同样有意义（契约 §2.3），先于解析填入。
	b.Info.Size = st.Size()
	info, err := container.ReadInfo(f)
	if err != nil {
		b.Corrupt = true
		return b, fmt.Errorf("读取备份 %s: %v: %w", path, err, container.ErrCorrupt)
	}
	b.Info = info
	return b, nil
}

// Delete 删除一个 .dbk 文件并返回其信息（损坏的也可删除）。
func (r *Repo) Delete(nameOrPath string) (Backup, error) {
	path, err := r.Resolve(nameOrPath)
	if err != nil {
		return Backup{}, err
	}
	if !strings.HasSuffix(path, container.Ext) {
		return Backup{}, fmt.Errorf("只能删除 %s 文件: %w", container.Ext, ErrInvalidName)
	}
	st, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Backup{}, fmt.Errorf("%s: %w", path, ErrNotFound)
		}
		return Backup{}, fmt.Errorf("读取备份状态 %s: %w", path, err)
	}
	if !st.Mode().IsRegular() {
		return Backup{}, fmt.Errorf("%s 不是普通文件: %w", path, ErrInvalidName)
	}
	// 损坏的备份也允许删除：能读到多少信息算多少。
	b, _ := r.readAt(path)
	if err := os.Remove(path); err != nil {
		return Backup{}, fmt.Errorf("删除备份 %s: %w", path, err)
	}
	return b, nil
}

// ValidateName 检查备份名是否合法：1~200 字节，不含 "/" 与 NUL，不以 "." 开头。
func ValidateName(name string) error {
	if len(name) == 0 || len(name) > 200 {
		return fmt.Errorf("备份名长度 %d 不在 1~200 字节内: %w", len(name), ErrInvalidName)
	}
	if strings.ContainsAny(name, "/\x00") {
		return fmt.Errorf("备份名含 / 或 NUL: %w", ErrInvalidName)
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("备份名不能以 . 开头: %w", ErrInvalidName)
	}
	return nil
}

// DefaultName 生成默认备份名 <源目录名>-<YYYYMMDD-HHMMSS>（本地时间），
// 源为 "/" 时源目录名记作 root。
//
// 两种契约未写明的边界的处理：
//   - 源目录名以 "." 开头（隐藏目录，如 .config）：去掉全部前导点；去完为空
//     （如 "/" 清理后得到的 "."）时用 root；
//   - 结果超过 200 字节（ValidateName 上限）：在 UTF-8 字符边界上截短源目录名
//     部分，不切断多字节字符。
func DefaultName(source string, now time.Time) string {
	base := filepath.Base(filepath.Clean(source))
	base = strings.TrimLeft(base, ".")
	// Windows 卷根（C:\）或 UNC 根的 Base 是 "\"，同样记作 root。
	if base == "" || base == "/" || base == "\\" {
		base = "root"
	}
	suffix := "-" + now.In(time.Local).Format("20060102-150405")
	if len(base)+len(suffix) > 200 {
		base = truncateUTF8(base, 200-len(suffix))
	}
	return base + suffix
}

// truncateUTF8 把 s 截短到至多 n 字节，回退到最近的 UTF-8 字符边界，
// 保证不切断多字节字符。
func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
