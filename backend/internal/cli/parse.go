package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/filter"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
)

// parsedArgs 是解析结果。
type parsedArgs struct {
	values map[string][]string // 有值参数（按出现顺序）
	bools  map[string]bool     // 布尔开关
	args   []string            // 位置参数
}

// parseArgs 解析子命令参数：支持 --flag value 与 --flag=value；布尔参数
// 不接受值；可重复参数多次给出；-- 之后一律按位置参数处理。
// valueFlags 列出有值参数；repeatable 中的项可重复；boolFlags 为布尔开关。
// 未知参数、缺值、重复出现单值参数、布尔带值均返回包装 ErrUsage 的错误。
func parseArgs(args []string, valueFlags, repeatable, boolFlags map[string]bool) (*parsedArgs, error) {
	p := &parsedArgs{values: map[string][]string{}, bools: map[string]bool{}}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			p.args = append(p.args, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "--") {
			if a == "-h" {
				p.bools["help"] = true
				continue
			}
			p.args = append(p.args, a)
			continue
		}
		name, val, hasEq := strings.Cut(a[2:], "=")
		if name == "" {
			return nil, fmt.Errorf("非法参数 %q: %w", a, ErrUsage)
		}
		switch {
		case boolFlags[name]:
			if hasEq {
				return nil, fmt.Errorf("参数 --%s 不接受值: %w", name, ErrUsage)
			}
			p.bools[name] = true
		case valueFlags[name] || repeatable[name]:
			if !hasEq {
				i++
				if i >= len(args) {
					return nil, fmt.Errorf("参数 --%s 缺少值: %w", name, ErrUsage)
				}
				val = args[i]
			}
			p.values[name] = append(p.values[name], val)
		default:
			return nil, fmt.Errorf("未知参数 --%s: %w", name, ErrUsage)
		}
	}
	return p, nil
}

// get 返回单值参数；重复出现返回错误。
func (p *parsedArgs) get(name string) (string, error) {
	vs := p.values[name]
	if len(vs) > 1 {
		return "", fmt.Errorf("参数 --%s 只能出现一次: %w", name, ErrUsage)
	}
	if len(vs) == 0 {
		return "", nil
	}
	return vs[0], nil
}

// require 返回必选参数的值；缺失或为空返回错误。
func (p *parsedArgs) require(name string) (string, error) {
	v, err := p.get(name)
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", fmt.Errorf("缺少必选参数 --%s: %w", name, ErrUsage)
	}
	return v, nil
}

// list 返回可重复参数的全部值。
func (p *parsedArgs) list(name string) []string {
	return p.values[name]
}

// flag 报告布尔开关是否给出。
func (p *parsedArgs) flag(name string) bool {
	return p.bools[name]
}

// extraArgs 在不允许位置参数时报错。
func (p *parsedArgs) noPositionals() error {
	if len(p.args) > 0 {
		return fmt.Errorf("多余的位置参数 %q: %w", strings.Join(p.args, " "), ErrUsage)
	}
	return nil
}

// onePositional 要求恰好一个位置参数。
func (p *parsedArgs) onePositional(what string) (string, error) {
	if len(p.args) == 0 {
		return "", fmt.Errorf("缺少 %s: %w", what, ErrUsage)
	}
	if len(p.args) > 1 {
		return "", fmt.Errorf("多余的位置参数 %q: %w", strings.Join(p.args[1:], " "), ErrUsage)
	}
	return p.args[0], nil
}

// ---- 参数值校验 ------------------------------------------------------------

// parseTypes 解析 --types 逗号列表（如 "f,d,l"）。
func parseTypes(s string) ([]fsmeta.FileType, error) {
	if s == "" {
		return nil, fmt.Errorf("--types 为空列表: %w", ErrUsage)
	}
	var types []fsmeta.FileType
	for _, letter := range strings.Split(s, ",") {
		t, err := fsmeta.ParseFileType(letter)
		if err != nil {
			return nil, fmt.Errorf("--types: %v: %w", err, ErrUsage)
		}
		types = append(types, t)
	}
	return types, nil
}

// parseTime 解析时间参数：YYYY-MM-DD 或 YYYY-MM-DDTHH:MM:SS（本地时区）。
// 只写日期时 isTo=false 取 00:00:00，isTo=true 取 23:59:59.999999999。
func parseTime(s string, isTo bool) (time.Time, error) {
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", s, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		if isTo {
			// 用 AddDate 而非 +24h：夏令时切换日一天不是 24 小时。
			return t.AddDate(0, 0, 1).Add(-time.Nanosecond), nil
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf("时间 %q 格式应为 YYYY-MM-DD 或 YYYY-MM-DDTHH:MM:SS: %w", s, ErrUsage)
}

