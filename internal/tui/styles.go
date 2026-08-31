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

	pickerTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	pickerRow = lipgloss.NewStyle().
			Foreground(lipgloss.Color("251"))

	pickerCursor = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	pickerHint = lipgloss.NewStyle().
			Foreground(lipgloss.Color("239"))

	helpKey = lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Width(15).
		Inline(true)

	helpDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("249"))

	helpBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2)

	// ---- week grid styles ----

	gridGutter = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Align(lipgloss.Right).
			PaddingRight(1)

	gridHeaderDay = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("245")).
			Width(10).
			Inline(true)

	gridHeaderDayWeekend = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("239")).
				Width(10).
				Inline(true)

	gridHeaderDaySelected = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205")).
				Background(lipgloss.Color("236")).
				Width(10).
				Inline(true)

	gridHeaderDayToday = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("214")).
				Width(10).
				Inline(true)

	gridCell = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, true).
			BorderForeground(lipgloss.Color("238")).
			Width(10).
			Inline(true)

	gridCellWeekend = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, true).
			BorderForeground(lipgloss.Color("235")).
			Foreground(lipgloss.Color("237")).
			Width(10).
			Inline(true)

	gridCellFocused = lipgloss.NewStyle().
			Background(lipgloss.Color("24")).
			Border(lipgloss.NormalBorder(), false, true, false, true).
			BorderForeground(lipgloss.Color("24")).
			Width(10).
			Inline(true)

	gridEventTop = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("214")).
			Background(lipgloss.Color("58")).
			Width(9).
			PaddingLeft(1)

	gridEventCont = lipgloss.NewStyle().
			Foreground(lipgloss.Color("235")).
			Background(lipgloss.Color("58")).
			Width(10)

	gridNow = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	gridAllDay = lipgloss.NewStyle().
			Foreground(lipgloss.Color("249")).
			Background(lipgloss.Color("60")).
			Width(10).
			Inline(true)

	gridAllDayToday = lipgloss.NewStyle().
			Foreground(lipgloss.Color("254")).
			Background(lipgloss.Color("61")).
			Width(10).
			Inline(true)
)
