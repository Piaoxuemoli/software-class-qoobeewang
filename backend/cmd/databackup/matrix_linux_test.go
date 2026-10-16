//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/cli"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/testutil"
)

// matrixTree 是组合矩阵的源目录：覆盖类型、名字、时间、尺寸、目录维度都能区分的条目。
// walk 按名字排序，"hard" 先于 "src/main.go" 出现，因此 hard 存数据、main.go 记为硬链接。
func matrixTree() testutil.Dir {
	day := func(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 12, 0, 0, 0, time.Local) }
	return testutil.Dir{
		"build": testutil.Dir{"out.o": testutil.File{Size: 2000}},
		"docs": testutil.Dir{
			"big.bin":   testutil.File{Size: 5000, Mtime: day(2024, 6, 1)},
			"readme.md": testutil.File{Content: strings.Repeat("readme ", 10), Mtime: day(2024, 1, 10)},
		},
		"hard": testutil.Hardlink{Target: "src/main.go"},
		"link": testutil.Symlink{Target: "docs/readme.md"},
		"logs": testutil.Dir{"app.log": testutil.File{Content: "log line\n"}},
		"pipe": testutil.FIFO{},
		"src": testutil.Dir{
			"main.go":      testutil.File{Content: "package main\n", Mtime: day(2025, 3, 1)},
			"main_test.go": testutil.File{Content: "package main_test\n"},
		},
	}
}

var allMatrixPaths = []string{
	"build", "build/out.o", "docs", "docs/big.bin", "docs/readme.md", "hard", "link",
	"logs", "logs/app.log", "pipe", "src", "src/main.go", "src/main_test.go",
}

func without(paths []string, drop ...string) []string {
	return slices.DeleteFunc(slices.Clone(paths), func(p string) bool { return slices.Contains(drop, p) })
}

// prune 返回只保留 keep 中路径的声明式目录树（用于生成"期望的还原结果"）。
func prune(d testutil.Dir, dir string, keep []string) testutil.Dir {
	out := testutil.Dir{}
	for name, e := range d {
		rel := path.Join(dir, name)
		if !slices.Contains(keep, rel) {
			continue
		}
		if sub, ok := e.(testutil.Dir); ok {
			e = prune(sub, rel, keep)
		}
		out[name] = e
	}
	return out
}

// TestFilterCompressEncryptMatrix（TC-CLI-400）：过滤 × 压缩 × 加密组合，经真实 CLI 入口
// 备份再还原；还原结果与"按过滤语义裁剪后的源目录"按五要素一致；JSONL 输出合法、
// 最后一行为 result、各阶段进度单调且末次等于结果（M6-3）。
func TestFilterCompressEncryptMatrix(t *testing.T) {
	uid := strconv.Itoa(os.Geteuid())
	dirs := []string{"build", "docs", "logs", "src"}
	filters := []struct {
		name string
		args []string
		keep []string
		// implicitDirs：--types 不含 d 时父目录由还原自动创建（0755、当前时间），不比较时间。
		implicitDirs bool
	}{
		{"无过滤", nil, allMatrixPaths, false},
		{"类型 f", []string{"--types", "f"}, without(allMatrixPaths, "link", "pipe"), true},
		{"排除目录", []string{"--exclude-dir", "build"}, without(allMatrixPaths, "build", "build/out.o"), false},
		{"排除文件名", []string{"--exclude-name", `_test\.go$`, "--exclude-name", `\.log$`},
			without(allMatrixPaths, "src/main_test.go", "logs/app.log"), false},
		{"修改时间", []string{"--mtime-from", "2024-05-01", "--mtime-to", "2025-12-31"},
			append(slices.Clone(dirs), "docs/big.bin", "hard", "src/main.go"), false},
		{"尺寸", []string{"--size-min", "1000", "--size-max", "4000"},
			append(slices.Clone(dirs), "build/out.o", "link", "pipe"), false},
		{"排除属主", []string{"--exclude-user", uid}, dirs, false},
		{"组合", []string{"--types", "f,d,l", "--exclude-dir", "build", "--exclude-name", `\.log$`, "--size-max", "4000"},
			[]string{"docs", "docs/readme.md", "hard", "link", "logs", "src", "src/main.go", "src/main_test.go"}, false},
	}
	for _, f := range filters {
		for _, compress := range []string{"none", "huffman"} {
			for _, encrypt := range []bool{false, true} {
				name := fmt.Sprintf("TC-CLI-400 %s/%s/encrypt=%v", f.name, compress, encrypt)
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					base := t.TempDir()
					src, repoDir, out, want := filepath.Join(base, "src"), filepath.Join(base, "repo"),
						filepath.Join(base, "out"), filepath.Join(base, "want")
					for _, d := range []string{src, want} {
						if err := os.Mkdir(d, 0o755); err != nil {
							t.Fatal(err)
						}
					}
					testutil.CreateTree(t, src, matrixTree())
					testutil.CreateTree(t, want, prune(matrixTree(), "", f.keep))

					args := append([]string{"--json", "backup", "--source", src, "--repo", repoDir,
						"--name", "m", "--compress", compress}, f.args...)
					var stdin string
					if encrypt {
						args, stdin = append(args, "--encrypt", "aes256", "--password-stdin"), "pw\n"
					}
					res := runJSON(t, stdin, args...)
					files, dirsN := res["files"].(float64), res["dirs"].(float64)
					if res["compressed"] != (compress == "huffman") || res["encrypted"] != encrypt {
						t.Errorf("backup result = %v", res)
					}

					rargs := []string{"--json", "restore", "--backup", "m", "--repo", repoDir, "--target", out}
					if encrypt {
						rargs = append(rargs, "--password-stdin")
					}
					rres := runJSON(t, stdin, rargs...)
					if rres["files"] != files || rres["dirs"] != dirsN {
						t.Errorf("restore %v/%v ≠ backup %v/%v", rres["files"], rres["dirs"], files, dirsN)
					}

					var opts []testutil.Option
					if f.implicitDirs {
						opts = append(opts, testutil.IgnoreTimes())
					}
					testutil.AssertTreeEqual(t, want, out, opts...)
				})
			}
		}
	}
}

