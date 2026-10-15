package filter

import (
	"errors"
	"os/user"
	"strconv"
	"testing"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/fsmeta"
)

func mustNew(t *testing.T, c Criteria) Filter {
	t.Helper()
	f, err := New(c)
	if err != nil {
		t.Fatalf("New(%+v): %v", c, err)
	}
	return f
}

func i64(v int64) *int64 { return &v }

var (
	t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	t2 = time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
)

func file(path string, size int64, mtime time.Time) fsmeta.FileInfo {
	return fsmeta.FileInfo{Path: path, Type: fsmeta.TypeRegular, Size: size, Mtime: mtime, Ctime: mtime}
}

func dir(path string) fsmeta.FileInfo {
	return fsmeta.FileInfo{Path: path, Type: fsmeta.TypeDir, Mtime: t1, Ctime: t1}
}

func TestTypes(t *testing.T) {
	f := mustNew(t, Criteria{Types: []fsmeta.FileType{fsmeta.TypeRegular}})
	cases := []struct {
		name string
		fi   fsmeta.FileInfo
		want Decision
	}{
		{"TC-FILT-001 普通文件命中", file("a.txt", 10, t1), Include},
		{"TC-FILT-002 目录不含 d 不记录", dir("sub"), Exclude},
		{"TC-FILT-003 软链接类型不符", fsmeta.FileInfo{Path: "ln", Type: fsmeta.TypeSymlink, Mtime: t1}, Exclude},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := f.Decide(tc.fi); got != tc.want {
				t.Errorf("Decide() = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("TC-FILT-004 空 Types 全部保留", func(t *testing.T) {
		all := mustNew(t, Criteria{})
		for _, ft := range fsmeta.AllTypes {
			if got := all.Decide(fsmeta.FileInfo{Path: "x", Type: ft, Mtime: t1}); got != Include {
				t.Errorf("type %v: Decide() = %v, want Include", ft, got)
			}
		}
	})
}

func TestExcludeDirs(t *testing.T) {
	f := mustNew(t, Criteria{ExcludeDirs: []string{"node_modules", "a/b"}})
	cases := []struct {
		name string
		fi   fsmeta.FileInfo
		want Decision
	}{
		{"TC-FILT-005 命中排除目录不深入", dir("node_modules"), SkipDir},
		{"TC-FILT-006 嵌套排除目录", dir("a/b"), SkipDir},
		{"TC-FILT-007 普通目录仍进入并记录", dir("src"), Include},
		{"TC-FILT-008 前缀相似不误伤", dir("node_modules2"), Include},
		{"TC-FILT-009 排除目录内文件被排除", file("node_modules/x.js", 1, t1), Exclude},
		{"TC-FILT-010 同名文件不按目录排除", file("node_modules", 1, t1), Include},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := f.Decide(tc.fi); got != tc.want {
				t.Errorf("Decide() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExcludeNames(t *testing.T) {
	f := mustNew(t, Criteria{ExcludeNames: []string{`\.tmp$`, `^secret`}})
	cases := []struct {
		name string
		fi   fsmeta.FileInfo
		want Decision
	}{
		{"TC-FILT-011 文件名命中正则", file("x.tmp", 1, t1), Exclude},
		{"TC-FILT-012 前缀命中", file("secret.key", 1, t1), Exclude},
		{"TC-FILT-013 不命中", file("x.txt", 1, t1), Include},
		{"TC-FILT-014 目录不受名字过滤", dir("x.tmp"), Include},
		{"TC-FILT-015 正则只匹配文件名不含目录", file("x.tmp/ok.txt", 1, t1), Include},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := f.Decide(tc.fi); got != tc.want {
				t.Errorf("Decide() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMtime(t *testing.T) {
	f := mustNew(t, Criteria{MtimeFrom: t0, MtimeTo: t2})
	cases := []struct {
		name string
		fi   fsmeta.FileInfo
		want Decision
	}{
		{"TC-FILT-016 区间内", file("a", 1, t1), Include},
		{"TC-FILT-017 早于下界", file("a", 1, t0.Add(-time.Second)), Exclude},
		{"TC-FILT-018 晚于上界", file("a", 1, t2.Add(time.Second)), Exclude},
		{"TC-FILT-019 恰好下界（闭区间）", file("a", 1, t0), Include},
		{"TC-FILT-020 恰好上界（闭区间）", file("a", 1, t2), Include},
		{"TC-FILT-021 目录不受时间过滤", dir("old"), Include},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := f.Decide(tc.fi); got != tc.want {
				t.Errorf("Decide() = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("TC-FILT-022 只给下界", func(t *testing.T) {
		ff := mustNew(t, Criteria{MtimeFrom: t2})
		if got := ff.Decide(file("a", 1, t1)); got != Exclude {
			t.Errorf("Decide() = %v, want Exclude", got)
		}
	})
}

func TestCtime(t *testing.T) {
	f := mustNew(t, Criteria{CtimeFrom: t0, CtimeTo: t2})
	t.Run("TC-FILT-023 有 btime 时用 btime", func(t *testing.T) {
		fi := file("a", 1, t1)
		fi.Btime = t0.Add(-time.Second) // 创建早于下界，ctime 在区间内
		if got := f.Decide(fi); got != Exclude {
			t.Errorf("Decide() = %v, want Exclude", got)
		}
	})
	t.Run("TC-FILT-024 无 btime 时退化为 ctime", func(t *testing.T) {
		fi := file("a", 1, t1)
		if got := f.Decide(fi); got != Include {
			t.Errorf("Decide() = %v, want Include", got)
		}
	})
	t.Run("TC-FILT-025 创建时间恰好边界", func(t *testing.T) {
		fi := file("a", 1, t1)
		fi.Btime = t2
		if got := f.Decide(fi); got != Include {
			t.Errorf("Decide() = %v, want Include", got)
		}
	})
}

func TestSize(t *testing.T) {
	f := mustNew(t, Criteria{SizeMin: i64(10), SizeMax: i64(100)})
	cases := []struct {
		name string
		fi   fsmeta.FileInfo
		want Decision
	}{
		{"TC-FILT-026 区间内", file("a", 50, t1), Include},
		{"TC-FILT-027 小于下界", file("a", 9, t1), Exclude},
		{"TC-FILT-028 大于上界", file("a", 101, t1), Exclude},
		{"TC-FILT-029 恰好下界", file("a", 10, t1), Include},
		{"TC-FILT-030 恰好上界", file("a", 100, t1), Include},
		{"TC-FILT-031 软链接不受尺寸过滤", fsmeta.FileInfo{Path: "l", Type: fsmeta.TypeSymlink, Mtime: t1}, Include},
		{"TC-FILT-032 FIFO 不受尺寸过滤", fsmeta.FileInfo{Path: "p", Type: fsmeta.TypeFIFO, Mtime: t1}, Include},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := f.Decide(tc.fi); got != tc.want {
				t.Errorf("Decide() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestUsersGroups(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip("无法获取当前用户")
	}
	uid, _ := strconv.ParseUint(u.Uid, 10, 32)
	gid, _ := strconv.ParseUint(u.Gid, 10, 32)

	t.Run("TC-FILT-033 数字 UID 排除", func(t *testing.T) {
		f := mustNew(t, Criteria{ExcludeUsers: []string{u.Uid}})
		if got := f.Decide(fsmeta.FileInfo{Path: "a", Type: fsmeta.TypeRegular, UID: uint32(uid), Mtime: t1}); got != Exclude {
			t.Errorf("Decide() = %v, want Exclude", got)
		}
	})
	t.Run("TC-FILT-034 用户名排除", func(t *testing.T) {
		f := mustNew(t, Criteria{ExcludeUsers: []string{u.Username}})
		if got := f.Decide(fsmeta.FileInfo{Path: "a", Type: fsmeta.TypeRegular, UID: uint32(uid), Mtime: t1}); got != Exclude {
			t.Errorf("Decide() = %v, want Exclude", got)
		}
	})
	t.Run("TC-FILT-035 其他用户不排除", func(t *testing.T) {
		f := mustNew(t, Criteria{ExcludeUsers: []string{u.Uid}})
		other := uint32(uid) + 1
		if got := f.Decide(fsmeta.FileInfo{Path: "a", Type: fsmeta.TypeRegular, UID: other, Mtime: t1}); got != Include {
			t.Errorf("Decide() = %v, want Include", got)
		}
	})
	t.Run("TC-FILT-036 组名排除", func(t *testing.T) {
		g, err := user.LookupGroupId(u.Gid)
		if err != nil {
			t.Skip("无法按 GID 反查组名")
		}
		f := mustNew(t, Criteria{ExcludeGroups: []string{g.Name}})
		if got := f.Decide(fsmeta.FileInfo{Path: "a", Type: fsmeta.TypeRegular, GID: uint32(gid), Mtime: t1}); got != Exclude {
			t.Errorf("Decide() = %v, want Exclude", got)
		}
	})
	t.Run("TC-FILT-037 目录不受用户组过滤", func(t *testing.T) {
		f := mustNew(t, Criteria{ExcludeUsers: []string{u.Uid}, ExcludeGroups: []string{u.Gid}})
		if got := f.Decide(fsmeta.FileInfo{Path: "d", Type: fsmeta.TypeDir, UID: uint32(uid), GID: uint32(gid), Mtime: t1}); got != Include {
			t.Errorf("Decide() = %v, want Include", got)
		}
	})
}

func TestCombined(t *testing.T) {
	t.Run("TC-FILT-038 多条件须同时满足", func(t *testing.T) {
		f := mustNew(t, Criteria{
			Types:        []fsmeta.FileType{fsmeta.TypeRegular},
			SizeMin:      i64(10),
			MtimeTo:      t2,
			ExcludeNames: []string{`\.log$`},
		})
		if got := f.Decide(file("a.txt", 5, t1)); got != Exclude {
			t.Errorf("尺寸不满足: Decide() = %v, want Exclude", got)
		}
		if got := f.Decide(file("a.log", 50, t1)); got != Exclude {
			t.Errorf("名字不满足: Decide() = %v, want Exclude", got)
		}
		if got := f.Decide(file("a.txt", 50, t1)); got != Include {
			t.Errorf("全部满足: Decide() = %v, want Include", got)
		}
	})
}

func TestInvalidCriteria(t *testing.T) {
	cases := []struct {
		name string
		c    Criteria
	}{
		{"TC-FILT-039 正则非法", Criteria{ExcludeNames: []string{"[unclosed"}}},
		{"TC-FILT-040 用户名不存在", Criteria{ExcludeUsers: []string{"definitely-not-a-user-qx7"}}},
		{"TC-FILT-041 组名不存在", Criteria{ExcludeGroups: []string{"definitely-not-a-group-qx7"}}},
		{"TC-FILT-042 mtime 下界大于上界", Criteria{MtimeFrom: t2, MtimeTo: t0}},
		{"TC-FILT-043 ctime 下界大于上界", Criteria{CtimeFrom: t2, CtimeTo: t0}},
		{"TC-FILT-044 尺寸下界大于上界", Criteria{SizeMin: i64(100), SizeMax: i64(10)}},
		{"TC-FILT-045 尺寸为负", Criteria{SizeMin: i64(-1)}},
		{"TC-FILT-046 排除目录为绝对路径", Criteria{ExcludeDirs: []string{"/etc"}}},
		{"TC-FILT-047 排除目录含 .. 段", Criteria{ExcludeDirs: []string{"a/../b"}}},
		{"TC-FILT-048 未知文件类型", Criteria{Types: []fsmeta.FileType{0}}},
		{"TC-FILT-049 排除目录为空串", Criteria{ExcludeDirs: []string{""}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.c); !errors.Is(err, ErrInvalidCriteria) {
				t.Errorf("New() err = %v, want ErrInvalidCriteria", err)
			}
		})
	}
}