// parseSize 解析 --size-min/--size-max（非负整数，字节）。
func parseSize(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("尺寸 %q 应为非负整数: %w", s, ErrUsage)
	}
	return n, nil
}

// checkExcludeDir 对 --exclude-dir 做基本校验：非空、不含 ".." 段。
// 绝对路径的合法性（是否位于源目录内）由 engine 换算时判定。
func checkExcludeDir(dir string) error {
	if dir == "" {
		return fmt.Errorf("--exclude-dir 为空: %w", ErrUsage)
	}
	for _, seg := range strings.Split(dir, "/") {
		if seg == ".." {
			return fmt.Errorf("--exclude-dir %q 含 .. 段: %w", dir, ErrUsage)
		}
	}
	return nil
}

// checkName 校验 --name：1~200 字节，不含 "/" 与 NUL，不以 "." 开头。
// 与 repo.ValidateName 规则一致（repo 落地后以其为准，此处先行校验以便尽早报 E_USAGE）。
func checkName(name string) error {
	if len(name) == 0 || len(name) > 200 {
		return fmt.Errorf("备份名长度 %d 不在 1~200 字节内: %w", len(name), ErrUsage)
	}
	if strings.ContainsAny(name, "/\x00") {
		return fmt.Errorf("备份名含 / 或 NUL: %w", ErrUsage)
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("备份名不能以 . 开头: %w", ErrUsage)
	}
	return nil
}

// buildCriteria 把解析出的过滤参数组装为 filter.Criteria。
// 这里只做词法级校验（格式）；语义校验（正则、用户/组存在性、区间方向）
// 统一交给 filter.New，以免两处规则漂移。--exclude-dir 可以是绝对路径，
// 由 engine 换算为相对路径后再交给 filter，故校验时不带它。
func buildCriteria(p *parsedArgs) (filter.Criteria, error) {
	var c filter.Criteria
	if vs := p.values["types"]; vs != nil {
		if len(vs) > 1 {
			return c, fmt.Errorf("参数 --types 只能出现一次: %w", ErrUsage)
		}
		types, err := parseTypes(vs[0])
		if err != nil {
			return c, err
		}
		c.Types = types
	}
	for _, d := range p.list("exclude-dir") {
		if err := checkExcludeDir(d); err != nil {
			return c, err
		}
		c.ExcludeDirs = append(c.ExcludeDirs, d)
	}
	c.ExcludeNames = p.list("exclude-name")
	var err error
	if c.MtimeFrom, c.MtimeTo, err = parseInterval(p, "mtime"); err != nil {
		return c, err
	}
	if c.CtimeFrom, c.CtimeTo, err = parseInterval(p, "ctime"); err != nil {
		return c, err
	}
	if c.SizeMin, c.SizeMax, err = parseSizeInterval(p); err != nil {
		return c, err
	}
	c.ExcludeUsers = p.list("exclude-user")
	c.ExcludeGroups = p.list("exclude-group")
	semantic := c
	semantic.ExcludeDirs = nil
	if _, err := filter.New(semantic); err != nil {
		return c, err
	}
	return c, nil
}

// parseInterval 解析 --<name>-from/--<name>-to 闭区间。
func parseInterval(p *parsedArgs, name string) (from, to time.Time, err error) {
	var s string
	if s, err = p.get(name + "-from"); err != nil {
		return from, to, err
	} else if s != "" {
		if from, err = parseTime(s, false); err != nil {
			return from, to, err
		}
	}
	if s, err = p.get(name + "-to"); err != nil {
		return from, to, err
	} else if s != "" {
		if to, err = parseTime(s, true); err != nil {
			return from, to, err
		}
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return from, to, fmt.Errorf("--%s 区间下界晚于上界: %w", name, ErrUsage)
	}
	return from, to, nil
}

// parseSizeInterval 解析 --size-min/--size-max 闭区间。
func parseSizeInterval(p *parsedArgs) (minV, maxV *int64, err error) {
	for _, b := range []struct {
		flag string
		dst  **int64
	}{{"size-min", &minV}, {"size-max", &maxV}} {
		s, err := p.get(b.flag)
		if err != nil {
			return nil, nil, err
		}
		if s != "" {
			n, err := parseSize(s)
			if err != nil {
				return nil, nil, err
			}
			*b.dst = &n
		}
	}
	if minV != nil && maxV != nil && *minV > *maxV {
		return nil, nil, fmt.Errorf("--size 区间下界大于上界: %w", ErrUsage)
	}
	return minV, maxV, nil
}
