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
	Listen   string `toml:"listen"`    // 双栈监听，默认 ":8080"
	DataDir  string `toml:"data_dir"`  // 数据持久化目录，默认 "./data"
	LogLevel string `toml:"log_level"` // 启动初始日志级别，默认 "info"
}

// Default 返回安全的默认配置
func Default() Config {
	return Config{
		Server: ServerConfig{
			Listen:   ":8080",
			DataDir:  "./data",
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
			// 文件不存在时写出一份标准样本供运维参考
			out, _ := toml.Marshal(cfg)
			_ = os.WriteFile(path, out, 0644)
			return cfg, nil
		}
		return cfg, err
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
