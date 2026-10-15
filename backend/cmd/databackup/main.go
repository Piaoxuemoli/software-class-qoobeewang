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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
