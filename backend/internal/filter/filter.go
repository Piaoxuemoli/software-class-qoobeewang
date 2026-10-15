// Package filter 实现备份时的过滤条件（FR-05~10）：类型、名字正则、修改/创建时间、
// 尺寸、排除目录、排除用户/组。语义见 docs/architecture/cli-contract.md §2.1"过滤语义"。
package filter

import (
	"errors"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
)

var errNotImplemented = errors.New("filter: 尚未实现")

// ErrInvalidCriteria 表示过滤条件本身非法（正则错误、用户名不存在、区间上下界颠倒等），
// cli 映射为 E_USAGE。具体原因用 fmt.Errorf("...: %w", ErrInvalidCriteria) 包装。
var ErrInvalidCriteria = errors.New("过滤条件非法")

// Criteria 是用户给出的过滤条件，字段与 CLI 过滤参数一一对应。
// 零值表示不过滤（全部备份）。
type Criteria struct {
	// Types 只备份所列类型；空表示全部。
	Types []fsmeta.FileType
	// ExcludeDirs 是要排除的目录，相对源目录、以 "/" 分隔（绝对路径已由调用方换算）。
	ExcludeDirs []string
	// ExcludeNames 是正则（RE2）列表，文件名（不含目录部分）匹配任一则排除。
	ExcludeNames []string
	// MtimeFrom/MtimeTo、CtimeFrom/CtimeTo 是闭区间；零值表示该端不限。
	// "创建时间"使用 FileInfo.CreateTime()。
	MtimeFrom, MtimeTo time.Time
	CtimeFrom, CtimeTo time.Time
	// SizeMin/SizeMax 是闭区间（字节），只作用于普通文件；nil 表示该端不限。
	SizeMin, SizeMax *int64
	// ExcludeUsers/ExcludeGroups 是名称或十进制数字 ID。
	ExcludeUsers, ExcludeGroups []string
}

// Decision 是对一个条目的过滤结论。
type Decision int

const (
	// Include 表示记录该条目（目录还会继续进入）。
	Include Decision = iota
	// Exclude 表示不记录该条目；若是目录，仍然进入其子项。
	Exclude
	// SkipDir 只用于目录：不记录，也不进入（被 --exclude-dir 命中）。
	SkipDir
)

// Filter 对遍历到的每个条目给出结论。实现必须是并发安全的纯函数。
type Filter interface {
	Decide(fi fsmeta.FileInfo) Decision
}

// New 校验并编译 Criteria（编译正则、解析用户/组名为 ID、检查区间），
// 返回组合了全部条件的 Filter。条件非法时返回包装了 ErrInvalidCriteria 的错误。
func New(c Criteria) (Filter, error) {
	return nil, errNotImplemented
}
