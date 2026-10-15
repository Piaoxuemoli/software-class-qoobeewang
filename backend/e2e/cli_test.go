//go:build e2e

// Package e2e exercises the compiled databackup binary end to end.
package e2e

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "databackup")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/databackup")
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func run(t *testing.T, bin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code = 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			t.Fatalf("exec %v: %v", args, err)
		}
	}
	return outBuf.String(), errBuf.String(), code
}

func TestCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in -short mode")
	}
	bin := buildBinary(t)
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("binary missing: %v", err)
	}

	t.Run("version", func(t *testing.T) {
		out, _, code := run(t, bin, "--version")
		if code != 0 || out != "databackup 0.1.0\n" {
			t.Errorf("--version: code=%d out=%q", code, out)
		}
	})

	t.Run("version json", func(t *testing.T) {
		out, _, code := run(t, bin, "--version", "--json")
		want := `{"type":"result","command":"version","version":"0.1.0"}` + "\n"
		if code != 0 || out != want {
			t.Errorf("--version --json: code=%d out=%q", code, out)
		}
	})

	t.Run("help", func(t *testing.T) {
		out, _, code := run(t, bin, "--help")
		if code != 0 || !strings.Contains(out, "Usage:") {
			t.Errorf("--help: code=%d out=%q", code, out)
		}
	})

	t.Run("bad arg exits 2", func(t *testing.T) {
		_, stderr, code := run(t, bin, "--bogus")
		if code != 2 {
			t.Errorf("--bogus: code=%d, want 2", code)
		}
		if !strings.Contains(stderr, "Usage:") {
			t.Errorf("--bogus: stderr missing usage, got %q", stderr)
		}
	})
}
