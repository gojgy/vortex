package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

func LoadConfig(configPath, configName string) (*Config, error) {
	viper.SetConfigType("yaml")

	configFile := filepath.Join(configPath, configName+".yaml")

	configBytes, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read config file %s: %w", configFile, err)
	}

	expandedConfig := os.ExpandEnv(string(configBytes))

	if err := viper.ReadConfig(bytes.NewBufferString(expandedConfig)); err != nil {
		return nil, fmt.Errorf("unable to read config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal config: %w", err)
	}

	cfg.setDefaults()

	if err := cfg.validateConfig(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}
