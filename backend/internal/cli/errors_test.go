package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/container"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/engine"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/filter"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/jsonl"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/pack"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/repo"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{context.Canceled, jsonl.CodeCancelled},
		{fmt.Errorf("--size-min: %w", ErrUsage), jsonl.CodeUsage},
		{fmt.Errorf("正则: %w", filter.ErrInvalidCriteria), jsonl.CodeUsage},
		{fmt.Errorf("还原: %w", container.ErrAuth), jsonl.CodeAuth},
		{container.ErrPasswordRequired, jsonl.CodePasswordRequired},
		{engine.ErrSourceNotFound, jsonl.CodeNotFound},
		{repo.ErrNotFound, jsonl.CodeNotFound},
		{engine.ErrTargetNotEmpty, jsonl.CodeTargetNotEmpty},
		{engine.ErrExists, jsonl.CodeExists},
		{fmt.Errorf("条目 3: %w", pack.ErrUnsafePath), jsonl.CodeCorrupt},
		{container.ErrUnsupportedVersion, jsonl.CodeCorrupt},
		{fmt.Errorf("读取: %w", &fs.PathError{Op: "open", Path: "/x", Err: fs.ErrPermission}), jsonl.CodeIO},
		{errors.New("意外"), jsonl.CodeInternal},
	}
	for _, tc := range tests {
		if got := Classify(tc.err); got != tc.want {
			t.Errorf("Classify(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}

// TestExitCodeMatchesContract 从契约 §4 表格读取"退出码 → 错误码"，与 ExitCode 比较。
// 读取契约文档是"只读 testdata"规则的例外（见 docs/conventions/testing.md）。
func TestExitCodeMatchesContract(t *testing.T) {
	b, err := os.ReadFile("../../../docs/architecture/cli-contract.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	sec := doc[strings.Index(doc, "## 4."):strings.Index(doc, "## 5.")]
	row := regexp.MustCompile(`^\| (\d) \|`)
	code := regexp.MustCompile("`(E_[A-Z_]+)`")
	want := map[string]int{}
	for _, line := range strings.Split(sec, "\n") {
		m := row.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		for _, c := range code.FindAllStringSubmatch(line, -1) {
			want[c[1]] = int(m[1][0] - '0')
		}
	}
	if len(want) != len(jsonl.ErrorCodes) {
		t.Fatalf("契约 §4 表格有 %d 个错误码，jsonl.ErrorCodes 有 %d 个", len(want), len(jsonl.ErrorCodes))
	}
	for c, exit := range want {
		if got := ExitCode(c); got != exit {
			t.Errorf("ExitCode(%s) = %d, 契约为 %d", c, got, exit)
		}
	}
	if ExitCode("") != 0 {
		t.Error(`ExitCode("") 应为 0`)
	}
}
