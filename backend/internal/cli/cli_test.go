package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/engine"
)

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = Run(context.Background(), args, strings.NewReader(""), &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string // exact match if non-empty
		outHas     string // substring check when wantStdout empty
		errHas     string
	}{
		{name: "help long", args: []string{"--help"}, wantCode: 0, outHas: "databackup - Linux data backup tool"},
		{name: "help short", args: []string{"-h"}, wantCode: 0, outHas: "Usage:"},
		{name: "version", args: []string{"--version"}, wantCode: 0, wantStdout: "databackup 1.0.1\n"},
		{
			name:       "version json",
			args:       []string{"--version", "--json"},
			wantCode:   0,
			wantStdout: `{"type":"result","command":"version","version":"1.0.1"}` + "\n",
		},
		{name: "no args", args: nil, wantCode: 2, errHas: "Usage:"},
		{name: "unknown flag", args: []string{"--nope"}, wantCode: 2, errHas: "Usage:"},
		{name: "unknown command", args: []string{"frobnicate"}, wantCode: 2, errHas: "Usage:"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := run(t, tc.args...)
			if code != tc.wantCode {
				t.Errorf("Run(%v) exit code = %d, want %d", tc.args, code, tc.wantCode)
			}
			if tc.wantStdout != "" && stdout != tc.wantStdout {
				t.Errorf("Run(%v) stdout = %q, want %q", tc.args, stdout, tc.wantStdout)
			}
			if tc.outHas != "" && !strings.Contains(stdout, tc.outHas) {
				t.Errorf("Run(%v) stdout missing %q, got %q", tc.args, tc.outHas, stdout)
			}
			if tc.errHas != "" && !strings.Contains(stderr, tc.errHas) {
				t.Errorf("Run(%v) stderr missing %q, got %q", tc.args, tc.errHas, stderr)
			}
		})
	}
}

func TestHumanReporter(t *testing.T) {
	t.Run("TC-CLI-140 进度原地刷新，换阶段/日志/结束前换行", func(t *testing.T) {
		var buf bytes.Buffer
		c := &cliCtx{stderr: &buf}
		rep := c.reporter()
		rep.Progress(engine.Progress{Phase: engine.PhaseScan, Files: 1, Bytes: 5})
		rep.Progress(engine.Progress{Phase: engine.PhaseScan, Files: 2, Bytes: 9})
		rep.Log(engine.LevelWarn, "跳过 x")
		rep.Progress(engine.Progress{Phase: engine.PhasePack, Files: 2, Bytes: 9, HasTotal: true, TotalFiles: 2, TotalBytes: 9})
		c.endProgress()
		c.endProgress() // 幂等
		want := "\rscan: 1 项, 5 字节\rscan: 2 项, 9 字节\n" +
			"databackup: warn: 跳过 x\n" +
			"\rpack: 2/2 项, 9/9 字节\n"
		if got := buf.String(); got != want {
			t.Errorf("stderr =\n%q\nwant\n%q", got, want)
		}
	})
}

func TestUsageListsPlannedSubcommands(t *testing.T) {
	_, stdout, _ := run(t, "--help")
	for _, sub := range []string{"backup", "restore", "list", "info", "delete", "config"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("usage missing subcommand %q", sub)
		}
	}
}
