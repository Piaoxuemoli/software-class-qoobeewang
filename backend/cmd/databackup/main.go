// Command databackup is the CLI entry point of the DataBackup backend.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/cli"
)

func main() {
	os.Exit(run())
}

// run 执行 CLI：SIGINT/SIGTERM 取消 ctx，由 engine 清理后以退出码 4 结束（契约 §3.5）。
func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cli.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
}
