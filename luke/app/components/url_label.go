package components

import (
	"fmt"
	"net/url"
)

// URLLabel represents a label with a URL link.
type URLLabel struct {
	Text string
	URL  *url.URL
}

func NewURLLabel(text string) (*URLLabel, error) {
	u, err := url.Parse(text)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	return &URLLabel{
		Text: text,
		URL:  u,
	}, nil
}

func (l *URLLabel) Open() error {
	// In a real application, this would open the browser.
	fmt.Printf("Opening URL: %s\n", l.URL.String())
	return nil
}
