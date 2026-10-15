// Package filter 实现备份时的过滤条件（FR-05~10）：类型、名字正则、修改/创建时间、
// 尺寸、排除目录、排除用户/组。语义见 docs/architecture/cli-contract.md §2.1"过滤语义"。
package filter

import (
	"errors"
	"fmt"
	"os/user"
	"path"
	"regexp"
	"strconv"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
)

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

// filter 是编译后的 Criteria：所有可预先解析的值都固化在字段中，
// Decide 是无副作用的纯函数，可并发使用。
type compiledFilter struct {
	allTypes    bool
	types       map[fsmeta.FileType]bool
	excludeDirs map[string]bool
	nameRes     []*regexp.Regexp
	mtimeFrom   time.Time
	mtimeTo     time.Time
	ctimeFrom   time.Time
	ctimeTo     time.Time
	sizeMin     *int64
	sizeMax     *int64
	users       map[uint32]bool
	groups      map[uint32]bool
}

// New 校验并编译 Criteria（编译正则、解析用户/组名为 ID、检查区间），
// 返回组合了全部条件的 Filter。条件非法时返回包装了 ErrInvalidCriteria 的错误。
func New(c Criteria) (Filter, error) {
	f := &compiledFilter{allTypes: len(c.Types) == 0}
	for _, t := range c.Types {
		if t < fsmeta.TypeRegular || t > fsmeta.TypeFIFO {
			return nil, fmt.Errorf("未知文件类型 %d: %w", t, ErrInvalidCriteria)
		}
		if f.types == nil {
			f.types = make(map[fsmeta.FileType]bool)
		}
		f.types[t] = true
	}
	for _, d := range c.ExcludeDirs {
		if err := checkRelPath(d); err != nil {
			return nil, fmt.Errorf("排除目录 %q: %w", d, err)
		}
		if f.excludeDirs == nil {
			f.excludeDirs = make(map[string]bool)
		}
		f.excludeDirs[d] = true
	}
	for _, pattern := range c.ExcludeNames {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("排除文件名正则 %q: %v: %w", pattern, err, ErrInvalidCriteria)
		}
		f.nameRes = append(f.nameRes, re)
	}
	if !c.MtimeFrom.IsZero() && !c.MtimeTo.IsZero() && c.MtimeFrom.After(c.MtimeTo) {
		return nil, fmt.Errorf("修改时间区间下界晚于上界: %w", ErrInvalidCriteria)
	}
	if !c.CtimeFrom.IsZero() && !c.CtimeTo.IsZero() && c.CtimeFrom.After(c.CtimeTo) {
		return nil, fmt.Errorf("创建时间区间下界晚于上界: %w", ErrInvalidCriteria)
	}
	f.mtimeFrom, f.mtimeTo = c.MtimeFrom, c.MtimeTo
	f.ctimeFrom, f.ctimeTo = c.CtimeFrom, c.CtimeTo
	for _, bound := range []struct {
		name string
		v    *int64
	}{{"size-min", c.SizeMin}, {"size-max", c.SizeMax}} {
		if bound.v != nil && *bound.v < 0 {
			return nil, fmt.Errorf("%s 为负数 %d: %w", bound.name, *bound.v, ErrInvalidCriteria)
		}
	}
	if c.SizeMin != nil && c.SizeMax != nil && *c.SizeMin > *c.SizeMax {
		return nil, fmt.Errorf("大小区间下界大于上界: %w", ErrInvalidCriteria)
	}
	f.sizeMin, f.sizeMax = c.SizeMin, c.SizeMax
	var err error
	if f.users, err = resolveIDs(c.ExcludeUsers, false); err != nil {
		return nil, err
	}
	if f.groups, err = resolveIDs(c.ExcludeGroups, true); err != nil {
		return nil, err
	}
	return f, nil
}