// runJSON 以 --json 运行 CLI，要求退出码 0、每行都是 JSON、最后一行为 result，
// 并检查各阶段进度单调、带总量的阶段总量与结果一致、阶段末次进度等于结果。返回 result。
func runJSON(t *testing.T, stdin string, args ...string) map[string]any {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := cli.Run(context.Background(), args, strings.NewReader(stdin), &stdout, &stderr); code != 0 {
		t.Fatalf("%v: exit %d\nstdout: %s\nstderr: %s", args, code, stdout.String(), stderr.String())
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	events := make([]map[string]any, len(lines))
	for i, l := range lines {
		if err := json.Unmarshal([]byte(l), &events[i]); err != nil {
			t.Fatalf("第 %d 行不是 JSON: %q", i+1, l)
		}
	}
	res := events[len(events)-1]
	if res["type"] != "result" {
		t.Fatalf("最后一行不是 result: %v", res)
	}
	last := map[string]map[string]any{}
	for _, ev := range events[:len(events)-1] {
		if ev["type"] != "progress" {
			continue
		}
		ph := ev["phase"].(string)
		if prev := last[ph]; prev != nil && (ev["files"].(float64) < prev["files"].(float64) || ev["bytes"].(float64) < prev["bytes"].(float64)) {
			t.Errorf("%s 阶段进度回退: %v → %v", ph, prev, ev)
		}
		if _, has := ev["totalFiles"]; has != (ph != "scan") {
			t.Errorf("%s 阶段 totalFiles 出现与否不对: %v", ph, ev)
		}
		last[ph] = ev
	}
	for ph, ev := range last {
		if ev["files"] != res["files"] || ev["bytes"] != res["bytes"] {
			t.Errorf("%s 阶段末次进度 %v/%v ≠ 结果 %v/%v", ph, ev["files"], ev["bytes"], res["files"], res["bytes"])
		}
		if ph != "scan" && (ev["totalFiles"] != res["files"] || ev["totalBytes"] != res["bytes"]) {
			t.Errorf("%s 阶段总量 %v/%v ≠ 结果 %v/%v", ph, ev["totalFiles"], ev["totalBytes"], res["files"], res["bytes"])
		}
	}
	if len(last) == 0 {
		t.Errorf("没有进度事件: %v", lines)
	}
	return res
}
