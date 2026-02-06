package main

import (
	"fmt"
	"os"

	"glpi-tui/internal/api"
	"glpi-tui/internal/config"
	"glpi-tui/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// 1. Carrega Configurações (valida .env e variáveis obrigatórias)
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Erro de Configuração: %v\n", err)
		os.Exit(1)
	}

	// 2. Cria o Cliente API (já com timeout e base URL configurados)
	client := api.NewClient(cfg)

	// 3. Inicia o Modelo TUI (Injetando o cliente)
	m := tui.InitialModel(client)

	// 4. Roda o Programa
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Erro fatal na TUI: %v\n", err)
		os.Exit(1)
	}
}