// checkRelPath 校验相对路径符合 FileInfo.Path 的格式：非空、非绝对、
// 不含 "."、".." 段、不以 "/" 结尾。
func checkRelPath(p string) error {
	if p == "" || path.IsAbs(p) {
		return fmt.Errorf("不是相对路径: %w", ErrInvalidCriteria)
	}
	for _, seg := range splitPath(p) {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("含非法路径段 %q: %w", seg, ErrInvalidCriteria)
		}
	}
	return nil
}

func splitPath(p string) []string {
	var segs []string
	start := 0
	for i := 0; i <= len(p); i++ {
		if i == len(p) || p[i] == '/' {
			segs = append(segs, p[start:i])
			start = i + 1
		}
	}
	return segs
}

// resolveIDs 把"名称或十进制数字 ID"列表解析为数字 ID 集合。
// 纯数字直接当 ID；否则按用户/组名查询，查不到返回 ErrInvalidCriteria。
func resolveIDs(names []string, group bool) (map[uint32]bool, error) {
	if len(names) == 0 {
		return nil, nil
	}
	ids := make(map[uint32]bool, len(names))
	for _, name := range names {
		if id, err := strconv.ParseUint(name, 10, 32); err == nil {
			ids[uint32(id)] = true
			continue
		}
		var idStr string
		if group {
			g, err := user.LookupGroup(name)
			if err != nil {
				return nil, fmt.Errorf("用户组 %q 不存在: %w", name, ErrInvalidCriteria)
			}
			idStr = g.Gid
		} else {
			u, err := user.Lookup(name)
			if err != nil {
				return nil, fmt.Errorf("用户 %q 不存在: %w", name, ErrInvalidCriteria)
			}
			idStr = u.Uid
		}
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("名称 %q 对应的 ID %q 非法: %w", name, idStr, ErrInvalidCriteria)
		}
		ids[uint32(id)] = true
	}
	return ids, nil
}

// inClosed 报告 t 是否在闭区间 [from, to] 内；零值端点表示该端不限。
func inClosed(t, from, to time.Time) bool {
	return (from.IsZero() || !t.Before(from)) && (to.IsZero() || !t.After(to))
}

// hasExcludedAncestor 报告 p 的任一祖先目录是否在排除集合中。
// 正常遍历时被 SkipDir 的目录不会进入，本检查是防御性的。
func (f *compiledFilter) hasExcludedAncestor(p string) bool {
	for d := path.Dir(p); d != "." && d != "" && d != "/"; d = path.Dir(d) {
		if f.excludeDirs[d] {
			return true
		}
	}
	return false
}

// Decide 返回对 fi 的过滤结论。语义见契约 §2.1"过滤语义"：
// 排除目录命中 → SkipDir；其他目录只由 --types 决定是否记录（仍进入）；
// 非目录项必须同时通过类型、名字、时间、尺寸（仅普通文件）、用户/组全部条件。
func (f *compiledFilter) Decide(fi fsmeta.FileInfo) Decision {
	if fi.Type == fsmeta.TypeDir {
		if f.excludeDirs[fi.Path] {
			return SkipDir
		}
		if f.allTypes || f.types[fsmeta.TypeDir] {
			return Include
		}
		return Exclude
	}
	if f.hasExcludedAncestor(fi.Path) {
		return Exclude
	}
	if !f.allTypes && !f.types[fi.Type] {
		return Exclude
	}
	base := path.Base(fi.Path)
	for _, re := range f.nameRes {
		if re.MatchString(base) {
			return Exclude
		}
	}
	if !inClosed(fi.Mtime, f.mtimeFrom, f.mtimeTo) {
		return Exclude
	}
	if !inClosed(fi.CreateTime(), f.ctimeFrom, f.ctimeTo) {
		return Exclude
	}
	if fi.Type == fsmeta.TypeRegular {
		if f.sizeMin != nil && fi.Size < *f.sizeMin {
			return Exclude
		}
		if f.sizeMax != nil && fi.Size > *f.sizeMax {
			return Exclude
		}
	}
	if f.users[fi.UID] || f.groups[fi.GID] {
		return Exclude
	}
	return Include
}
