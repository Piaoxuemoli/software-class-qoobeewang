// Package config 读写用户配置（FR-15），存于 $XDG_CONFIG_HOME/databackup/config.json
// （未设置时为 ~/.config/databackup/config.json）。
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrInvalid 表示配置值非法（如仓库路径不是绝对路径、已存在但不是目录）或 KEY 未知。
// cli 映射为 E_USAGE。
var ErrInvalid = errors.New("配置值非法")

// Keys 是 config get/set 支持的 KEY。
var Keys = []string{"repo"}

// Config 是用户配置。JSON 字段名即 config 命令的 KEY。
type Config struct {
	// Repo 是备份仓库目录的绝对路径。
	Repo string `json:"repo"`
}

// DefaultPath 返回配置文件路径（按 XDG_CONFIG_HOME、HOME 环境变量计算）。
func DefaultPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("无法确定用户主目录: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "databackup", "config.json"), nil
}

// Default 返回默认配置：Repo 为 ~/DataBackup。
func Default() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("无法确定用户主目录: %w", err)
	}
	return Config{Repo: filepath.Join(home, "DataBackup")}, nil
}

// Load 读取 path 处的配置；文件不存在时返回 Default()，缺失的字段用默认值补齐。
// 文件不是合法 JSON 或配置值非法时返回包装了 ErrInvalid 的错误。
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Default()
	}
	if err != nil {
		return Config{}, fmt.Errorf("读取配置文件 %s: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("配置文件 %s 不是合法 JSON（%v）: %w", path, err, ErrInvalid)
	}
	if c.Repo == "" {
		def, err := Default()
		if err != nil {
			return Config{}, err
		}
		c.Repo = def.Repo
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Save 校验 c 后写入 path：先写同目录临时文件再原子改名，必要时创建父目录。
func Save(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建配置目录 %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时文件: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("写入临时文件: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("关闭临时文件: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("替换配置文件 %s: %w", path, err)
	}
	return nil
}

// Validate 检查配置是否合法，非法时返回包装了 ErrInvalid 的错误。
func (c Config) Validate() error {
	if !filepath.IsAbs(c.Repo) {
		return fmt.Errorf("仓库路径 %q 不是绝对路径: %w", c.Repo, ErrInvalid)
	}
	info, err := os.Stat(c.Repo)
	switch {
	case err == nil && !info.IsDir():
		return fmt.Errorf("仓库路径 %q 已存在但不是目录: %w", c.Repo, ErrInvalid)
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("检查仓库路径 %q: %w", c.Repo, err)
	}
	return nil
}

// Set 把 KEY=VALUE 写入 c 的副本并校验，返回新配置。KEY 未知时返回 ErrInvalid。
func (c Config) Set(key, value string) (Config, error) {
	switch key {
	case "repo":
		c.Repo = value
	default:
		return Config{}, fmt.Errorf("未知配置项 %q（可选: %v）: %w", key, Keys, ErrInvalid)
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}
