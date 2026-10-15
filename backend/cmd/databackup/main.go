// Command databackup is the CLI entry point of the DataBackup backend.
package main

import (
	"os"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
