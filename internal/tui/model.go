package tui

import (
	"fmt"
	"glpi-tui/internal/api"
	"glpi-tui/internal/domain"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- MENSAGENS DO SISTEMA ---
// loginSuccessMsg indica que o token foi obtido
type loginSuccessMsg struct{}

// ticketsLoadedMsg traz a lista de chamados do backend
type ticketsLoadedMsg []domain.Chamado

// errMsg para tratar erros de forma genérica
type errMsg error
type ticketActorsLoadedMsg struct {
	ticketID int
	actors   []domain.TicketActor
}

// ticketFollowupsLoadedMsg traz a lista de acompanhamentos
type ticketFollowupsLoadedMsg struct {
	ticketID  int
	followups []domain.TicketFollowup
}
type followupCreatedMsg struct{} // Sucesso no envio
type assignedSuccessMsg struct{} // Indica sucesso na atribuiçao

// --- MODEL PRINCIPAL ---
type model struct {
	client   *api.Client
	list     list.Model
	viewport viewport.Model
	spinner  spinner.Model

	// Novos campos para a área de resposta
	textarea           textarea.Model
	responding         bool // true = mostrando a caixa de texto, false = navegando
	refreshing         bool
	chamadoSelecionado *domain.Chamado
	err                error
	loading            bool
	ready              bool
}

// --- INITIAL MODEL ---
func InitialModel(client *api.Client) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Chamados GLPI"
	l.SetShowHelp(false)

	// Configuração do Textarea
	ta := textarea.New()
	ta.Placeholder = "Digite sua resposta aqui... (Ctrl+S para enviar, Esc para cancelar)"
	ta.Focus()          // Fica focado quando ativado
	ta.CharLimit = 2000 // Limite razoável para GLPI
	ta.SetWidth(50)     // Largura inicial, será ajustada no WindowSizeMsg
	ta.SetHeight(5)     // Altura da caixa de texto
	ta.ShowLineNumbers = false

	return model{
		client:     client,
		list:       l,
		spinner:    s,
		textarea:   ta,    // <--- Injecao
		responding: false, // Começa oculto
		loading:    true,
	}
}

// Init é a primeira função que o Bubble Tea roda
func (m model) Init() tea.Cmd {
	// Inicia o spinner E dispara o login em paralelo
	return tea.Batch(
		spinner.Tick,
		performLoginCmd(m.client),
	)
}

// --- COMANDOS ASSÍNCRONOS (API) ---

// performLoginCmd realiza o login (Network I/O)
func performLoginCmd(c *api.Client) tea.Cmd {
	return func() tea.Msg {
		if err := c.Login(); err != nil {
			return errMsg(err)
		}
		return loginSuccessMsg{}
	}
}

// fetchTicketsCmd busca os chamados usando o token já salvo
func fetchTicketsCmd(c *api.Client) tea.Cmd {
	return func() tea.Msg {
		tickets, err := c.GetTickets()
		if err != nil {
			return errMsg(err)
		}
		return ticketsLoadedMsg(tickets)
	}
}

func createFollowupCmd(c *api.Client, ticketID int, content string) tea.Cmd {
	return func() tea.Msg {
		if err := c.CreateTicketFollowup(ticketID, content); err != nil {
			return errMsg(err)
		}
		return followupCreatedMsg{}
	}
}

// fetchActorsCmd busca os atores de um ticket específico em background
func fetchActorsCmd(c *api.Client, id int) tea.Cmd {
	return func() tea.Msg {
		actors, err := c.GetTicketActors(id)
		if err != nil {
			// Em caso de erro, podemos retornar um erro genérico ou logar
			// Por enquanto retornamos vazio para não travar a UI
			return ticketActorsLoadedMsg{ticketID: id, actors: []domain.TicketActor{}}
		}
		return ticketActorsLoadedMsg{
			ticketID: id,
			actors:   actors,
		}
	}
}

// fetchFollowupsCmd busca os acompanhamentos em background
func fetchFollowupsCmd(c *api.Client, id int) tea.Cmd {
	return func() tea.Msg {
		followups, err := c.GetTicketFollowups(id)
		if err != nil {
			// Retorna lista vazia em caso de erro para não travar
			return ticketFollowupsLoadedMsg{ticketID: id, followups: []domain.TicketFollowup{}}
		}
		return ticketFollowupsLoadedMsg{
			ticketID:  id,
			followups: followups,
		}
	}
}

// fetchMyIDCmd busca o ID do usuário em background
func fetchMyIDCmd(c *api.Client) tea.Cmd {
	return func() tea.Msg {
		if err := c.GetMyID(); err != nil {
			return errMsg(err)
		}
		return nil // Sem mensagem específica, apenas sucesso silencioso (ou log)
	}
}

// assignToMeCmd dispara a atribuição via UPDATE (PATCH)
func assignToMeCmd(c *api.Client, ticketID int, entityID int) tea.Cmd {
	return func() tea.Msg {
		// CORREÇÃO: Chamar AssignTicketViaUpdate em vez de AssignTicketToMe
		if err := c.AssignTicketViaUpdate(ticketID, entityID); err != nil {
			return errMsg(err)
		}
		return assignedSuccessMsg{}
	}
}

