package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/config"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/container"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/engine"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/jsonl"
	"github.com/Piaoxuemoli/software-class-qoobeewang/backend/internal/repo"
)

// command 描述一个子命令。
type command struct {
	usage string
	run   func(c *cliCtx, args []string) int
}

// 子命令共用的参数定义。
var (
	commonBool = map[string]bool{"json": true, "help": true}
	filterArgs = map[string]bool{
		"types": true, "mtime-from": true, "mtime-to": true,
		"ctime-from": true, "ctime-to": true, "size-min": true, "size-max": true,
	}
	filterRepeatable = map[string]bool{
		"exclude-dir": true, "exclude-name": true,
		"exclude-user": true, "exclude-group": true,
	}
)

func mergeFlags(ms ...map[string]bool) map[string]bool {
	out := make(map[string]bool)
	for _, m := range ms {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// parse 解析子命令参数：先扫描 --json（解析失败时也按 JSON 输出错误），
// --help 打印用法。返回的 exitCode 只在 p 为 nil 时使用（help → 0；解析失败 → fail 的退出码）。
func parse(c *cliCtx, args []string, usage string, valueFlags, repeatable, boolFlags map[string]bool) (*parsedArgs, int) {
	for _, a := range args {
		if a == "--" {
			break
		}
		if a == "--json" {
			c.jsonMode = true
			break
		}
	}
	p, err := parseArgs(args, valueFlags, repeatable, mergeFlags(commonBool, boolFlags))
	if err != nil {
		return nil, c.fail(err, usage)
	}
	if p.flag("help") {
		fmt.Fprint(c.stdout, usage)
		return nil, 0
	}
	return p, -1
}

// repoFlag 读取可选的 --repo 单值参数。
func repoFlag(c *cliCtx, p *parsedArgs, usage string) (string, int) {
	v, err := p.get("repo")
	if err != nil {
		return "", c.fail(err, usage)
	}
	return v, -1
}

// repoDir 按契约顺序确定仓库目录：--repo → 配置文件 → ~/DataBackup。
func (c *cliCtx) repoDir(flagVal string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	path, err := config.DefaultPath()
	if err != nil {
		return "", err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return "", err
	}
	return cfg.Repo, nil
}

// ---- backup ---------------------------------------------------------------

const backupUsage = `Usage:
  databackup [--json] backup --source DIR [--repo DIR] [--name NAME]
             [filters...] [--compress none|huffman]
             [--encrypt none|aes256 --password-stdin] [--dry-run]

Options:
  --source DIR          source directory (required)
  --repo DIR            backup repository directory
  --name NAME           backup name (default: <srcdir>-<YYYYMMDD-HHMMSS>)
  --types LIST          only these file types: f d l c b s p
  --exclude-dir PATH    exclude directory (repeatable)
  --exclude-name REGEX  exclude file names matching RE2 regex (repeatable)
  --mtime-from T        modification time lower bound (YYYY-MM-DD[THH:MM:SS])
  --mtime-to T          modification time upper bound
  --ctime-from T        creation time lower bound
  --ctime-to T          creation time upper bound
  --size-min N          minimum file size in bytes (regular files only)
  --size-max N          maximum file size in bytes
  --exclude-user NAME   exclude owner user (repeatable)
  --exclude-group NAME  exclude owner group (repeatable)
  --compress ALG        none (default) | huffman
  --encrypt ALG         none (default) | aes256 (requires --password-stdin)
  --password-stdin      read password from stdin
  --dry-run             scan only; report counts without writing
  -h, --help            show this help
`

var cmdBackup = &command{
	usage: backupUsage,
	run: func(c *cliCtx, args []string) int {
		p, exit := parse(c, args, backupUsage,
			mergeFlags(filterArgs, map[string]bool{
				"source": true, "repo": true, "name": true,
				"compress": true, "encrypt": true,
			}),
			filterRepeatable,
			map[string]bool{"dry-run": true, "password-stdin": true},
		)
		if p == nil {
			return exit
		}
		return runBackup(c, p)
	},
}

func runBackup(c *cliCtx, p *parsedArgs) int {
	fail := func(err error) int { return c.fail(err, backupUsage) }
	if err := p.noPositionals(); err != nil {
		return fail(err)
	}
	source, err := p.require("source")
	if err != nil {
		return fail(err)
	}
	name, err := p.get("name")
	if err != nil {
		return fail(err)
	}
	// 名称规则只在 repo.ValidateName 一处实现；先行校验以便在读口令、扫描之前报 E_USAGE。
	// 显式给出的空名（--name ""）同样非法，不退化为缺省名。
	if _, given := p.values["name"]; given {
		if err := repo.ValidateName(name); err != nil {
			return fail(err)
		}
	}
	criteria, err := buildCriteria(p)
	if err != nil {
		return fail(err)
	}
	compress, err := p.get("compress")
	if err != nil {
		return fail(err)
	}
	var compression container.Compression
	switch compress {
	case "", "none":
		compression = container.CompressionNone
	case "huffman":
		compression = container.CompressionHuffman
	default:
		return fail(fmt.Errorf("--compress 只支持 none|huffman: %w", ErrUsage))
	}
	encrypt, err := p.get("encrypt")
	if err != nil {
		return fail(err)
	}
	var encrypted bool
	switch encrypt {
	case "", "none":
	case "aes256":
		encrypted = true
	default:
		return fail(fmt.Errorf("--encrypt 只支持 none|aes256: %w", ErrUsage))
	}
	dryRun := p.flag("dry-run")
	// --encrypt aes256 与 --password-stdin 必须配套；--dry-run 时加密参数被接受但忽略，
	// 不需要也不读取口令。
	if !dryRun && encrypted != p.flag("password-stdin") {
		return fail(fmt.Errorf("--encrypt aes256 必须与 --password-stdin 配套: %w", ErrUsage))
	}

	repoFlagVal, exit := repoFlag(c, p, backupUsage)
	if exit >= 0 {
		return exit
	}
	repoDir, err := c.repoDir(repoFlagVal)
	if err != nil {
		return fail(err)
	}
	var password []byte
	if !dryRun && p.flag("password-stdin") {
		if password, err = readPassword(c.stdin); err != nil {
			return fail(err)
		}
		defer clear(password)
	}
	opts := engine.BackupOptions{
		Source:      source,
		Repo:        repoDir,
		Name:        name,
		Criteria:    criteria,
		Compression: compression,
		Password:    password,
	}
	rep := c.reporter()
	if dryRun {
		res, err := engine.DryRun(c.ctx, opts, rep)
		if err != nil {
			return c.fail(err, "")
		}
		return c.ok(jsonl.DryRunResult{
			Type: jsonl.TypeResult, Command: jsonl.CommandDryRun,
			Files: res.Files, Dirs: res.Dirs, Bytes: res.Bytes, Skipped: res.Skipped,
		}, fmt.Sprintf("将备份 %d 个文件、%d 个目录，共 %d 字节（跳过 %d 项）\n",
			res.Files, res.Dirs, res.Bytes, res.Skipped))
	}
	res, err := engine.Backup(c.ctx, opts, rep)
	if err != nil {
		return c.fail(err, "")
	}
	return c.ok(jsonl.BackupResult{
		Type: jsonl.TypeResult, Command: jsonl.CommandBackup,
		Name: res.Name, Path: res.Path,
		Files: res.Files, Dirs: res.Dirs, Bytes: res.Bytes, OutputBytes: res.OutputBytes,
		Compressed: res.Compressed, Encrypted: res.Encrypted,
		Skipped: res.Skipped, Warnings: res.Warnings, MS: res.Duration.Milliseconds(),
	}, fmt.Sprintf("备份完成: %s（%d 个文件，%d → %d 字节，%dms）\n",
		res.Path, res.Files, res.Bytes, res.OutputBytes, res.Duration.Milliseconds()))
}

// ---- restore --------------------------------------------------------------

const restoreUsage = `Usage:
  databackup [--json] restore --backup NAME|PATH --target DIR [--repo DIR] [--password-stdin]

Options:
  --backup NAME|PATH  backup name in repo or .dbk file path (required)
  --target DIR        restore target directory: must not exist or be empty (required)
  --repo DIR          backup repository directory
  --password-stdin    read password from stdin (required for encrypted backups)
  -h, --help          show this help
`

var cmdRestore = &command{
	usage: restoreUsage,
	run: func(c *cliCtx, args []string) int {
		p, exit := parse(c, args, restoreUsage,
			map[string]bool{"backup": true, "target": true, "repo": true}, nil,
			map[string]bool{"password-stdin": true})
		if p == nil {
			return exit
		}
		fail := func(err error) int { return c.fail(err, restoreUsage) }
		if err := p.noPositionals(); err != nil {
			return fail(err)
		}
		backup, err := p.require("backup")
		if err != nil {
			return fail(err)
		}
		target, err := p.require("target")
		if err != nil {
			return fail(err)
		}
		repoFlagVal, exit := repoFlag(c, p, restoreUsage)
		if exit >= 0 {
			return exit
		}
		repoDir, err := c.repoDir(repoFlagVal)
		if err != nil {
			return fail(err)
		}
		var password []byte
		if p.flag("password-stdin") {
			if password, err = readPassword(c.stdin); err != nil {
				return fail(err)
			}
			defer clear(password)
		}
		r, err := repo.New(repoDir)
		if err != nil {
			return c.fail(err, "")
		}
		path, err := r.Resolve(backup)
		if err != nil {
			return c.fail(err, "")
		}
		res, err := engine.Restore(c.ctx, engine.RestoreOptions{
			Backup: path, Target: target, Password: password,
		}, c.reporter())
		if err != nil {
			return c.fail(err, "")
		}
		return c.ok(jsonl.RestoreResult{
			Type: jsonl.TypeResult, Command: jsonl.CommandRestore,
			Target: res.Target, Files: res.Files, Dirs: res.Dirs, Bytes: res.Bytes,
			Warnings: res.Warnings, MS: res.Duration.Milliseconds(),
		}, fmt.Sprintf("还原完成: %s（%d 个文件，%d 字节，%dms）\n",
			res.Target, res.Files, res.Bytes, res.Duration.Milliseconds()))
	},
}

// ---- list -----------------------------------------------------------------

const listUsage = `Usage:
  databackup [--json] list [--repo DIR]

Options:
  --repo DIR  backup repository directory
  -h, --help  show this help
`

var cmdList = &command{
	usage: listUsage,
	run: func(c *cliCtx, args []string) int {
		p, exit := parse(c, args, listUsage, map[string]bool{"repo": true}, nil, nil)
		if p == nil {
			return exit
		}
		if err := p.noPositionals(); err != nil {
			return c.fail(err, listUsage)
		}
		repoFlagVal, exit := repoFlag(c, p, listUsage)
		if exit >= 0 {
			return exit
		}
		repoDir, err := c.repoDir(repoFlagVal)
		if err != nil {
			return c.fail(err, listUsage)
		}
		r, err := repo.New(repoDir)
		if err != nil {
			return c.fail(err, "")
		}
		backups, err := r.List()
		if err != nil {
			return c.fail(err, "")
		}
		abs, err := filepath.Abs(repoDir)
		if err != nil {
			abs = repoDir
		}
		infos := make([]jsonl.BackupInfo, len(backups))
		for i, b := range backups {
			infos[i] = backupInfoOf(b)
		}
		var human strings.Builder
		fmt.Fprintf(&human, "%-40s %-12s %-20s %s\n", "NAME", "SIZE", "CREATED", "FLAGS")
		for _, b := range infos {
			var flags []string
			if b.Compressed {
				flags = append(flags, "compressed")
			}
			if b.Encrypted {
				flags = append(flags, "encrypted")
			}
			if b.Corrupt {
				flags = append(flags, "corrupt")
			}
			fmt.Fprintf(&human, "%-40s %-12d %-20s %s\n",
				b.Name, b.OutputBytes, b.CreatedAt, strings.Join(flags, " "))
		}
		return c.ok(jsonl.ListResult{
			Type: jsonl.TypeResult, Command: jsonl.CommandList,
			Repo: abs, Backups: infos,
		}, human.String())
	},
}

// backupInfoOf 把 repo.Backup 映射为线上的 BackupInfo。
func backupInfoOf(b repo.Backup) jsonl.BackupInfo {
	info := jsonl.BackupInfo{
		Name: b.Name, Path: b.Path, Corrupt: b.Corrupt,
	}
	created := b.ModTime
	// OutputBytes 对损坏项同样有意义：repo 在解析失败时仍填入文件 stat 大小。
	info.OutputBytes = b.Info.Size
	if !b.Corrupt {
		created = b.Info.Header.CreatedAt
		info.Source = b.Info.Header.Source
		info.Compressed = b.Info.Header.Compression != container.CompressionNone
		info.Encrypted = b.Info.Header.Encryption != container.EncryptionNone
		info.Files = b.Info.Summary.Files
		info.Dirs = b.Info.Summary.Dirs
		info.Bytes = b.Info.Summary.Bytes
		info.FormatVersion = int(b.Info.Header.Version)
	}
	info.CreatedAt = created.Local().Format("2006-01-02T15:04:05-07:00")
	return info
}

// ---- info -----------------------------------------------------------------

const infoUsage = `Usage:
  databackup [--json] info NAME|PATH [--repo DIR]

Arguments:
  NAME|PATH  backup name in repo or .dbk file path

Options:
  --repo DIR  backup repository directory
  -h, --help  show this help
`

var cmdInfo = &command{
	usage: infoUsage,
	run: func(c *cliCtx, args []string) int {
		p, exit := parse(c, args, infoUsage, map[string]bool{"repo": true}, nil, nil)
		if p == nil {
			return exit
		}
		name, err := p.onePositional("备份名或路径")
		if err != nil {
			return c.fail(err, infoUsage)
		}
		repoFlagVal, exit := repoFlag(c, p, infoUsage)
		if exit >= 0 {
			return exit
		}
		repoDir, err := c.repoDir(repoFlagVal)
		if err != nil {
			return c.fail(err, infoUsage)
		}
		r, err := repo.New(repoDir)
		if err != nil {
			return c.fail(err, "")
		}
		b, err := r.Info(name)
		if err != nil {
			return c.fail(err, "")
		}
		info := backupInfoOf(b)
		return c.ok(jsonl.InfoResult{
			Type: jsonl.TypeResult, Command: jsonl.CommandInfo, BackupInfo: info,
		}, fmt.Sprintf("name: %s\npath: %s\nsize: %d\ncreated: %s\nsource: %s\nfiles: %d  dirs: %d  bytes: %d\ncompressed: %v  encrypted: %v\n",
			info.Name, info.Path, info.OutputBytes, info.CreatedAt, info.Source,
			info.Files, info.Dirs, info.Bytes, info.Compressed, info.Encrypted))
	},
}

// ---- delete ---------------------------------------------------------------

const deleteUsage = `Usage:
  databackup [--json] delete NAME|PATH [--repo DIR]

Arguments:
  NAME|PATH  backup name in repo or .dbk file path

Options:
  --repo DIR  backup repository directory
  -h, --help  show this help
`

var cmdDelete = &command{
	usage: deleteUsage,
	run: func(c *cliCtx, args []string) int {
		p, exit := parse(c, args, deleteUsage, map[string]bool{"repo": true}, nil, nil)
		if p == nil {
			return exit
		}
		name, err := p.onePositional("备份名或路径")
		if err != nil {
			return c.fail(err, deleteUsage)
		}
		// 只删除 .dbk 文件：显式路径必须以 .dbk 结尾。
		if strings.Contains(name, "/") && !strings.HasSuffix(name, ".dbk") {
			return c.fail(fmt.Errorf("只能删除 .dbk 文件: %w", ErrUsage), deleteUsage)
		}
		repoFlagVal, exit := repoFlag(c, p, deleteUsage)
		if exit >= 0 {
			return exit
		}
		repoDir, err := c.repoDir(repoFlagVal)
		if err != nil {
			return c.fail(err, deleteUsage)
		}
		r, err := repo.New(repoDir)
		if err != nil {
			return c.fail(err, "")
		}
		b, err := r.Delete(name)
		if err != nil {
			return c.fail(err, "")
		}
		return c.ok(jsonl.DeleteResult{
			Type: jsonl.TypeResult, Command: jsonl.CommandDelete,
			Name: b.Name, Path: b.Path,
		}, fmt.Sprintf("已删除: %s\n", b.Path))
	},
}

// ---- config ---------------------------------------------------------------

const configUsage = `Usage:
  databackup [--json] config get [KEY]
  databackup [--json] config set KEY VALUE

Arguments:
  KEY   configuration key (currently only "repo")
  VALUE new value; "repo" must be an absolute path

Options:
  -h, --help  show this help
`

var cmdConfig = &command{
	usage: configUsage,
	run: func(c *cliCtx, args []string) int {
		p, exit := parse(c, args, configUsage, nil, nil, nil)
		if p == nil {
			return exit
		}
		fail := func(err error) int { return c.fail(err, configUsage) }
		if len(p.args) == 0 {
			return fail(fmt.Errorf("缺少 get|set 子动作: %w", ErrUsage))
		}
		action, rest := p.args[0], p.args[1:]
		// 先完成全部参数级校验（E_USAGE），再触达 config 包：
		// DefaultPath/Load 的错误属运行期错误。
		var key, value string
		switch action {
		case "get":
			if len(rest) > 1 {
				return fail(fmt.Errorf("多余的位置参数 %q: %w", strings.Join(rest[1:], " "), ErrUsage))
			}
			if len(rest) == 1 {
				key = rest[0]
				if !configKeyKnown(key) {
					return fail(fmt.Errorf("未知配置项 %q: %w", key, ErrUsage))
				}
			}
		case "set":
			if len(rest) == 0 {
				return fail(fmt.Errorf("缺少 KEY 与 VALUE: %w", ErrUsage))
			}
			if len(rest) == 1 {
				return fail(fmt.Errorf("缺少 VALUE: %w", ErrUsage))
			}
			if len(rest) > 2 {
				return fail(fmt.Errorf("多余的位置参数 %q: %w", strings.Join(rest[2:], " "), ErrUsage))
			}
			key, value = rest[0], rest[1]
			if !configKeyKnown(key) {
				return fail(fmt.Errorf("未知配置项 %q: %w", key, ErrUsage))
			}
			// repo 必须是绝对路径，先行校验以便尽早报 E_USAGE；config.Set 会再校验。
			if key == "repo" && !filepath.IsAbs(value) {
				return fail(fmt.Errorf("repo 必须是绝对路径: %w", ErrUsage))
			}
		default:
			return fail(fmt.Errorf("未知子动作 %q（应为 get|set）: %w", action, ErrUsage))
		}
		confPath, err := config.DefaultPath()
		if err != nil {
			return c.fail(err, "")
		}
		cfg, err := config.Load(confPath)
		if err != nil && action == "set" && errors.Is(err, config.ErrInvalid) {
			// 现有配置损坏或已失效时 set 必须仍然可用，否则用户无法修复它。
			cfg, err = config.Default()
		}
		if err != nil {
			return c.fail(err, "")
		}
		if action == "set" {
			cfg, err = cfg.Set(key, value)
			if err != nil {
				return fail(err)
			}
			if err := config.Save(confPath, cfg); err != nil {
				return c.fail(err, "")
			}
		}
		human := "repo = " + cfg.Repo + "\n"
		if action == "get" && key != "" {
			human = cfg.Repo + "\n"
		}
		return c.ok(jsonl.ConfigResult{
			Type: jsonl.TypeResult, Command: jsonl.CommandConfig, Repo: cfg.Repo,
		}, human)
	},
}

func configKeyKnown(key string) bool {
	for _, k := range config.Keys {
		if k == key {
			return true
		}
	}
	return false
}
