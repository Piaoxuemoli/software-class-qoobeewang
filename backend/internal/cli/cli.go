// Package cli implements the databackup command-line interface.
package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/jsonl"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/version"
)

const usage = `databackup - Linux data backup tool

Usage:
  databackup [--json] <command> [options]
  databackup --version [--json]
  databackup --help

Commands:
  backup    create a backup of a directory tree
  restore   restore a directory tree from a backup
  list      list backups stored in a repository
  info      show a backup's container header
  delete    delete a backup
  config    show or change configuration

Options:
  -h, --help       show this help text
      --version    print version
      --json       machine-readable JSON Lines output
`

// cliCtx 携带一次命令执行的上下文：io 流、--json 模式与输出发射器。
type cliCtx struct {
	ctx      context.Context
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
	jsonMode bool
	emit     *jsonl.Emitter
}

// Run executes the CLI with the given arguments (excluding argv[0]) and
// returns the process exit code (docs/architecture/cli-contract.md §4).
// ctx is cancelled on SIGINT/SIGTERM; stdin is read only for --password-stdin.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	c := &cliCtx{ctx: ctx, stdin: stdin, stdout: stdout, stderr: stderr, emit: jsonl.NewEmitter(stdout)}

	// 子命令前的全局参数：--json、--help、--version；第一个非参数词即子命令。
	i := 0
scan:
	for ; i < len(args); i++ {
		switch args[i] {
		case "--json":
			c.jsonMode = true
		case "--help", "-h":
			fmt.Fprint(stdout, usage)
			return 0
		case "--version":
			// 允许 --version 之后跟 --json（契约示例是 --json --version）。
			for _, a := range args[i+1:] {
				if a == "--json" {
					c.jsonMode = true
				}
			}
			return c.printVersion()
		default:
			if strings.HasPrefix(args[i], "-") {
				return c.fail(fmt.Errorf("未知参数 %q: %w", args[i], ErrUsage), usage)
			}
			break scan
		}
	}
	if i >= len(args) {
		return c.fail(fmt.Errorf("缺少子命令: %w", ErrUsage), usage)
	}
	cmd, ok := commands[args[i]]
	if !ok {
		return c.fail(fmt.Errorf("未知子命令 %q: %w", args[i], ErrUsage), usage)
	}
	return cmd.run(c, args[i+1:])
}

// printVersion 输出 --version 结果。
func (c *cliCtx) printVersion() int {
	if c.jsonMode {
		if err := c.emit.Emit(jsonl.VersionResult{
			Type:    jsonl.TypeResult,
			Command: jsonl.CommandVersion,
			Version: version.Version,
		}); err != nil {
			return c.fail(fmt.Errorf("输出结果: %w", err), "")
		}
	} else {
		fmt.Fprintf(c.stdout, "databackup %s\n", version.Version)
	}
	return 0
}

// fail 按契约输出错误并返回退出码：--json 模式 stdout 最后一行为 error 事件；
// 否则错误写 stderr，用法错误附 usageText。
func (c *cliCtx) fail(err error, usageText string) int {
	code := Classify(err)
	msg := message(err)
	if c.jsonMode {
		if e := c.emit.Emit(jsonl.Error{Type: jsonl.TypeError, Code: code, Message: msg}); e != nil {
			fmt.Fprintf(c.stderr, "databackup: %v\n", e)
		}
	} else {
		fmt.Fprintf(c.stderr, "databackup: %s\n", msg)
	}
	if usageText != "" {
		fmt.Fprint(c.stderr, usageText)
	}
	return ExitCode(code)
}

// ok 输出成功结果：--json 模式为最后一行的 result 事件；否则为人类可读文本。
func (c *cliCtx) ok(result any, human string) int {
	if c.jsonMode {
		if err := c.emit.Emit(result); err != nil {
			return c.fail(fmt.Errorf("输出结果: %w", err), "")
		}
	} else {
		fmt.Fprint(c.stdout, human)
	}
	return 0
}

// commands 是子命令表。
var commands = map[string]*command{
	"backup":  cmdBackup,
	"restore": cmdRestore,
	"list":    cmdList,
	"info":    cmdInfo,
	"delete":  cmdDelete,
	"config":  cmdConfig,
}
