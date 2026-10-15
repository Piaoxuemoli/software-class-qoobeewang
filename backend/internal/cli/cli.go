// Package cli implements the databackup command-line interface.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/jsonl"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/version"
)

const usage = `databackup - Linux data backup tool

Usage:
  databackup <command> [options]

Commands:
  backup    create a backup of a directory tree        (planned)
  restore   restore a directory tree from a backup     (planned)
  list      list backups stored in a repository        (planned)
  info      show a backup's container header           (planned)
  delete    delete a backup                            (planned)
  config    show or change configuration               (planned)

Options:
  -h, --help       show this help text
      --version    print version
      --json       machine-readable JSON Lines output (with --version)
`

// Run executes the CLI with the given arguments (excluding argv[0]) and
// returns the process exit code (docs/architecture/cli-contract.md §4).
// ctx is cancelled on SIGINT/SIGTERM; stdin is read only for --password-stdin.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("databackup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }

	help := fs.Bool("help", false, "show help")
	fs.BoolVar(help, "h", false, "show help")
	showVersion := fs.Bool("version", false, "print version")
	asJSON := fs.Bool("json", false, "JSON output")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *help {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if *showVersion {
		if *asJSON {
			e := jsonl.NewEmitter(stdout)
			if err := e.Emit(jsonl.VersionResult{
				Type:    jsonl.TypeResult,
				Command: jsonl.CommandVersion,
				Version: version.Version,
			}); err != nil {
				fmt.Fprintf(stderr, "databackup: %v\n", err)
				return 1
			}
		} else {
			fmt.Fprintf(stdout, "databackup %s\n", version.Version)
		}
		return 0
	}
	fmt.Fprint(stderr, usage)
	return 2
}
