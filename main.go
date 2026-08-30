package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

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

	calendar := flag.String("calendar", os.Getenv("GOOGLE_CALENDAR_ID"), "calendar id to use (default: primary)")
	token := flag.String("token", os.Getenv(gcal.EnvToken), "path where the refresh token is stored")
	port := flag.Int("port", 8765, "local port for the OAuth callback")
	flag.Parse()

	client, err := gcal.New(context.Background(), gcal.Options{
		CalendarID: *calendar,
		Token:      *token,
		Port:       *port,
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

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
