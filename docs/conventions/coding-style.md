# 编码规范

源码质量占完成分 10 分。原则：读者是不懂 Go 的作者本人和答辩老师——清晰优先于技巧。

## 1. Go 后端

- 格式：`gofmt`（`make format`），不手动对齐。
- 静态检查：`go vet` + `staticcheck` 零告警；`scripts/check-imports.sh` 通过（禁用 import 名单见 `AGENTS.md` §3）。
- 依赖：只用标准库；`go.mod` 不得出现 `require`。
- 命名遵循 Effective Go：包名小写单词、无下划线；导出标识符驼峰；缩写保持大小写一致（`ID`、`URL`、`AES`）。
- 注释：每个导出标识符都有文档注释，以标识符名开头，可用中文；复杂算法（Huffman 建树、AES 轮函数、头记录布局）在实现处说明原理并标注参考标准。
- 错误处理：
  - 函数返回 `error`，不用 panic 表达业务错误（仅在不可能分支用 panic）。
  - 包装错误带上下文：`fmt.Errorf("打开源目录 %s: %w", path, err)`。
  - 面向用户的错误类别用包级哨兵错误（如 `engine.ErrTargetNotEmpty`、`container.ErrAuth`），`cli` 负责映射为退出码与 `error.code`。
- 资源：打开的文件用 `defer` 关闭，写入路径要检查 `Close()` 的错误。
- I/O：大数据一律流式（`io.Reader`/`io.Writer`），不要 `os.ReadFile` 整个用户文件。
- 平台：Linux 专属代码放 `*_linux.go` 或加 `//go:build linux`。
- 函数保持短小（建议 ≤60 行）；一个文件聚焦一个概念。
- 常量化魔数：头记录字段长度、块大小、魔数等定义为具名常量并注释来源。

## 2. TypeScript 客户端

- `strict` 模式；禁止 `any`（确需时用 `unknown` 再收窄）。
- ESLint + Prettier 零告警（`make client-lint`）。
- 渲染进程不得直接访问 Node API；一切系统能力经 preload 暴露的 `window.databackup`。
- 主进程与渲染进程共享的类型放 `src/shared/`，与 `docs/architecture/cli-contract.md` 一一对应。
- 组件使用函数组件 + Hooks；业务状态与界面分离，便于单测。
- 用户可见文案为中文。

## 3. 脚本与配置

- Shell 脚本以 `#!/usr/bin/env bash` + `set -euo pipefail` 开头，通过 `bash script.sh` 调用。
- 所有文本文件 UTF-8、LF 换行（`.gitattributes` 强制）。

## 4. 参考

- Effective Go、Go Code Review Comments
- 课程推荐的《华为软件编程规范》（本地资料）中与语言无关的部分：命名清晰、函数单一职责、注释说明意图、防御性检查输入。
