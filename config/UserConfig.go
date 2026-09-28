package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Loader struct{ HomeDir func() (string, error) }

func LoadUserConfig() (map[string]string, error) {
	return (Loader{}).Load()
}

func (loader Loader) Load() (map[string]string, error) {
	homeDir := loader.HomeDir
	if homeDir == nil {
		homeDir = os.UserHomeDir
	}
	home, err := homeDir()
	if err != nil {
		return nil, fmt.Errorf("locate user configuration: %w", err)
	}
	path := filepath.Join(home, ".kamaji", "config.yaml")

	config := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, fmt.Errorf("read user configuration: %w", err)
	}
	defer f.Close()

	var values struct {
		Python string `yaml:"python"`
	}
	if err := DecodeYAML(f, &values); err != nil && err != io.EOF {
		return nil, fmt.Errorf("user configuration %s: %w", path, err)
	}
	if values.Python != "" {
		config["python"] = values.Python
	}
	return config, nil
}

func ResolveConfigValue(flagVal, envVar string, yamlVal string, fallback string) string {
	if flagVal != "" {
		return flagVal
	}
	if val := os.Getenv(envVar); val != "" {
		return val
	}
	if yamlVal != "" {
		return yamlVal
	}
	return fallback
}
