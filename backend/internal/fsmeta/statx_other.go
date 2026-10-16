//go:build linux && !amd64 && !arm64

package fsmeta

// sysStatx 为 0 表示本架构未定义 statx 调用号：statxBtime 视为不支持，
// Btime 一律留零值（目标平台为 amd64/arm64，见 AGENTS.md §3）。
const sysStatx = 0
