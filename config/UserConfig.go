package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func LoadUserConfig() map[string]string {
	home, err := os.UserHomeDir()
	if err != nil {
		return map[string]string{}
	}
	path := filepath.Join(home, ".kamaji", "config.yaml")

	config := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return config
	}
	defer f.Close()

	_ = yaml.NewDecoder(f).Decode(&config)
	return config
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
