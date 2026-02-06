package domain

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const (
	StatusNew     = 1
	StatusAssign  = 2
	StatusPlanned = 3
	StatusPending = 4
	StatusSolved  = 5
	StatusClosed  = 6
)

// TicketStatus representa o objeto de status retornado pela API High-Level
type TicketStatus struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
type TicketActor struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // User, Group, Supplier
	Role string `json:"role"` // requester, assigned, observer
}

// TicketFollowupUser representa quem escreveu o acompanhamento
type TicketFollowupUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// TicketFollowup representa um item da timeline (Acompanhamento)
// Endpoint: GET /Assistance/Ticket/{id}/Timeline/Followup
type TicketFollowup struct {
	ID      int                `json:"id"`
	Date    string             `json:"date"`
	Content string             `json:"content"`
	User    TicketFollowupUser `json:"user"`
}
type TicketEntity struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Chamado reflete o schema "Ticket"
type Chamado struct {
	ID       int          `json:"id"`
	Name     string       `json:"name"`
	Content  string       `json:"content"`
	Date     string       `json:"date"`
	Status   TicketStatus `json:"status"`
	Priority int          `json:"priority"`

	// O GLPI retorna: "entity": {"id": 0, "name": "Root entity", ...}
	Entity TicketEntity `json:"entity"`

	// Campos carregados sob demanda
	Actors    []TicketActor    `json:"-"`
	Followups []TicketFollowup `json:"-"`
}

func (c Chamado) Title() string { return c.Name }

func (c Chamado) Description() string {
	// Ajustado para usar c.Status.ID e c.Status.Name
	statusTexto, cor := c.getStatusInfo()
	badge := lipgloss.NewStyle().Foreground(cor).Render(statusTexto)
	return fmt.Sprintf("%s | Prio: %s | ID: %d | %s", badge, c.GetPriorityLabel(), c.ID, c.GetFormattedDate())
}

func (c Chamado) FilterValue() string { return c.Name }

func (c Chamado) getStatusInfo() (string, lipgloss.Color) {
	// O switch agora verifica o ID dentro do objeto Status
	switch c.Status.ID {
	case StatusNew:
		return "Novo", lipgloss.Color("#04B575")
	case StatusAssign:
		return "Atribuído", lipgloss.Color("#007BFF")
	case StatusPlanned:
		return "Planejado", lipgloss.Color("#FFC107")
	case StatusPending:
		return "Pendente", lipgloss.Color("#FF8800")
	case StatusSolved:
		return "Solucionado", lipgloss.Color("#6C757D")
	case StatusClosed:
		return "Fechado", lipgloss.Color("#000000")
	default:
		// Fallback usando o Nome retornado pela API se houver
		name := c.Status.Name
		if name == "" {
			name = fmt.Sprintf("Status %d", c.Status.ID)
		}
		return name, lipgloss.Color("#888888")
	}
}

func (c Chamado) GetPriorityLabel() string {
	switch c.Priority {
	case 1:
		return "M. Baixa"
	case 2:
		return "Baixa"
	case 3:
		return "Média"
	case 4:
		return "Alta"
	case 5:
		return "M. Alta"
	case 6:
		return "!CRÍTICA!"
	default:
		return fmt.Sprintf("%d", c.Priority)
	}
}

func (c Chamado) GetFormattedDate() string {
	// A API retorna date-time (ex: 2024-01-30T10:00:00)
	// Vamos tentar parsing genérico compatível com RFC3339 ou SQL datetime
	layouts := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
	}

	var t time.Time
	var err error

	for _, layout := range layouts {
		t, err = time.Parse(layout, c.Date)
		if err == nil {
			break
		}
	}

	if err != nil {
		return c.Date
	}
	return t.Format("02/01/06 15:04")
}

func (c Chamado) GetCleanContent() string {
	text := c.Content
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = strings.ReplaceAll(text, "<br />", "\n")
	text = strings.ReplaceAll(text, "</p>", "\n\n")

	// Remove todas as tags HTML
	re := regexp.MustCompile(`<[^>]*>`)
	text = re.ReplaceAllString(text, "")

	text = html.UnescapeString(text)
	return strings.TrimSpace(text)
}

// GetRequesters retorna uma string formatada com os nomes dos requerentes
func (c Chamado) GetRequesters() string {
	var names []string
	for _, actor := range c.Actors {
		if actor.Role == "requester" {
			names = append(names, actor.Name)
		}
	}
	if len(names) == 0 {
		return "N/A"
	}
	return strings.Join(names, ", ")
}

// GetTechnicians retorna uma string formatada com os nomes dos técnicos
func (c Chamado) GetTechnicians() string {
	var names []string
	for _, actor := range c.Actors {
		if actor.Role == "assigned" {
			names = append(names, actor.Name)
		}
	}
	if len(names) == 0 {
		return "Pendente"
	}
	return strings.Join(names, ", ")
}

func (f TicketFollowup) GetCleanContent() string {
	// Reutiliza a lógica de limpeza que já temos (poderíamos refatorar para uma função utilitária global, mas por agora duplicamos a lógica simples para manter isolamento)
	text := f.Content
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = strings.ReplaceAll(text, "</p>", "\n\n")

	re := regexp.MustCompile(`<[^>]*>`)
	text = re.ReplaceAllString(text, "")

	text = html.UnescapeString(text)
	return strings.TrimSpace(text)
}

func (f TicketFollowup) GetFormattedDate() string {
	// Tenta parsear com o formato RFC3339 (que cobre o formato T...-03:00 do seu JSON)
	t, err := time.Parse(time.RFC3339, f.Date)
	if err != nil {
		// Se falhar, tenta o formato SQL simples só por garantia
		t, err = time.Parse("2006-01-02 15:04:05", f.Date)
		if err != nil {
			return f.Date
		}
	}
	// Formata para o padrão brasileiro de leitura
	return t.Format("02/01/06 15:04")
}
