package cli

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/jsonl"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/repo"
)

func TestParseArgs(t *testing.T) {
	value := map[string]bool{"source": true}
	repeat := map[string]bool{"exclude-dir": true}
	bools := map[string]bool{"json": true, "dry-run": true}

	t.Run("TC-CLI-101 --flag value 与 --flag=value", func(t *testing.T) {
		p, err := parseArgs([]string{"--source", "/a", "--source=/b"}, value, repeat, bools)
		if err != nil {
			t.Fatal(err)
		}
		if got := p.list("source"); len(got) != 2 || got[0] != "/a" || got[1] != "/b" {
			t.Errorf("source values = %v", got)
		}
	})

	t.Run("TC-CLI-102 位置参数与 flag 混排", func(t *testing.T) {
		p, err := parseArgs([]string{"name1", "--json", "name2"}, value, repeat, bools)
		if err != nil {
			t.Fatal(err)
		}
		if !p.flag("json") || len(p.args) != 2 {
			t.Errorf("args = %v, json = %v", p.args, p.flag("json"))
		}
	})

	t.Run("TC-CLI-103 未知参数", func(t *testing.T) {
		if _, err := parseArgs([]string{"--bogus"}, value, repeat, bools); !errors.Is(err, ErrUsage) {
			t.Errorf("err = %v, want ErrUsage", err)
		}
	})

	t.Run("TC-CLI-104 参数缺少值", func(t *testing.T) {
		if _, err := parseArgs([]string{"--source"}, value, repeat, bools); !errors.Is(err, ErrUsage) {
			t.Errorf("err = %v, want ErrUsage", err)
		}
	})

	t.Run("TC-CLI-105 布尔参数带值", func(t *testing.T) {
		if _, err := parseArgs([]string{"--json=1"}, value, repeat, bools); !errors.Is(err, ErrUsage) {
			t.Errorf("err = %v, want ErrUsage", err)
		}
	})

	t.Run("TC-CLI-106 双横线后按位置参数", func(t *testing.T) {
		p, err := parseArgs([]string{"--", "--not-a-flag"}, value, repeat, bools)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.args) != 1 || p.args[0] != "--not-a-flag" {
			t.Errorf("args = %v", p.args)
		}
	})
}

func TestParseTime(t *testing.T) {
	t.Run("TC-CLI-110 只写日期 from 取当天零点", func(t *testing.T) {
		got, err := parseTime("2026-06-15", false)
		if err != nil {
			t.Fatal(err)
		}
		if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 || got.Nanosecond() != 0 {
			t.Errorf("from = %v, want 00:00:00", got)
		}
	})
	t.Run("TC-CLI-111 只写日期 to 取当天最后一刻", func(t *testing.T) {
		got, err := parseTime("2026-06-15", true)
		if err != nil {
			t.Fatal(err)
		}
		if got.Hour() != 23 || got.Minute() != 59 || got.Second() != 59 || got.Nanosecond() != 999999999 {
			t.Errorf("to = %v, want 23:59:59.999999999", got)
		}
	})
	t.Run("TC-CLI-112 完整时间戳", func(t *testing.T) {
		got, err := parseTime("2026-06-15T10:20:30", false)
		if err != nil {
			t.Fatal(err)
		}
		if got.Hour() != 10 || got.Minute() != 20 {
			t.Errorf("got = %v", got)
		}
	})
	t.Run("TC-CLI-113 非法时间", func(t *testing.T) {
		if _, err := parseTime("15/06/2026", false); !errors.Is(err, ErrUsage) {
			t.Errorf("err = %v, want ErrUsage", err)
		}
	})
	t.Run("TC-CLI-114 使用本地时区", func(t *testing.T) {
		got, err := parseTime("2026-06-15", false)
		if err != nil {
			t.Fatal(err)
		}
		if got.Location() != time.Local {
			t.Errorf("location = %v, want time.Local", got.Location())
		}
	})
}

