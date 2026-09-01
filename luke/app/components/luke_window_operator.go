package components

// ColorTheme represents the UI color theme.
type ColorTheme int

const (
	LightTheme ColorTheme = iota
	DarkTheme
)

// LukeWindowOperator is the operator for the root window.
type LukeWindowOperator interface {
	ComponentOperator

	SetColorTheme(theme ColorTheme)
}
