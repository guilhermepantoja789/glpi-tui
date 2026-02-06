# glpi-tui

`glpi-tui` é uma interface de terminal (TUI) para interagir com o [GLPI](https://glpi-project.org/), um sistema de Gerenciamento de Ativos de TI open-source. Ele permite que você visualize, gerencie e responda a chamados de suporte diretamente da sua linha de comando, oferecendo um fluxo de trabalho rápido e focado no teclado.

Construído com Go e as bibliotecas do [Charm](https://charm.sh/) (`bubbletea`, `lipgloss`, `bubbles`).

## Funcionalidades

- 📋 **Listar Chamados:** Navegue pelos seus chamados GLPI em uma lista rolável.
- 📄 **Ver Detalhes:** Visualize detalhes completos do chamado, incluindo descrição, requerentes e técnicos.
- 💬 **Histórico de Conversa:** Visualize acompanhamentos do chamado (timeline/histórico) em um formato limpo.
- ↩️ **Responder:** Adicione novos acompanhamentos/respostas aos chamados diretamente da TUI.
- 🙋 **Atribuir a Mim:** Atribua rapidamente um chamado a si mesmo com um único toque.
- 🔄 **Atualizar:** Atualize o histórico do chamado para ver os comentários mais recentes.
- 🎨 **Interface Rica:** Interface de terminal moderna com cores, spinners e layouts responsivos.

## Requisitos

- Go 1.25+

## Instalação

1. Clone o repositório:
   ```bash
   git clone https://github.com/seu-usuario/glpi-tui.git
   cd glpi-tui
   ```

2. Instale as dependências:
   ```bash
   go mod download
   ```

## Configuração

A aplicação usa variáveis de ambiente para configuração. Você pode criar um arquivo `.env` na raiz do projeto:

> **Aviso de Segurança:** Certifique-se de que o arquivo `.env` esteja incluído no seu `.gitignore` para evitar o vazamento de credenciais.

```bash
# Arquivo .env
GLPI_BASE_URL=https://seu-glpi.com/apirest.php
GLPI_CLIENT_ID=sua_app_token_key
GLPI_CLIENT_SECRET=sua_app_token_secret
GLPI_USER=seu_usuario
GLPI_PASS=sua_senha
```

| Variável | Descrição |
|----------|-------------|
| `GLPI_BASE_URL` | O endpoint da API da sua instalação GLPI (geralmente termina em `/apirest.php`). |
| `GLPI_CLIENT_ID` | Sua Chave de App Token da API GLPI (ou Client ID). |
| `GLPI_CLIENT_SECRET` | Seu Segredo de App Token da API GLPI (ou Client Secret). |
| `GLPI_USER` | Seu nome de usuário do GLPI. |
| `GLPI_PASS` | Sua senha do GLPI. |

> **Nota:** Certifique-se de ter habilitado a API nas configurações do GLPI (`Configurar > Geral > API`) e gerado o App Token necessário.

## Uso

Execute a aplicação:

```bash
go run main.go
```

### Atalhos de Teclado

**Global:**
- `Ctrl+C`: Sair da aplicação.

**Lista de Chamados:**
- `↑` / `↓` (ou `k` / `j`): Navegar na lista.
- `Enter`: Abrir detalhes do chamado selecionado.

**Detalhes do Chamado:**
- `Esc`: Voltar para a lista de chamados.
- `r`: **Responder** ao chamado (abre o editor de texto).
- `a`: **Atribuir** o chamado a você mesmo.
- `u`: **Atualizar** acompanhamentos do chamado.

**Modo de Resposta:**
- `Ctrl+S`: Enviar resposta.
- `Esc`: Cancelar resposta.

## Construído Com

- [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- [Lip Gloss](https://github.com/charmbracelet/lipgloss)
- [Bubbles](https://github.com/charmbracelet/bubbles)