// TestParseTimeDST 临时替换 time.Local，不能与其他测试并行。
func TestParseTimeDST(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	saved := time.Local
	time.Local = ny
	t.Cleanup(func() { time.Local = saved })

	// 2026-03-08 美东进入夏令时，当天只有 23 小时；11-01 退出，当天 25 小时。
	for _, day := range []string{"2026-03-08", "2026-11-01"} {
		t.Run("TC-CLI-115 夏令时切换日 to 仍取当天最后一刻 "+day, func(t *testing.T) {
			got, err := parseTime(day, true)
			if err != nil {
				t.Fatal(err)
			}
			if got.Format("2006-01-02") != day || got.Hour() != 23 || got.Minute() != 59 || got.Nanosecond() != 999999999 {
				t.Errorf("to = %v, want %s 23:59:59.999999999", got, day)
			}
		})
	}
}

// TestCheckName：--name 的规则收敛到 repo.ValidateName（M6-3），这里锁定 CLI 侧的口径：
// 非法名字映射为 E_USAGE，消息去掉哨兵后缀。
func TestCheckName(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"TC-CLI-120 合法名字", "docs-20261015", false},
		{"TC-CLI-121 含斜杠", "a/b", true},
		{"TC-CLI-122 点开头", ".hidden", true},
		{"TC-CLI-123 超长", strings.Repeat("a", 201), true},
		{"TC-CLI-124 空名字", "", true},
		{"TC-CLI-125 含 NUL", "a\x00b", true},
		{"TC-CLI-126 恰好 200 字节", strings.Repeat("名", 66) + "ab", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := repo.ValidateName(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateName(%q) err = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
			if err != nil {
				if code := Classify(err); code != jsonl.CodeUsage {
					t.Errorf("Classify = %s, want E_USAGE", code)
				}
				if msg := message(err); strings.HasSuffix(msg, repo.ErrInvalidName.Error()) {
					t.Errorf("message 未去掉哨兵后缀: %q", msg)
				}
			}
		})
	}
}

func TestReadPassword(t *testing.T) {
	t.Run("TC-CLI-130 读到换行为止", func(t *testing.T) {
		pw, err := readPassword(strings.NewReader("secret\nrest"))
		if err != nil {
			t.Fatal(err)
		}
		if string(pw) != "secret" {
			t.Errorf("pw = %q", pw)
		}
	})
	t.Run("TC-CLI-131 去掉结尾回车", func(t *testing.T) {
		pw, err := readPassword(strings.NewReader("secret\r\n"))
		if err != nil {
			t.Fatal(err)
		}
		if string(pw) != "secret" {
			t.Errorf("pw = %q", pw)
		}
	})
	t.Run("TC-CLI-132 EOF 结束", func(t *testing.T) {
		pw, err := readPassword(strings.NewReader("secret"))
		if err != nil {
			t.Fatal(err)
		}
		if string(pw) != "secret" {
			t.Errorf("pw = %q", pw)
		}
	})
	t.Run("TC-CLI-133 空口令", func(t *testing.T) {
		if _, err := readPassword(strings.NewReader("")); !errors.Is(err, ErrUsage) {
			t.Errorf("err = %v, want ErrUsage", err)
		}
		if _, err := readPassword(strings.NewReader("\n")); !errors.Is(err, ErrUsage) {
			t.Errorf("err = %v, want ErrUsage", err)
		}
	})
	t.Run("TC-CLI-134 超长口令", func(t *testing.T) {
		if _, err := readPassword(strings.NewReader(strings.Repeat("a", 1025))); !errors.Is(err, ErrUsage) {
			t.Errorf("err = %v, want ErrUsage", err)
		}
	})
	t.Run("TC-CLI-135 上限 1024 字节", func(t *testing.T) {
		pw, err := readPassword(strings.NewReader(strings.Repeat("a", 1024)))
		if err != nil {
			t.Fatal(err)
		}
		if len(pw) != 1024 {
			t.Errorf("len = %d", len(pw))
		}
	})
}
