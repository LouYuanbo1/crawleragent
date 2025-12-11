package config

import (
	"encoding/json"
	"path/filepath"
)

func ParseConfig(byteConfig []byte) (*Config, error) {
	var cfg Config
	err := json.Unmarshal(byteConfig, &cfg)
	if err != nil {
		return nil, err
	}
	absPath, err := filepath.Abs(cfg.Chromedp.UserDataDir)
	if err != nil {
		return nil, err
	}
	cfg.Chromedp.UserDataDir = absPath
	return &cfg, nil
}
