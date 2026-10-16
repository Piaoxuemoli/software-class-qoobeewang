package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/cli"
	"github.com/rogpeppe/go-internal/testscript"
)

// TestMain 注册 testscript 可调用的命令：
//   - databackup：真实 CLI 入口（与 main.go 相同，含 SIGINT/SIGTERM 处理）。
//   - expect CODE[,CODE...] ARGS…：以 databackup 语义运行 ARGS，实际退出码
//     不在 CODE 列表中时报错；用于断言精确退出码。
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"databackup": func() {
			os.Exit(run())
		},
		"expect": func() {
			if len(os.Args) < 2 {
				fmt.Fprintln(os.Stderr, "expect: usage: expect CODE[,CODE...] ARGS...")
				os.Exit(2)
			}
			var codes []int
			for _, s := range strings.Split(os.Args[1], ",") {
				c, err := strconv.Atoi(s)
				if err != nil {
					fmt.Fprintf(os.Stderr, "expect: bad exit code %q\n", s)
					os.Exit(2)
				}
				codes = append(codes, c)
			}
			got := cli.Run(context.Background(), os.Args[2:], os.Stdin, os.Stdout, os.Stderr)
			for _, c := range codes {
				if got == c {
					return
				}
			}
			fmt.Fprintf(os.Stderr, "expect: args %v: want exit in %v, got %d\n", os.Args[2:], codes, got)
			os.Exit(1)
		},
	})
}

// TestScripts 运行 testdata/script 下的全部 txtar 用例（TC-CLI-2xx 参数、TC-CLI-3xx 全链路）。
func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:  "testdata/script",
		Cmds: scriptCmds,
		Setup: func(env *testscript.Env) error {
			// 隔离配置与默认仓库：testscript 默认 HOME=/no-home。
			env.Setenv("HOME", env.WorkDir+"/home")
			env.Setenv("XDG_CONFIG_HOME", env.WorkDir+"/xdg")
			return nil
		},
	})
}
