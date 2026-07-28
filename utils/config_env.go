package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

type databaseEnvBinding struct {
	name  string
	path  []string
	parse func(string) (any, error)
}

var databaseEnvBindings = []databaseEnvBinding{
	{name: "NAV_DB_TYPE", path: []string{"system", "db-type"}, parse: parseStringEnv},
	{name: "NAV_PGSQL_HOST", path: []string{"pgsql", "host"}, parse: parseStringEnv},
	{name: "NAV_PGSQL_PORT", path: []string{"pgsql", "port"}, parse: parseStringEnv},
	{name: "NAV_PGSQL_CONFIG", path: []string{"pgsql", "config"}, parse: parseStringEnv},
	{name: "NAV_PGSQL_DB_NAME", path: []string{"pgsql", "db-name"}, parse: parseStringEnv},
	{name: "NAV_PGSQL_USERNAME", path: []string{"pgsql", "username"}, parse: parseStringEnv},
	{name: "NAV_PGSQL_PASSWORD", path: []string{"pgsql", "password"}, parse: parseStringEnv},
	{name: "NAV_PGSQL_LOG_MODE", path: []string{"pgsql", "log-mode"}, parse: parseStringEnv},
	{name: "NAV_PGSQL_LOG_ZAP", path: []string{"pgsql", "log-zap"}, parse: parseBoolEnv},
	{name: "NAV_PGSQL_MAX_IDLE_CONNS", path: []string{"pgsql", "max-idle-conns"}, parse: parseIntEnv},
	{name: "NAV_PGSQL_MAX_OPEN_CONNS", path: []string{"pgsql", "max-open-conns"}, parse: parseIntEnv},
}

// PrepareDatabaseEnvConfig renders database environment overrides to a private
// runtime config because nav-common-go-lib only reads configuration from files.
func PrepareDatabaseEnvConfig() (func(), error) {
	overrides := make(map[string]any)
	for _, binding := range databaseEnvBindings {
		raw, ok := os.LookupEnv(binding.name)
		if !ok {
			continue
		}
		value, err := binding.parse(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", binding.name, err)
		}
		setNestedConfigValue(overrides, binding.path, value)
	}
	if len(overrides) == 0 {
		return func() {}, nil
	}

	configPath, err := activeConfigPath()
	if err != nil {
		return nil, err
	}
	rawConfig, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config for database environment overrides: %w", err)
	}

	var config map[string]any
	if err = yaml.Unmarshal(rawConfig, &config); err != nil {
		return nil, fmt.Errorf("parse config for database environment overrides: %w", err)
	}
	mergeConfigValues(config, overrides)

	rendered, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("render database environment overrides: %w", err)
	}
	runtimeConfig, err := os.CreateTemp("", "navapi-config-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("create runtime config: %w", err)
	}
	runtimePath := runtimeConfig.Name()
	cleanupFile := func() {
		_ = runtimeConfig.Close()
		_ = os.Remove(runtimePath)
	}
	if _, err = runtimeConfig.Write(rendered); err != nil {
		cleanupFile()
		return nil, fmt.Errorf("write runtime config: %w", err)
	}
	if err = runtimeConfig.Close(); err != nil {
		cleanupFile()
		return nil, fmt.Errorf("close runtime config: %w", err)
	}

	previousConfig, hadPreviousConfig := os.LookupEnv("NAV_CONFIG")
	if err = os.Setenv("NAV_CONFIG", runtimePath); err != nil {
		cleanupFile()
		return nil, fmt.Errorf("activate runtime config: %w", err)
	}
	return func() {
		if hadPreviousConfig {
			_ = os.Setenv("NAV_CONFIG", previousConfig)
		} else {
			_ = os.Unsetenv("NAV_CONFIG")
		}
		_ = os.Remove(runtimePath)
	}, nil
}

func activeConfigPath() (string, error) {
	if configPath := configArgPath(os.Args[1:]); configPath != "" {
		return filepath.Abs(configPath)
	}
	if configPath := strings.TrimSpace(os.Getenv("NAV_CONFIG")); configPath != "" {
		return filepath.Abs(configPath)
	}

	modeConfig := "config.debug.yaml"
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GIN_MODE"))) {
	case "release":
		modeConfig = "config.release.yaml"
	case "test":
		modeConfig = "config.test.yaml"
	}
	if _, err := os.Stat(modeConfig); err == nil {
		return filepath.Abs(modeConfig)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if _, err := os.Stat(defaultConfigFileName); err == nil {
		return filepath.Abs(defaultConfigFileName)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return "", errors.New("no active config file found for database environment overrides")
}

func configArgPath(args []string) string {
	for i, arg := range args {
		if (arg == "-c" || arg == "--c") && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, "-c=") {
			return strings.TrimPrefix(arg, "-c=")
		}
		if strings.HasPrefix(arg, "--c=") {
			return strings.TrimPrefix(arg, "--c=")
		}
	}
	return ""
}

func setNestedConfigValue(config map[string]any, path []string, value any) {
	current := config
	for _, key := range path[:len(path)-1] {
		next, ok := current[key].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[key] = next
		}
		current = next
	}
	current[path[len(path)-1]] = value
}

func mergeConfigValues(target, overrides map[string]any) {
	for key, value := range overrides {
		overrideMap, isMap := value.(map[string]any)
		if !isMap {
			target[key] = value
			continue
		}
		targetMap, ok := target[key].(map[string]any)
		if !ok {
			targetMap = make(map[string]any)
			target[key] = targetMap
		}
		mergeConfigValues(targetMap, overrideMap)
	}
}

func parseStringEnv(value string) (any, error) {
	return value, nil
}

func parseIntEnv(value string) (any, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return nil, errors.New("must be an integer")
	}
	if parsed < 0 {
		return nil, errors.New("must be zero or greater")
	}
	return parsed, nil
}

func parseBoolEnv(value string) (any, error) {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return nil, errors.New("must be a boolean")
	}
	return parsed, nil
}
