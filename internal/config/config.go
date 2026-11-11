package config

import (
	"flag"
	"os"
)

type Config struct {
	Listen   string
	DataDir  string
	LogLevel string
}

// Load 纯标准库实现：命令行 Flag > 环境变量 > 默认值
func Load() Config {
	// 1. 获取环境变量（兜底默认值）
	defaultListen := getEnv("LISTEN", ":8080")
	defaultDataDir := getEnv("DATA_DIR", "./data")
	defaultLogLevel := getEnv("LOG_LEVEL", "info")

	// 2. 允许命令行参数临时覆盖（适合临时排错与调试）
	listen := flag.String("listen", defaultListen, "Server listen address")
	dataDir := flag.String("data-dir", defaultDataDir, "Directory for database, git repo, and binaries")
	logLevel := flag.String("log-level", defaultLogLevel, "Log level (debug, info, warn, error)")
	flag.Parse()

	return Config{
		Listen:   *listen,
		DataDir:  *dataDir,
		LogLevel: *logLevel,
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
