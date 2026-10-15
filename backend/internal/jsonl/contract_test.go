package jsonl

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// contractPath 是契约文档相对本包目录的位置。契约测试是"只读 testdata"规则的
// 唯一例外（见 docs/conventions/testing.md）。
const contractPath = "../../../docs/architecture/cli-contract.md"

// wireTypes 把样例的种类（type，或 result/<command>）映射到对应的 Go 线上类型。
var wireTypes = map[string]func() any{
	TypeProgress:                      func() any { return new(Progress) },
	TypeLog:                           func() any { return new(Log) },
	TypeError:                         func() any { return new(Error) },
	TypeResult + "/" + CommandVersion: func() any { return new(VersionResult) },
	TypeResult + "/" + CommandBackup:  func() any { return new(BackupResult) },
	TypeResult + "/" + CommandDryRun:  func() any { return new(DryRunResult) },
	TypeResult + "/" + CommandRestore: func() any { return new(RestoreResult) },
	TypeResult + "/" + CommandList:    func() any { return new(ListResult) },
	TypeResult + "/" + CommandInfo:    func() any { return new(InfoResult) },
	TypeResult + "/" + CommandDelete:  func() any { return new(DeleteResult) },
	TypeResult + "/" + CommandConfig:  func() any { return new(ConfigResult) },
}

func readContract(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("读取契约文档: %v", err)
	}
	return string(b)
}

// contractSamples 返回契约文档中所有 ```jsonl 代码块里的非空行。
func contractSamples(t *testing.T, doc string) []string {
	t.Helper()
	var samples []string
	in := false
	sc := bufio.NewScanner(strings.NewReader(doc))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		switch {
		case !in && strings.TrimSpace(line) == "```jsonl":
			in = true
		case in && strings.TrimSpace(line) == "```":
			in = false
		case in && strings.TrimSpace(line) != "":
			samples = append(samples, line)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(samples) == 0 {
		t.Fatal("契约文档中没有 jsonl 样例")
	}
	return samples
}

func sampleKind(t *testing.T, line string) string {
	t.Helper()
	var head struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(line), &head); err != nil {
		t.Fatalf("样例不是合法 JSON: %v\n%s", err, line)
	}
	if head.Type == TypeResult {
		return head.Type + "/" + head.Command
	}
	return head.Type
}

func TestContractSamples(t *testing.T) {
	seen := map[string]bool{}
	for _, line := range contractSamples(t, readContract(t)) {
		kind := sampleKind(t, line)
		newT, ok := wireTypes[kind]
		if !ok {
			t.Errorf("样例种类 %q 没有对应的 Go 类型:\n%s", kind, line)
			continue
		}
		seen[kind] = true

		v := newT()
		dec := json.NewDecoder(strings.NewReader(line))
		dec.DisallowUnknownFields()
		if err := dec.Decode(v); err != nil {
			t.Errorf("样例无法解码为 %T: %v\n%s", v, err, line)
			continue
		}
		got, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, []byte(line)) {
			t.Errorf("%T 重新编码与契约样例不一致（字段名/顺序/可选性）\n契约: %s\nGo:   %s", v, line, got)
		}
	}
	for kind := range wireTypes {
		if !seen[kind] {
			t.Errorf("Go 线上类型 %q 在契约文档中没有样例", kind)
		}
	}
}

func TestContractErrorCodes(t *testing.T) {
	doc := readContract(t)
	start := strings.Index(doc, "## 4.")
	end := strings.Index(doc, "## 5.")
	if start < 0 || end < start {
		t.Fatal("契约文档缺少 §4/§5 标题")
	}
	found := regexp.MustCompile("`(E_[A-Z_]+)`").FindAllStringSubmatch(doc[start:end], -1)
	var docCodes []string
	for _, m := range found {
		if !slices.Contains(docCodes, m[1]) {
			docCodes = append(docCodes, m[1])
		}
	}
	slices.Sort(docCodes)
	goCodes := slices.Clone(ErrorCodes)
	slices.Sort(goCodes)
	if !slices.Equal(docCodes, goCodes) {
		t.Errorf("错误码不一致\n契约 §4: %v\nGo:      %v", docCodes, goCodes)
	}
}
