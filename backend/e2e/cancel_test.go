//go:build e2e && linux

package e2e

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// event 是 JSONL 事件中本测试关心的字段。
type event struct {
	Type    string `json:"type"`
	Phase   string `json:"phase"`
	Bytes   int64  `json:"bytes"`
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// runUntil 以 --json 启动 bin，读取 stdout 事件直到 trigger 返回 true，随即发送 SIGTERM，
// 再读完剩余事件。返回全部事件、退出码，以及发出信号前进程是否已结束。
func runUntil(t *testing.T, bin, stdin string, trigger func(event) bool, args ...string) ([]event, int) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"--json"}, args...)...)
	cmd.Stdin = strings.NewReader(stdin)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var events []event
	signalled := false
	sc := bufio.NewScanner(out)
	for sc.Scan() {
		var ev event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("stdout 不是 JSONL: %q: %v", sc.Text(), err)
		}
		events = append(events, ev)
		if !signalled && trigger(ev) {
			if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatalf("SIGTERM: %v", err)
			}
			signalled = true
		}
	}
	_, _ = io.Copy(io.Discard, out)
	err = cmd.Wait()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	if !signalled {
		t.Fatalf("进程在触发条件出现前结束（退出码 %d）: %+v\nstderr: %s", code, events, stderr.String())
	}
	return events, code
}

// sparse 创建 size 字节的稀疏文件：读取很快，但足以让备份/还原持续到信号到达。
func sparse(t *testing.T, path string, size int64) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCancel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in -short mode")
	}
	bin := buildBinary(t)
	base := t.TempDir()
	src, repo := filepath.Join(base, "src"), filepath.Join(base, "repo")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	sparse(t, filepath.Join(src, "big.img"), 256<<20)

	t.Run("TC-E2E-101 备份收到 SIGTERM：退出码 4、E_CANCELLED、无 .partial 残留", func(t *testing.T) {
		start := time.Now()
		events, code := runUntil(t, bin, "pw\n", func(ev event) bool {
			return ev.Type == "progress" && ev.Phase == "pack" && ev.Bytes > 0
		}, "backup", "--source", src, "--repo", repo, "--name", "c",
			"--compress", "huffman", "--encrypt", "aes256", "--password-stdin")
		last := events[len(events)-1]
		if code != 4 || last.Type != "error" || last.Code != "E_CANCELLED" {
			t.Errorf("exit %d, last event %+v", code, last)
		}
		entries, _ := os.ReadDir(repo)
		for _, e := range entries {
			t.Errorf("取消后仓库残留 %s", e.Name())
		}
		t.Logf("取消用时 %v", time.Since(start))
	})

	t.Run("TC-E2E-102 还原收到 SIGTERM：先发目标不完整 warn，再 E_CANCELLED", func(t *testing.T) {
		out, _, code := run(t, bin, "backup", "--source", src, "--repo", repo, "--name", "full")
		if code != 0 {
			t.Fatalf("准备备份失败: %d %s", code, out)
		}
		target := filepath.Join(base, "restored")
		events, code := runUntil(t, bin, "", func(ev event) bool {
			return ev.Type == "progress" && ev.Phase == "restore" && ev.Bytes > 0
		}, "restore", "--backup", "full", "--repo", repo, "--target", target)
		if code != 4 {
			t.Errorf("exit %d, want 4", code)
		}
		n := len(events)
		if n < 2 || events[n-1].Code != "E_CANCELLED" || events[n-2].Level != "warn" ||
			!strings.Contains(events[n-2].Message, "目标目录内容不完整") {
			t.Errorf("末尾事件 = %+v", events[max(0, n-2):])
		}
	})
}
