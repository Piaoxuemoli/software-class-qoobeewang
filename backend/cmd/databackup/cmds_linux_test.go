//go:build linux

package main

import (
	"strings"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/testutil"
	"github.com/rogpeppe/go-internal/testscript"
)

// scriptCmds 是 txtar 中可用的自定义命令：
//   - treeeq [-ignore-owner] WANT GOT：按正确性五要素（testutil.DiffTrees）比较两棵目录树；
//     加 ! 前缀时断言二者不同。
var scriptCmds = map[string]func(ts *testscript.TestScript, neg bool, args []string){
	"treeeq": func(ts *testscript.TestScript, neg bool, args []string) {
		var opts []testutil.Option
		if len(args) > 0 && args[0] == "-ignore-owner" {
			opts, args = append(opts, testutil.IgnoreOwner()), args[1:]
		}
		if len(args) != 2 {
			ts.Fatalf("usage: treeeq [-ignore-owner] WANT GOT")
		}
		diffs, err := testutil.DiffTrees(ts.MkAbs(args[0]), ts.MkAbs(args[1]), opts...)
		if err != nil {
			ts.Fatalf("treeeq: %v", err)
		}
		switch {
		case neg && len(diffs) == 0:
			ts.Fatalf("treeeq: %s 与 %s 一致", args[0], args[1])
		case !neg && len(diffs) > 0:
			ts.Fatalf("treeeq: %d 处差异:\n%s", len(diffs), strings.Join(diffs, "\n"))
		}
	},
}
