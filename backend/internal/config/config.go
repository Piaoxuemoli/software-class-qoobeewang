// Package config 读写用户配置（FR-15），存于 $XDG_CONFIG_HOME/databackup/config.json
// （未设置时为 ~/.config/databackup/config.json）。
package config

import (
	"errors"
)

var errNotImplemented = errors.New("config: 尚未实现")

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
	return "", errNotImplemented
}

// Default 返回默认配置：Repo 为 ~/DataBackup。
func Default() (Config, error) {
	return Config{}, errNotImplemented
}

// Load 读取 path 处的配置；文件不存在时返回 Default()，缺失的字段用默认值补齐。
func Load(path string) (Config, error) {
	return Config{}, errNotImplemented
}

// Save 校验 c 后写入 path：先写同目录临时文件再原子改名，必要时创建父目录。
func Save(path string, c Config) error {
	return errNotImplemented
}

// Validate 检查配置是否合法，非法时返回包装了 ErrInvalid 的错误。
func (c Config) Validate() error {
	return errNotImplemented
}

// Set 把 KEY=VALUE 写入 c 的副本并校验，返回新配置。KEY 未知时返回 ErrInvalid。
func (c Config) Set(key, value string) (Config, error) {
	return Config{}, errNotImplemented
}
