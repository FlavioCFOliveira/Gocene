package components

// LukeWindow implements LukeWindowOperator.
type LukeWindow struct {
	theme ColorTheme
}

func NewLukeWindow() *LukeWindow {
	return &LukeWindow{
		theme: LightTheme,
	}
}

func (w *LukeWindow) SetColorTheme(theme ColorTheme) {
	w.theme = theme
}
