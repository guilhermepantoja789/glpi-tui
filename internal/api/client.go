package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"glpi-tui/internal/config"
	"glpi-tui/internal/domain"
)

// TokenResponse mapeia a resposta do endpoint /token
type TokenResponse struct {
	AccessToken string `json:"access_token"`
}
type Client struct {
	cfg        *config.Config
	HTTPClient *http.Client
	Token      string
	UserID     int // <--- NOVO CAMPO: Guarda seu ID após o login
}
type FollowupInput struct {
	Content       string `json:"content"`         // Obrigatório
	RequestTypeID int    `json:"requesttypes_id"` // 1 = Helpdesk
	ItemsID       int    `json:"items_id"`        // Forçando o ID do ticket
	ItemType      string `json:"itemtype"`        // Forçando o tipo "Ticket"
}
type FollowupPayload struct {
	Content       string `json:"content"`
	RequestTypeID int    `json:"requesttypes_id"`
	ItemsID       int    `json:"items_id"`
	ItemType      string `json:"itemtype"`
}
type TeamMemberPayload struct {
	Type string `json:"type"` // "User"
	Role string `json:"role"` // "assigned"
	// UsersID int    `json:"users_id"`
}
type UserMeResponse struct {
	ID int `json:"id"`
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Login realiza a autenticação (mantido conforme original, assumindo que /token está correto no doc)
func (c *Client) Login() error {
	// Conforme doc.json: tokenUrl: "/api.php/token"
	url := c.cfg.BaseURL + "/token"

	payload := map[string]string{
		"grant_type":    "password",     // Conforme 'securitySchemes' -> 'password'
		"client_id":     c.cfg.ClientID, // Se o seu GLPI exigir, ok. Senão, user/pass basta
		"client_secret": c.cfg.ClientSecret,
		"username":      c.cfg.Username,
		"password":      c.cfg.Password,
		"scope":         "api user", // Escopos listados no doc
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("erro ao criar payload de login: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("erro ao criar requisição: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("erro de conexão com GLPI: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login falhou status: %d - resp: %s", resp.StatusCode, string(body))
	}

	var t TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return fmt.Errorf("erro ao decodificar resposta do token: %w", err)
	}

	c.Token = t.AccessToken
	return nil
}

func (c *Client) GetTickets() ([]domain.Chamado, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("client não autenticado: token vazio")
	}

	// CORREÇÃO 1: A rota correta no doc.json é /Assistance/Ticket
	endpoint := c.cfg.BaseURL + "/Assistance/Ticket"

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("erro na URL: %w", err)
	}

	q := u.Query()

	// CORREÇÃO 2: Parâmetros suportados no doc para GET /Assistance/Ticket:
	// filter, start, limit, sort
	q.Set("limit", "20")           // Default é 100, ajustando para teste
	q.Set("sort", "date_mod:desc") // Formato correto segundo doc: property:direction

	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar req: %w", err)
	}

	// Headers obrigatórios
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	// Headers opcionais listados no doc que podem ser úteis no futuro:
	// req.Header.Set("GLPI-Entity", "0")
	// req.Header.Set("GLPI-Entity-Recursive", "true")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro de conexão: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != 206 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("erro na API (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var chamados []domain.Chamado
	// O doc diz que a resposta é um array direto de schemas.Ticket
	if err := json.NewDecoder(resp.Body).Decode(&chamados); err != nil {
		return nil, fmt.Errorf("erro de decode do JSON: %w", err)
	}

	return chamados, nil
}

// GetTicketActors busca os atores (Team Members) de um chamado específico.
// Endpoint: GET /Assistance/Ticket/{id}/TeamMember
func (c *Client) GetTicketActors(ticketID int) ([]domain.TicketActor, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("client não autenticado")
	}

	endpoint := fmt.Sprintf("%s/Assistance/Ticket/%d/TeamMember", c.cfg.BaseURL, ticketID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar req de atores: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro de conexão ao buscar atores: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != 206 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("erro API atores (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var actors []domain.TicketActor
	if err := json.NewDecoder(resp.Body).Decode(&actors); err != nil {
		return nil, fmt.Errorf("erro de decode dos atores: %w", err)
	}

	return actors, nil
}

// GetTicketFollowups busca os acompanhamentos de um chamado na timeline.
// Endpoint: GET /Assistance/Ticket/{id}/Timeline/Followup
func (c *Client) GetTicketFollowups(ticketID int) ([]domain.TicketFollowup, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("client não autenticado")
	}

	endpoint := fmt.Sprintf("%s/Assistance/Ticket/%d/Timeline/Followup", c.cfg.BaseURL, ticketID)

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("erro parsing url: %w", err)
	}

	// Mantemos o expand_dropdowns=true pois vimos no debug que ele garantiu o objeto 'user' com nome
	q := u.Query()
	q.Set("expand_dropdowns", "true")
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar req de followups: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro de conexão ao buscar followups: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != 206 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("erro API followups (HTTP %d): %s", resp.StatusCode, string(body))
	}

	// --- CORREÇÃO DE ESTRUTURA ---
	// Criamos uma struct temporária apenas para fazer o decode desse "envelope" da Timeline
	type timelineItemWrapper struct {
		Type string                `json:"type"`
		Item domain.TicketFollowup `json:"item"`
	}

	var rawResponse []timelineItemWrapper

	// Decodificamos o JSON para essa lista de envelopes
	if err := json.NewDecoder(resp.Body).Decode(&rawResponse); err != nil {
		return nil, fmt.Errorf("erro de decode dos followups: %w", err)
	}

	// Agora extraímos apenas o dado útil (domain.TicketFollowup) para retornar
	var followups []domain.TicketFollowup
	for _, wrapper := range rawResponse {
		followups = append(followups, wrapper.Item)
	}

	return followups, nil
}