// --- UPDATE LOOP ---

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	// --- 1. MODO DE RESPOSTA (Foco na Caixa de Texto) ---
	if m.responding {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				// Cancela e volta para visualização
				m.responding = false
				m.textarea.Reset()
				return m, nil

			case "ctrl+s":
				// Envia o followup
				content := m.textarea.Value()
				if content == "" {
					return m, nil // Não envia vazio
				}
				m.responding = false
				m.textarea.Reset()

				// Dispara comando de criação + loading visual se quisesse
				return m, createFollowupCmd(m.client, m.chamadoSelecionado.ID, content)
			}
		}

		// Atualiza o componente textarea (digitação, cursor, etc)
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd
	}

	// --- 2. MODO NORMAL (Navegação) ---

	switch msg := msg.(type) {
	// Teclas Globais
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "esc":
			if m.chamadoSelecionado != nil {
				// Sai dos detalhes e volta pra lista
				m.chamadoSelecionado = nil
				return m, nil
			}
		case "a":
			if m.chamadoSelecionado != nil {
				// Verifica se já temos o ID do usuário
				if m.client.UserID == 0 {
					m.err = fmt.Errorf("aguarde, carregando perfil de usuário...")
					return m, nil
				}
				m.refreshing = true // Feedback visual
				return m, assignToMeCmd(m.client, m.chamadoSelecionado.ID, m.chamadoSelecionado.Entity.ID)
			}
		// Abre a caixa de resposta se estiver vendo um chamado
		case "r":
			if m.chamadoSelecionado != nil {
				m.responding = true
				m.textarea.Placeholder = "Escreva sua resposta para o chamado #" + fmt.Sprint(m.chamadoSelecionado.ID) + "..."
				m.textarea.Focus()
				return m, textarea.Blink // Comando necessário para o cursor piscar
			}
		case "u":
			if m.chamadoSelecionado != nil && !m.refreshing { // Evita spam de 'u'
				m.refreshing = true // 1. Ativa o indicador
				// Opcional: Adiciona spinner.Tick se quiser animar o icone, mas só texto já basta
				return m, fetchFollowupsCmd(m.client, m.chamadoSelecionado.ID)
			}
		}

	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height)

		// Ajusta viewport (deixando espaço para rodapé se precisar)
		m.viewport = viewport.New(msg.Width, msg.Height-5)

		// Ajusta largura da caixa de texto para caber na tela
		m.textarea.SetWidth(msg.Width - 4)
		m.ready = true

	// --- MENSAGENS DE API ---

	case loginSuccessMsg:
		// SUCESSO NO LOGIN: Dispara busca de Tickets E busca do ID do Usuário
		cmds = append(cmds, fetchTicketsCmd(m.client))
		cmds = append(cmds, fetchMyIDCmd(m.client))

	case ticketsLoadedMsg:
		m.loading = false
		items := make([]list.Item, len(msg))
		for i, t := range msg {
			items[i] = t
		}
		m.list.SetItems(items)

	case ticketActorsLoadedMsg:
		if m.chamadoSelecionado != nil && m.chamadoSelecionado.ID == msg.ticketID {
			m.chamadoSelecionado.Actors = msg.actors
			m.renderChamadoDetalhes()
		}

	case ticketFollowupsLoadedMsg:
		if m.chamadoSelecionado != nil && m.chamadoSelecionado.ID == msg.ticketID {
			m.refreshing = false // 2. Desativa o indicador quando chega

			// Lógica de ordenação (mantém a que fizemos antes)
			var lista []domain.TicketFollowup
			if msg.followups == nil {
				lista = []domain.TicketFollowup{}
			} else {
				lista = msg.followups
			}

			sort.Slice(lista, func(i, j int) bool {
				return lista[i].ID > lista[j].ID
			})

			m.chamadoSelecionado.Followups = lista
			m.renderChamadoDetalhes()
		}

	case followupCreatedMsg:
		if m.chamadoSelecionado != nil {
			// Adiciona um feedback visual temporário se quiser, ou só recarrega
			cmds = append(cmds, fetchFollowupsCmd(m.client, m.chamadoSelecionado.ID))
		}

	case assignedSuccessMsg:
		// SUCESSO NA ATRIBUIÇÃO
		m.refreshing = false
		if m.chamadoSelecionado != nil {
			// Recarrega os Atores para mostrar o nome do técnico na tela imediatamente
			cmds = append(cmds, fetchActorsCmd(m.client, m.chamadoSelecionado.ID))
		}

	case errMsg:
		m.err = msg
		m.loading = false
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmdSpinner tea.Cmd
			m.spinner, cmdSpinner = m.spinner.Update(msg)
			return m, cmdSpinner
		}
	}

	// Lógica Padrão (Lista ou Viewport)
	if m.chamadoSelecionado == nil {
		if !m.loading {
			m.list, cmd = m.list.Update(msg)
			cmds = append(cmds, cmd)
		}
		// Enter para selecionar
		if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" && !m.loading {
			if i, ok := m.list.SelectedItem().(domain.Chamado); ok {
				m.chamadoSelecionado = &i
				// Limpa cache visual
				m.chamadoSelecionado.Actors = nil
				m.chamadoSelecionado.Followups = nil
				m.renderChamadoDetalhes()

				cmds = append(cmds, fetchActorsCmd(m.client, i.ID))
				cmds = append(cmds, fetchFollowupsCmd(m.client, i.ID))
			}
		}
	} else {
		// Estamos vendo detalhes (mas não respondendo)
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// Helper para renderizar o conteúdo bonito no viewport
func (m *model) renderChamadoDetalhes() {
	if m.chamadoSelecionado == nil {
		return
	}

	c := m.chamadoSelecionado

	// Estilos
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Cabeçalho
	header := fmt.Sprintf("%s\n%s",
		titleStyle.Render(fmt.Sprintf("#%d %s", c.ID, c.Name)),
		infoStyle.Render(fmt.Sprintf("Aberto em: %s", c.GetFormattedDate())),
	)

	// Atores
	requester := "Carregando..."
	tech := "Carregando..."
	if c.Actors != nil {
		requester = c.GetRequesters()
		tech = c.GetTechnicians()
	}

	actorsInfo := fmt.Sprintf("\n👤 Requerente: %s\n🔧 Técnico: %s\n",
		lipgloss.NewStyle().Bold(true).Render(requester),
		lipgloss.NewStyle().Bold(true).Render(tech),
	)

	// Conteúdo Principal (Descrição)
	descriptionSection := fmt.Sprintf("%s\n%s",
		dividerStyle.Render(strings.Repeat("─", m.viewport.Width)),
		c.GetCleanContent(),
	)

	// Seção de Followups (Acompanhamentos)
	var followupsSection string

	if c.Followups == nil {
		followupsSection = "\n\n" + infoStyle.Render("Carregando histórico...")
	} else if len(c.Followups) > 0 {
		var sb strings.Builder
		sb.WriteString("\n\n" + titleStyle.Background(lipgloss.Color("#444")).Render(" 💬 Acompanhamentos ") + "\n")

		for _, f := range c.Followups {
			// Estilo do cabeçalho do followup (Quem e Quando)
			fHeader := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00D7D7")).Render(f.User.Name)
			fDate := infoStyle.Render(f.GetFormattedDate())

			sb.WriteString(fmt.Sprintf("\n%s em %s\n%s\n", fHeader, fDate, dividerStyle.Render(strings.Repeat("-", 20))))
			sb.WriteString(fmt.Sprintf("%s\n", f.GetCleanContent()))
		}
		followupsSection = sb.String()
	} else {
		followupsSection = "\n\n" + infoStyle.Render("Nenhum acompanhamento registrado.")
	}

	// Montagem Final
	content := fmt.Sprintf("%s\n%s%s%s",
		header,
		actorsInfo,
		descriptionSection,
		followupsSection,
	)

	m.viewport.SetContent(content)
}

// --- VIEW (Renderização) ---

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("\n  ❌ Erro: %v\n\n  Pressione Ctrl+C para sair.", m.err)
	}

	if m.loading {
		return fmt.Sprintf("\n %s Conectando ao GLPI...\n", m.spinner.View())
	}

	// Se estiver vendo detalhes
	if m.chamadoSelecionado != nil {

		// 1. Renderiza o conteúdo do chamado (Viewport)
		viewContent := m.viewport.View()

		// 2. Se estiver respondendo, desenha a caixa de texto embaixo
		if m.responding {
			borderColor := lipgloss.Color("205") // Rosa choque para destaque
			if m.textarea.Focused() {
				borderColor = lipgloss.Color("69") // Azul se focado (sempre está aqui)
			}

			boxStyle := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(borderColor).
				Padding(0, 1).
				MarginTop(1)

			textareaView := boxStyle.Render(m.textarea.View())

			// Dica de rodapé
			help := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Ctrl+S: Enviar • Esc: Cancelar")

			// Junta o viewport + caixa de texto + ajuda
			return fmt.Sprintf("%s\n%s\n%s", viewContent, textareaView, help)
		}

		// --- RODAPÉ DINÂMICO ---
		var footer string

		if m.refreshing {
			// Mostra feedback de carregamento em amarelo/laranja
			footer = lipgloss.NewStyle().
				Foreground(lipgloss.Color("208")).
				Bold(true).
				Render("\nAtualizando histórico... aguarde.")
		} else {
			// Mostra os comandos normais
			footer = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Render("\n[r] Responder • [u] Atualizar • [a] Atribuir a Mim • [Esc] Voltar")
		}

		return fmt.Sprintf("%s\n%s", viewContent, footer)
	}

	// Tela de Lista Principal
	return m.list.View()
}
