// internal/config/config.go
package config

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Server ServerConfig `toml:"server"`
}

type ServerConfig struct {
	Listen   string `toml:"listen"`
	DataDir  string `toml:"data_dir"`
	LogLevel string `toml:"log_level"`
}

// Default 返回安全的默认配置
func Default() Config {
	// 核心：若容器注入了 DATA_DIR（如 /data），优先使用容器环境变量；容器外裸跑回退到 ./data
	dataDir := "./data"
	if envDir := os.Getenv("DATA_DIR"); envDir != "" {
		dataDir = envDir
	}

	return Config{
		Server: ServerConfig{
			Listen:   ":8080",
			DataDir:  dataDir,
			LogLevel: "info",
		},
	}
}

// Load 读取指定的 TOML 文件，若不存在则回退至默认值并自动创建样本
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			out, _ := toml.Marshal(cfg)
			_ = os.WriteFile(path, out, 0644)
			return cfg, nil
		}
		return cfg, err
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	// 容错兜底：若 TOML 里没有显式写 data_dir 或为空，回退到默认
	if cfg.Server.DataDir == "" {
		cfg.Server.DataDir = Default().Server.DataDir
	}

	return cfg, nil
}
