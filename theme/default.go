package theme

import (
	"github.com/pterm/pterm"
)

// Theme defines the colour scheme and styling for the application
type Theme struct {
	// Log level colours
	Debug *pterm.Style
	Info  *pterm.Style
	Warn  *pterm.Style
	Error *pterm.Style
	Fatal *pterm.Style

	// Component colours
	Success   *pterm.Style
	Highlight *pterm.Style
	Muted     *pterm.Style
	Accent    *pterm.Style

	// Functional colours (now pre-built styles)
	Primary   *pterm.Style
	Secondary *pterm.Style
	Danger    *pterm.Style
	Warning   *pterm.Style
	Good      *pterm.Style

	// Application colours (now pre-built styles)
	Counts      *pterm.Style // Colour for counts like Microsofty apps Eg. Outlook (234) unread emails
	Numbers     *pterm.Style
	Endpoint    *pterm.Style
	HealthCheck *pterm.Style

	// Health check colours (now pre-built styles)
	HealthHealthy   *pterm.Style
	HealthUnhealthy *pterm.Style
	HealthUnknown   *pterm.Style
	HealthBusy      *pterm.Style
	HealthOffline   *pterm.Style
	HealthWarming   *pterm.Style
}

// Default returns the default application theme
func Default() *Theme {
	return &Theme{
		// Log level styling
		Debug: pterm.NewStyle(pterm.FgLightBlue),
		Info:  pterm.NewStyle(pterm.FgGreen),
		Warn:  pterm.NewStyle(pterm.FgYellow, pterm.Bold),
		Error: pterm.NewStyle(pterm.FgRed, pterm.Bold),
		Fatal: pterm.NewStyle(pterm.FgWhite, pterm.BgRed, pterm.Bold),

		// Component styling
		Success:   pterm.NewStyle(pterm.FgGreen, pterm.Bold),
		Highlight: pterm.NewStyle(pterm.FgCyan, pterm.Bold),
		Muted:     pterm.NewStyle(pterm.FgGray),
		Accent:    pterm.NewStyle(pterm.FgMagenta),

		// Functional styles (converted from Color to Style)
		Primary:   pterm.NewStyle(pterm.FgBlue),
		Secondary: pterm.NewStyle(pterm.FgCyan),
		Danger:    pterm.NewStyle(pterm.FgRed),
		Warning:   pterm.NewStyle(pterm.FgYellow),
		Good:      pterm.NewStyle(pterm.FgGreen),

		// Application styles (converted from Color to Style)
		Counts:      pterm.NewStyle(pterm.FgLightBlue),
		Numbers:     pterm.NewStyle(pterm.FgLightCyan),
		Endpoint:    pterm.NewStyle(pterm.FgLightMagenta),
		HealthCheck: pterm.NewStyle(pterm.FgGreen),

		// Health check styles (converted from Color to Style)
		HealthHealthy:   pterm.NewStyle(pterm.FgGreen),
		HealthUnhealthy: pterm.NewStyle(pterm.FgRed),
		HealthUnknown:   pterm.NewStyle(pterm.FgGray),
		HealthBusy:      pterm.NewStyle(pterm.FgYellow),
		HealthOffline:   pterm.NewStyle(pterm.FgLightRed),
		HealthWarming:   pterm.NewStyle(pterm.FgLightBlue),
	}
}

// Dark returns a dark theme variant
func Dark() *Theme {
	return &Theme{
		Debug: pterm.NewStyle(pterm.FgLightBlue),
		Info:  pterm.NewStyle(pterm.FgLightGreen),
		Warn:  pterm.NewStyle(pterm.FgLightYellow, pterm.Bold),
		Error: pterm.NewStyle(pterm.FgLightRed, pterm.Bold),
		Fatal: pterm.NewStyle(pterm.FgWhite, pterm.BgRed, pterm.Bold),

		Success:   pterm.NewStyle(pterm.FgLightGreen, pterm.Bold),
		Highlight: pterm.NewStyle(pterm.FgLightCyan, pterm.Bold),
		Muted:     pterm.NewStyle(pterm.FgGray),
		Accent:    pterm.NewStyle(pterm.FgLightMagenta),
