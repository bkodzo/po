package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/bkodzo/critter/internal/mood"
	"github.com/bkodzo/critter/internal/pet"
	"github.com/bkodzo/critter/internal/theme"
)

func TestViewIsCompact(t *testing.T) {
	m := Model{th: theme.Soccer, pet: pet.New(), line: "Bad tackle by the compiler.", shown: 99}
	for _, md := range mood.All {
		m.pet.Mood = md
		view := strings.TrimRight(m.View(), "\n")
		rows := strings.Count(view, "\n") + 1
		width := 0
		for _, l := range strings.Split(view, "\n") {
			if w := lipgloss.Width(l); w > width {
				width = w
			}
		}
		if rows > 16 || width > 60 {
			t.Errorf("%s: view is %d rows by %d columns, want at most 16 by 60", md, rows, width)
		}
	}
}

func TestQuitClearsView(t *testing.T) {
	m := Model{th: theme.Soccer, pet: pet.New(), quit: true}
	if m.View() != "" {
		t.Error("view should be empty once quitting")
	}
}
