//go:build !linux

package main

import "github.com/rogpeppe/go-internal/testscript"

// scriptCmds 在非 Linux 平台为空：testutil 的目录树比较器只在 Linux 上可用。
var scriptCmds map[string]func(ts *testscript.TestScript, neg bool, args []string)
