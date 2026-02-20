package tui

import (
	"fmt"
	"strings"

	"github.com/RobertoGongora/pgpipe/internal/tui/styles"
)

// helpItem represents a single key binding and its description for rendering help text.
type helpItem struct {
	Key         string
	Description string
}

// renderHelp renders a consistent help footer from the provided key bindings.
func renderHelp(items ...helpItem) string {
	if len(items) == 0 {
		return ""
	}

	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, fmt.Sprintf("[%s] %s", item.Key, item.Description))
	}

	return styles.Help.Render(strings.Join(parts, "  "))
}
