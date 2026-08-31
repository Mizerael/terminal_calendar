package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"

	"gitnub.com/Mizerael/terminal_calendar/internal/gcal"
	"gitnub.com/Mizerael/terminal_calendar/internal/tui"
)

func main() {
	// Load .env from the current directory without overriding real env vars.
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			log.Printf("warning: could not load .env: %v", err)
		}
	}

	calendarID := flag.String("calendar", os.Getenv("GOOGLE_CALENDAR_ID"), "calendar id to use (default: primary)")
	token := flag.String("token", os.Getenv(gcal.EnvToken), "path where the refresh token is stored")
	account := flag.String("account", os.Getenv(gcal.EnvAccount), "google account to authorize with (email); else you are prompted")
	port := flag.Int("port", 8765, "local port for the OAuth callback")
	flag.Parse()

	client, err := gcal.New(context.Background(), gcal.Options{
		CalendarID: *calendarID,
		Token:      *token,
		Port:       *port,
		Account:    *account,
		PromptAccount: func(current string) (string, error) {
			return tui.PromptAccount(current)
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	model, err := tui.New(client)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	model.SetStatePath(statePathDefault())
	model.SetInitialTarget(*calendarID)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// statePathDefault returns the location of the persisted calendar selection,
// based on the token cache directory when available.
func statePathDefault() string {
	tok := os.Getenv(gcal.EnvToken)
	if tok == "" {
		tok = "token.json"
	}
	dir := filepath.Dir(tok)
	if dir == "." || dir == "" {
		return "calendar_state.json"
	}
	return filepath.Join(dir, "calendar_state.json")
}
