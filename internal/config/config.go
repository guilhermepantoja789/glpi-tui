package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config segura todas as variáveis de ambiente da aplicação
type Config struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
}

// Load carrega as variáveis do .env e retorna um erro se algo faltar
func Load() (*Config, error) {
	// Carrega o .env, mas não falha se o arquivo não existir (pode estar rodando via Docker envs reais)
	_ = godotenv.Load()

	cfg := &Config{
		BaseURL:      os.Getenv("GLPI_BASE_URL"),
		ClientID:     os.Getenv("GLPI_CLIENT_ID"),
		ClientSecret: os.Getenv("GLPI_CLIENT_SECRET"),
		Username:     os.Getenv("GLPI_USER"),
		Password:     os.Getenv("GLPI_PASS"),
	}

	// Validação simples para garantir que não vamos tentar rodar sem credenciais
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("GLPI_BASE_URL é obrigatório")
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("GLPI_CLIENT_ID e GLPI_CLIENT_SECRET são obrigatórios")
	}
	if cfg.Username == "" || cfg.Password == "" {
		return nil, fmt.Errorf("usuário e senha são obrigatórios")
	}

	return cfg, nil
}
