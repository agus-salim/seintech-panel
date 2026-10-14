package config

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Port        int
	ListenAddr  string
	DataDir     string
	WebRoot     string
	SSLBase     string
	ACMERoot    string
	Environment string
	DevMode     bool
}

func LoadConfig() *Config {
	port := 8443
	if envPort := os.Getenv("SEINTECH_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			port = p
		}
	}

	listenAddr := "0.0.0.0"
	if envAddr := os.Getenv("SEINTECH_ADDR"); envAddr != "" {
		listenAddr = envAddr
	}

	dataDir := "/var/lib/seintech/data"
	webRoot := "/var/www"
	sslBase := "/var/lib/seintech/ssl"
	acmeRoot := "/var/lib/seintech/acme-challenge"

	devMode := false
	if os.Getenv("SEINTECH_DEV") == "1" || os.Getenv("ENVIRONMENT") == "development" {
		devMode = true
		cwd, _ := os.Getwd()
		dataDir = filepath.Join(cwd, "data")
		webRoot = filepath.Join(cwd, "vhosts")
		sslBase = filepath.Join(cwd, "ssl")
		acmeRoot = filepath.Join(cwd, "acme-challenge")
	}

	return &Config{
		Port:        port,
		ListenAddr:  listenAddr,
		DataDir:     dataDir,
		WebRoot:     webRoot,
		SSLBase:     sslBase,
		ACMERoot:    acmeRoot,
		Environment: os.Getenv("ENVIRONMENT"),
		DevMode:     devMode,
	}
}
