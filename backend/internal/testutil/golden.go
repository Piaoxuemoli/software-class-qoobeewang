package testutil

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update 是 "go test ./internal/<包> -update"：显式用实际输出覆盖黄金文件。
// 引入 testutil 的测试包不得再自行定义同名 flag。
var update = flag.Bool("update", false, "用实际输出覆盖 golden 文件")

// goldenWindow 是不一致时打印的差异附近字节数。
const goldenWindow = 16

// Golden 把 got 与黄金文件 path（通常是 testdata/*.golden）逐字节比较；
// 带 -update 运行时改为把 got 写入 path。
func Golden(t testing.TB, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("Golden: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("Golden: %v", err)
		}
		t.Logf("已更新 golden 文件 %s（%d 字节）", path, len(got))
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Golden: 读取 %s: %v（首次生成请加 -update）", path, err)
	}
	if bytes.Equal(want, got) {
		return
	}
	off := 0
	for off < len(want) && off < len(got) && want[off] == got[off] {
		off++
	}
	t.Errorf("与 golden 文件 %s 不一致：期望 %d 字节，实际 %d 字节，首个差异在偏移 %d\n期望: % x\n实际: % x",
		path, len(want), len(got), off, window(want, off), window(got, off))
}

func window(b []byte, off int) []byte {
	return b[off:min(len(b), off+goldenWindow)]
}
