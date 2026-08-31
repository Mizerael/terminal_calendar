package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	subtle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	statusLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			MaxWidth(120)

	rowTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("254"))

	timebox = lipgloss.NewStyle().
		Foreground(lipgloss.Color("204")).
		Width(7).
		Inline(true)

	listCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	selectedRow = lipgloss.NewStyle().
			Background(lipgloss.Color("238")).
			Foreground(lipgloss.Color("254"))

	dayHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("245"))

	dayHeaderSelected = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205")).
				Background(lipgloss.Color("236")).
				Padding(0, 1)

	popupTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("214"))

	popupBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("203")).
			Padding(1, 2)

	detailLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("203"))

	detailValue = lipgloss.NewStyle().
			Foreground(lipgloss.Color("251")).
			MaxWidth(70)

	spinnerBox = lipgloss.NewStyle().
			Foreground(lipgloss.Color("111"))

	emptyBox = lipgloss.NewStyle().
			Foreground(lipgloss.Color("246")).
			Padding(0, 2)

	errorBox = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Padding(0, 2)

	formLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	formFocusLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	formHint = lipgloss.NewStyle().
			Foreground(lipgloss.Color("239"))

	formError = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196"))

	confirmText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("251"))

	formBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(2, 4)

	confirmBox = lipgloss.NewStyle().
			Padding(1)

	helpKey = lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Width(15).
		Inline(true)

	helpDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("249"))

	helpBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2)
)
