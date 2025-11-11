package bank

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	NodeID           int64
	DBConnStr        string
	DBMaxConnections int32
	ENV              string
}
type Config struct {
	NodeID           int64  `yaml:"nodeID"`
	DbURLFormat      string `yaml:"dbURLFormat"`
	DBMaxConnections int32  `yaml:"dbMaxConnections"`
	DBUsername       string `yaml:"dbUsername"`
	DBPassword       string `yaml:"dbPassword"`
}

func LoadConfig() (*AppConfig, error) {
	baseConfigFile, err := os.ReadFile("config/bank.yaml")

	if err != nil {
		return nil, fmt.Errorf("read base config failed: %w", err)
	}

	var config Config
	err = yaml.Unmarshal(baseConfigFile, &config)

	if err != nil {
		return nil, fmt.Errorf("parse base config failed: %w", err)
	}

	err = validateConfig(config)

	if err != nil {
		return nil, err
	}

	appEnv := os.Getenv("APP_ENV")

	if appEnv == "" {
		return toAppConfig(config, "local"), nil
	}

	overrideConfigFile, err := os.ReadFile("config/bank." + appEnv + ".yaml")

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return toAppConfig(config, appEnv), err
		}

		return nil, fmt.Errorf("read override config failed: %w", err)
	}

	var overrideConfig Config
	err = yaml.Unmarshal(overrideConfigFile, &overrideConfig)

	if err != nil {
		return nil, fmt.Errorf("parse override config failed: %w", err)
	}

	if overrideConfig.NodeID != 0 {
		config.NodeID = overrideConfig.NodeID
	}
	if overrideConfig.DbURLFormat != "" {
		config.DbURLFormat = overrideConfig.DbURLFormat
	}
	if overrideConfig.DBMaxConnections != 0 {
		config.DBMaxConnections = overrideConfig.DBMaxConnections
	}
	if overrideConfig.DBUsername != "" {
		config.DBUsername = overrideConfig.DBUsername
	}
	if overrideConfig.DBPassword != "" {
		config.DBPassword = overrideConfig.DBPassword
	}

	err = validateConfig(config)

	if err != nil {
		return nil, err
	}

	return toAppConfig(config, appEnv), nil
}

func validateConfig(config Config) error {
	if config.DbURLFormat == "" {
		return errors.New("DB URL format is not set")
	}

	if config.DBMaxConnections == 0 {
		return errors.New("DB max connections is not set")
	}

	if config.DBUsername == "" {
		return errors.New("DB username is not set")
	}

	if config.DBPassword == "" {
		return errors.New("DB password is not set")
	}

	return nil
}

func toAppConfig(config Config, env string) *AppConfig {
	appConfig := &AppConfig{
		NodeID:           config.NodeID,
		DBConnStr:        fmt.Sprintf(config.DbURLFormat, config.DBUsername, config.DBPassword),
		DBMaxConnections: config.DBMaxConnections,
		ENV:              env,
	}

	return appConfig
}
