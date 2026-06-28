package main

import (
	"os"
	"strconv"
)

// Config, ajanın çalışma parametrelerini ortam değişkenlerinden okur.
// Bu sayede Docker / docker-compose içinde kolayca yapılandırılabilir.
type Config struct {
	SSHPort       string // örn: "2222" (gerçek 22 portu genelde root/cap gerektirir)
	HTTPPort      string // örn: "8080"
	BackendURL    string // HoneyDashboard log-submit endpoint'i
	HostKeyPath   string // SSH host key dosya yolu (yoksa otomatik üretilir)
	NodeName      string // bu ajanın kimliği (birden fazla ajan çalıştırıyorsan ayırt etmek için)
	MaxAttemptsIP int    // tek bir IP'den dakikada kabul edilecek max bağlantı denemesi
}

func loadConfig() Config {
	cfg := Config{
		SSHPort:       getEnv("SSH_PORT", "2222"),
		HTTPPort:      getEnv("HTTP_PORT", "8080"),
		BackendURL:    getEnv("BACKEND_URL", "http://localhost:8000/api/v1/log-submit"),
		HostKeyPath:   getEnv("SSH_HOST_KEY_PATH", "./host_key"),
		NodeName:      getEnv("NODE_NAME", "gopherpot-1"),
		MaxAttemptsIP: getEnvInt("MAX_ATTEMPTS_PER_MIN", 30),
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
