package tui

import (
	"errors"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ErrAborted is returned when the user dismisses the account prompt.
var ErrAborted = errors.New("authorization aborted")

// PromptAccount asks which Google account to authorize with and returns the
// email. An empty value means "let the browser pick". defaultEmail pre-fills
// the field with a previously used account.
func PromptAccount(defaultEmail string) (string, error) {
	ti := textinput.New()
	ti.Placeholder = "you@gmail.com (empty = browser account chooser)"
	ti.Prompt = ""
	ti.Width = 44
	ti.CharLimit = 128
	if defaultEmail != "" {
		ti.SetValue(defaultEmail)
	}
	ti.Focus()

	p := tea.NewProgram(accountPrompt{input: ti})
	m, err := p.Run()
	if err != nil {
		return "", err
	}
	pm, ok := m.(accountPrompt)
	if !ok || pm.aborted {
		return "", ErrAborted
	}
	return strings.TrimSpace(pm.input.Value()), nil
}

type accountPrompt struct {
	input   textinput.Model
	aborted bool
	width   int
	height  int
}

func (m accountPrompt) Init() tea.Cmd { return textinput.Blink }

func (m accountPrompt) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return m, tea.Quit
		case "esc", "ctrl+c":
			m.aborted = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m accountPrompt) View() string {
	if m.height == 0 {
		m.width, m.height = 80, 24
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).
		Render("Authorize terminal_calendar")
	label := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("Google account:")
	body := title + "\n\n" + label + " " + m.input.View()
	body += "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("239")).
		Render("enter: continue · esc/ctrl+c: cancel")
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).Padding(2, 4).
		Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