// CreateTicketFollowup envia um novo acompanhamento.
// Endpoint: POST /Assistance/Ticket/{id}/Timeline/Followup
func (c *Client) CreateTicketFollowup(ticketID int, content string) error {
	if c.Token == "" {
		return fmt.Errorf("client não autenticado")
	}

	endpoint := fmt.Sprintf("%s/Assistance/Ticket/%d/Timeline/Followup", c.cfg.BaseURL, ticketID)

	// TENTATIVA 3: Enviar o JSON "plano", sem o wrapper input, e com HTML simples
	// Às vezes o GLPI ignora texto plano se a validação de RichText estiver estrita
	payload := FollowupPayload{
		Content:       fmt.Sprintf("<p>%s</p>", content), // Envelopa em HTML
		RequestTypeID: 1,
		ItemsID:       ticketID,
		ItemType:      "Ticket",
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("erro ao criar payload: %w", err)
	}

	// DEBUG: Vamos imprimir no terminal o que está sendo enviado para ter certeza
	fmt.Printf("\n--- DEBUG PAYLOAD ---\n%s\n---------------------\n", string(jsonPayload))

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("erro ao criar requisição: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	// Se você usa App-Token no header globalmente, verifique se ele está aqui
	// req.Header.Set("App-Token", "SEU_APP_TOKEN")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("erro de conexão ao criar followup: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("erro API criar followup (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetMyID consulta o endpoint fornecido pela documentação
func (c *Client) GetMyID() error {
	if c.Token == "" {
		return fmt.Errorf("client não autenticado")
	}

	// CORREÇÃO: Usando estritamente o endpoint fornecido pelo NotebookLM
	// Antes eu presumi incorretamente que seria apenas /User/Me
	endpoint := c.cfg.BaseURL + "/Administration/User/Me"

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("erro ao criar req UserMe: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("erro de conexão UserMe: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// Se der 404 de novo aqui, significa que a documentação pode ter nos dado o caminho da interface web e não da API
		// Mas primeiro temos que testar o que ela mandou.
		return fmt.Errorf("erro API UserMe (HTTP %d) no endpoint %s: %s", resp.StatusCode, endpoint, string(body))
	}

	var u UserMeResponse
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return fmt.Errorf("erro decode UserMe: %w", err)
	}

	c.UserID = u.ID
	return nil
}

// AssignTicketViaUpdate atribui o ticket usando a rota principal (PATCH)
// Documentação: PATCH /Assistance/Ticket/{id} exige envelope "input"
func (c *Client) AssignTicketViaUpdate(ticketID int, entityID int) error {
	if c.UserID == 0 {
		return fmt.Errorf("ID do usuário desconhecido. GetMyID foi chamado?")
	}

	endpoint := fmt.Sprintf("%s/Assistance/Ticket/%d", c.cfg.BaseURL, ticketID)

	// ESTRUTURA DO PAYLOAD (Escrita):
	// Usamos "users_id_assign" para definir o técnico.
	// Usamos "status": 2 para mover para "Processing/Assigned".
	// Isso evita mexer no array complexo "team" e sobrescrever dados.
	payload := map[string]interface{}{
		"input": map[string]interface{}{
			"status":          2,        // 2 = Em atendimento
			"users_id_assign": c.UserID, // O ID do técnico (você)
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("erro payload patch: %w", err)
	}

	// IMPORTANTE: Método PATCH (Atualização Parcial)
	req, err := http.NewRequest("PATCH", endpoint, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("erro req patch: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	// Contexto da Entidade (Obrigatório segundo doc.txt)
	req.Header.Set("GLPI-Entity", fmt.Sprintf("%d", entityID))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("erro conexão patch: %w", err)
	}
	defer resp.Body.Close()

	// O GLPI retorna 200 ou 204 no sucesso do PATCH
	if resp.StatusCode != http.StatusOK && resp.StatusCode != 200 && resp.StatusCode != 204 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyString := string(bodyBytes)

		var debugBuilder strings.Builder
		debugBuilder.WriteString(fmt.Sprintf("\n--- STATUS: %d ---\n", resp.StatusCode))
		debugBuilder.WriteString(fmt.Sprintf("PAYLOAD: %s\n", string(jsonPayload)))
		debugBuilder.WriteString("--- HEADERS RESPOSTA ---\n")
		for k, v := range resp.Header {
			if strings.Contains(k, "GLPI") || strings.Contains(k, "Message") {
				debugBuilder.WriteString(fmt.Sprintf("%s: %s\n", k, v))
			}
		}
		return fmt.Errorf("FALHA PATCH:\n%s\nBODY: %s", debugBuilder.String(), bodyString)
	}

	return nil
}
