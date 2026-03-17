package main

import (
	"os"
	"strings"

	"github.com/luanlima/gaal-lib/pkg/logger"
)

const (
	defaultAddr      = "127.0.0.1:8080"
	defaultAppName   = "demo-app"
	defaultAgentName = "demo-agent"
)

type config struct {
	appName   string
	agentName string
	addr      string
	logLevel  logger.Level
}

func defaultConfig() config {
	return config{
		appName:   defaultAppName,
		agentName: defaultAgentName,
		addr:      defaultAddr,
		logLevel:  logger.LevelInfo,
	}
}

func configFromEnv() config {
	cfg := defaultConfig()
	if value := strings.TrimSpace(os.Getenv("DEMO_APP_NAME")); value != "" {
		cfg.appName = value
	}
	if value := strings.TrimSpace(os.Getenv("DEMO_AGENT_NAME")); value != "" {
		cfg.agentName = value
	}
	if value := strings.TrimSpace(os.Getenv("DEMO_APP_ADDR")); value != "" {
		cfg.addr = value
	}
	if value := strings.TrimSpace(os.Getenv("DEMO_LOG_LEVEL")); value != "" {
		cfg.logLevel = logger.Level(strings.ToLower(value))
	}
	return cfg
}

func normalizeConfig(cfg config) config {
	if strings.TrimSpace(cfg.appName) == "" {
		cfg.appName = defaultAppName
	}
	if strings.TrimSpace(cfg.agentName) == "" {
		cfg.agentName = defaultAgentName
	}
	if strings.TrimSpace(cfg.addr) == "" {
		cfg.addr = defaultAddr
	}
	if cfg.logLevel == "" {
		cfg.logLevel = logger.LevelInfo
	}
	return cfg
}
