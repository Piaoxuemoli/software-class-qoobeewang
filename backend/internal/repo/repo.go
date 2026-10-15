// Package repo 管理备份仓库目录（FR-14）：通过扫描目录、读取容器头尾列出备份，
// 不使用数据库或索引文件（ADR-005）。
package repo

import (
	"errors"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/container"
)

var errNotImplemented = errors.New("repo: 尚未实现")

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
	// Info 是容器头尾信息；Corrupt 为 true 时为零值。
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
	return nil, errNotImplemented
}

// List 列出仓库中的全部 .dbk（忽略 .partial 与其他文件），损坏的文件以 Corrupt 列出。
// 按创建时间降序、名字升序排列。仓库目录不存在时返回空切片。
func (r *Repo) List() ([]Backup, error) {
	return nil, errNotImplemented
}

// Resolve 把"名称或路径"解析为备份文件的绝对路径：含 "/" 视为路径，
// 否则视为仓库中的名称（可带或不带 .dbk 后缀）。不检查文件是否存在。
func (r *Repo) Resolve(nameOrPath string) (string, error) {
	return "", errNotImplemented
}

// Info 读取一个备份的信息。不存在返回 ErrNotFound，损坏返回 container.ErrCorrupt。
func (r *Repo) Info(nameOrPath string) (Backup, error) {
	return Backup{}, errNotImplemented
}

// Delete 删除一个 .dbk 文件并返回其信息（损坏的也可删除）。
func (r *Repo) Delete(nameOrPath string) (Backup, error) {
	return Backup{}, errNotImplemented
}

// ValidateName 检查备份名是否合法：1~200 字节，不含 "/" 与 NUL，不以 "." 开头。
func ValidateName(name string) error {
	return errNotImplemented
}

// DefaultName 生成默认备份名 <源目录名>-<YYYYMMDD-HHMMSS>（本地时间），
// 源为 "/" 时源目录名记作 root。
func DefaultName(source string, now time.Time) string {
	return ""
}
