package tui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/uozalp/kube-waste/internal/model"
)

// Waste colouring thresholds, expressed as a percentage of the request.
const (
	wasteWarnPercent = 50
	wasteHighPercent = 80
)

// Catppuccin Macchiato palette.
const (
	ctpPink     = lipgloss.Color("#f5bde6")
	ctpMauve    = lipgloss.Color("#c6a0f6")
	ctpRed      = lipgloss.Color("#ed8796")
	ctpPeach    = lipgloss.Color("#f5a97f")
	ctpYellow   = lipgloss.Color("#eed49f")
	ctpGreen    = lipgloss.Color("#a6da95")
	ctpTeal     = lipgloss.Color("#8bd5ca")
	ctpSky      = lipgloss.Color("#91d7e3")
	ctpBlue     = lipgloss.Color("#8aadf4")
	ctpText     = lipgloss.Color("#cad3f5")
	ctpSubtext0 = lipgloss.Color("#a5adcb")
	ctpOverlay1 = lipgloss.Color("#8087a2")
	ctpSurface2 = lipgloss.Color("#5b6078")
	ctpBase     = lipgloss.Color("#24273a")
)

var (
	styleTitle   = lipgloss.NewStyle().Bold(true).Foreground(ctpBlue)
	stylePath    = lipgloss.NewStyle().Foreground(ctpTeal)
	styleSection = lipgloss.NewStyle().Bold(true).Foreground(ctpText)
	styleHeader  = lipgloss.NewStyle().Bold(true).Foreground(ctpSubtext0)
	// The active sort column keeps the header weight but switches hue.
	styleSortHeader = lipgloss.NewStyle().Bold(true).Foreground(ctpSky)
	styleDim        = lipgloss.NewStyle().Foreground(ctpOverlay1)
	styleNormal     = lipgloss.NewStyle()
	styleSelected   = lipgloss.NewStyle().Background(ctpBlue).Foreground(ctpBase).Bold(true)
	styleError      = lipgloss.NewStyle().Foreground(ctpRed)
	styleWarn       = lipgloss.NewStyle().Foreground(ctpYellow)
	styleOK         = lipgloss.NewStyle().Foreground(ctpGreen)
	styleHigh       = lipgloss.NewStyle().Foreground(ctpRed)
	styleOver       = lipgloss.NewStyle().Foreground(ctpMauve).Bold(true)
	styleKey        = lipgloss.NewStyle().Bold(true).Foreground(ctpSky)
	styleOverlay    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ctpSurface2).Padding(0, 2)

	styleBorder    = lipgloss.NewStyle().Foreground(ctpSurface2)
	styleCount     = lipgloss.NewStyle().Foreground(ctpPink)
	styleFilterTag = lipgloss.NewStyle().Foreground(ctpPeach)
)

// InitColors disables all colouring when NO_COLOR is set.
func InitColors() {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
}

// wasteStyle picks a colour from the relative waste while the displayed value
// stays absolute. Usage above the request gets a warning style.
func wasteStyle(waste, requested int64, known bool) lipgloss.Style {
	if !known {
		return styleDim
	}
	if waste < 0 {
		return styleOver
	}
	pct, ok := model.Percent(waste, requested)
	if !ok {
		return styleDim
	}
	switch {
	case pct > wasteHighPercent:
		return styleHigh
	case pct >= wasteWarnPercent:
		return styleWarn
	default:
		return styleOK
	}
}

// utilStyle colours a utilisation percentage: low utilisation is the problem.
func utilStyle(pct float64, ok bool) lipgloss.Style {
	if !ok {
		return styleDim
	}
	switch {
	case pct < 100-wasteHighPercent:
		return styleHigh
	case pct < 100-wasteWarnPercent:
		return styleWarn
	default:
		return styleOK
	}
}
