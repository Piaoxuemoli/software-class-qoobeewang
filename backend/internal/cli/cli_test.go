package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
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
		{name: "version", args: []string{"--version"}, wantCode: 0, wantStdout: "databackup 0.1.0\n"},
		{
			name:       "version json",
			args:       []string{"--version", "--json"},
			wantCode:   0,
			wantStdout: `{"type":"result","command":"version","version":"0.1.0"}` + "\n",
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

func TestUsageListsPlannedSubcommands(t *testing.T) {
	_, stdout, _ := run(t, "--help")
	for _, sub := range []string{"backup", "restore", "list", "info", "delete", "config"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("usage missing planned subcommand %q", sub)
		}
	}
	if !strings.Contains(stdout, "(planned)") {
		t.Error("usage should mark subcommands as planned")
	}
}
