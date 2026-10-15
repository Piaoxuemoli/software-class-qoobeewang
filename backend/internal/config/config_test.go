package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// setHome 把 HOME 与 XDG_CONFIG_HOME 指到 t.TempDir()，返回 home 路径。
func setHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	return home
}

func TestDefaultPath(t *testing.T) {
	t.Run("TC-CFG-001 优先使用 XDG_CONFIG_HOME", func(t *testing.T) {
		setHome(t)
		p, err := DefaultPath()
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "databackup", "config.json")
		if p != want {
			t.Errorf("DefaultPath() = %q, want %q", p, want)
		}
	})

	t.Run("TC-CFG-002 未设置 XDG 时回退到 HOME/.config", func(t *testing.T) {
		home := setHome(t)
		t.Setenv("XDG_CONFIG_HOME", "")
		p, err := DefaultPath()
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(home, ".config", "databackup", "config.json")
		if p != want {
			t.Errorf("DefaultPath() = %q, want %q", p, want)
		}
	})
}

func TestDefault(t *testing.T) {
	t.Run("TC-CFG-003 默认仓库为 ~/DataBackup", func(t *testing.T) {
		home := setHome(t)
		c, err := Default()
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(home, "DataBackup"); c.Repo != want {
			t.Errorf("Default().Repo = %q, want %q", c.Repo, want)
		}
	})
}

func TestLoad(t *testing.T) {
	t.Run("TC-CFG-004 文件不存在返回默认值", func(t *testing.T) {
		home := setHome(t)
		c, err := Load(filepath.Join(t.TempDir(), "missing", "config.json"))
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(home, "DataBackup"); c.Repo != want {
			t.Errorf("Load().Repo = %q, want %q", c.Repo, want)
		}
	})

	t.Run("TC-CFG-005 缺失字段用默认值补齐", func(t *testing.T) {
		home := setHome(t)
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
		c, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(home, "DataBackup"); c.Repo != want {
			t.Errorf("Load().Repo = %q, want %q", c.Repo, want)
		}
	})

	t.Run("TC-CFG-006 非 JSON 文件返回 ErrInvalid", func(t *testing.T) {
		setHome(t)
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte("not json {"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); !errors.Is(err, ErrInvalid) {
			t.Errorf("Load() err = %v, want ErrInvalid", err)
		}
	})

	t.Run("TC-CFG-007 配置中的相对路径返回 ErrInvalid", func(t *testing.T) {
		setHome(t)
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(`{"repo":"relative/dir"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); !errors.Is(err, ErrInvalid) {
			t.Errorf("Load() err = %v, want ErrInvalid", err)
		}
	})
}

func TestValidate(t *testing.T) {
	t.Run("TC-CFG-008 相对路径不合法", func(t *testing.T) {
		c := Config{Repo: "relative/dir"}
		if err := c.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("Validate() err = %v, want ErrInvalid", err)
		}
	})

	t.Run("TC-CFG-009 已存在但不是目录不合法", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "afile")
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		c := Config{Repo: f}
		if err := c.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("Validate() err = %v, want ErrInvalid", err)
		}
	})

	t.Run("TC-CFG-010 不存在的绝对路径合法", func(t *testing.T) {
		c := Config{Repo: filepath.Join(t.TempDir(), "not-yet")}
		if err := c.Validate(); err != nil {
			t.Errorf("Validate() err = %v, want nil", err)
		}
	})
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Run("TC-CFG-011 保存后读回一致并自动建父目录", func(t *testing.T) {
		setHome(t)
		repo := filepath.Join(t.TempDir(), "repo")
		path := filepath.Join(t.TempDir(), "deep", "nested", "config.json")
		want := Config{Repo: repo}
		if err := Save(path, want); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("Load() = %+v, want %+v", got, want)
		}
	})

	t.Run("TC-CFG-012 非法配置不写入文件", func(t *testing.T) {
		setHome(t)
		path := filepath.Join(t.TempDir(), "config.json")
		if err := Save(path, Config{Repo: "relative"}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("Save() err = %v, want ErrInvalid", err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("配置文件不应被创建: %v", err)
		}
	})
}

func TestSet(t *testing.T) {
	t.Run("TC-CFG-013 设置 repo 并校验", func(t *testing.T) {
		setHome(t)
		repo := filepath.Join(t.TempDir(), "repo")
		c, err := (Config{}).Set("repo", repo)
		if err != nil {
			t.Fatal(err)
		}
		if c.Repo != repo {
			t.Errorf("Set().Repo = %q, want %q", c.Repo, repo)
		}
	})

	t.Run("TC-CFG-014 未知 KEY 返回 ErrInvalid", func(t *testing.T) {
		if _, err := (Config{}).Set("timezone", "UTC"); !errors.Is(err, ErrInvalid) {
			t.Errorf("Set() err = %v, want ErrInvalid", err)
		}
	})

	t.Run("TC-CFG-015 设置非法值返回 ErrInvalid", func(t *testing.T) {
		if _, err := (Config{}).Set("repo", "not-absolute"); !errors.Is(err, ErrInvalid) {
			t.Errorf("Set() err = %v, want ErrInvalid", err)
		}
	})
}

func TestConcurrentSave(t *testing.T) {
	t.Run("TC-CFG-016 并发保存不产生损坏文件", func(t *testing.T) {
		setHome(t)
		path := filepath.Join(t.TempDir(), "config.json")
		repos := make([]string, 8)
		for i := range repos {
			repos[i] = filepath.Join(t.TempDir(), "repo")
		}
		var wg sync.WaitGroup
		for i := range repos {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 20 {
					_ = Save(path, Config{Repo: repos[i]})
				}
			}()
		}
		wg.Wait()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var c Config
		if err := json.Unmarshal(data, &c); err != nil {
			t.Fatalf("配置文件损坏: %v\n%s", err, data)
		}
	})
}
