package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bkodzo/critter/internal/mood"
	"github.com/bkodzo/critter/internal/pet"
	"github.com/bkodzo/critter/internal/theme"
)

type tickMsg time.Time

type readMsg struct {
	reading mood.Reading
	err     error
}

type Model struct {
	th     theme.Theme
	pet    *pet.Pet
	reader mood.Reader

	tick    int
	input   string
	line    string
	shown   int
	queue   []string
	busy    bool
	width   int
	height  int
	lastErr string
	quit    bool
}

func Run(th theme.Theme, p *pet.Pet, r mood.Reader) error {
	_, err := tea.NewProgram(Model{th: th, pet: p, reader: r}).Run()
	return err
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second/theme.Fps, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) Init() tea.Cmd { return tickCmd() }

func (m Model) read(text string) tea.Cmd {
	r := m.reader
	return func() tea.Msg {
		reading, err := r.Read(mood.Event{Kind: "text", Text: text})
		return readMsg{reading, err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tickMsg:
		m.tick++
		if m.shown < len([]rune(m.line)) {
			m.shown += 2
		}
		return m, tickCmd()

	case readMsg:
		m.busy = false
		m.lastErr = ""
		if msg.err != nil {
			m.lastErr = msg.err.Error()
			m.line = m.th.Confused()
		} else if m.pet.Apply(msg.reading) {
			m.line = m.th.Say(m.pet.Mood)
		} else {
			m.line = m.th.Confused()
		}
		m.shown = 0
		_ = m.pet.Save()
		if len(m.queue) > 0 {
			next := m.queue[0]
			m.queue = m.queue[1:]
			m.busy = true
			return m, m.read(next)
		}

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			_ = m.pet.Save()
			m.quit = true
			return m, tea.Quit
		case tea.KeyEnter:
			text := strings.TrimSpace(m.input)
			m.input = ""
			if text == "" {
				return m, nil
			}
			if m.busy {
				m.queue = append(m.queue, text)
				return m, nil
			}
			m.busy = true
			return m, m.read(text)
		case tea.KeyBackspace:
			if r := []rune(m.input); len(r) > 0 {
				m.input = string(r[:len(r)-1])
			}
		case tea.KeySpace:
			m.input += " "
		case tea.KeyRunes:
			m.input += string(msg.Runes)
		}
	}
	return m, nil
}

var (
	bubbleStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("245")).
			Padding(0, 1)
	dim = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
)

func (m Model) View() string {
	if m.quit {
		return ""
	}
	canvas, pal := m.th.Frame(m.pet.Mood, m.tick)
	art := canvas.Render(pal)

	status := string(m.pet.Mood)
	switch {
	case m.busy:
		status += " · thinking" + strings.Repeat(".", m.tick/3%4)
	case m.lastErr != "":
		status += " · reader error"
	}

	pane := lipgloss.JoinHorizontal(lipgloss.Top, art, "  ", m.bubble())
	prompt := fmt.Sprintf("> %s%s", m.input, dim.Render("▏"))
	return pane + "\n" + dim.Render(status) + "\n" + prompt + "\n"
}

// bubble is the speech box beside the creature, padded to a fixed height so
// the layout never jumps as text appears.
func (m Model) bubble() string {
	const height = 12
	if m.line == "" {
		return strings.Repeat("\n", height-1)
	}
	runes := []rune(m.line)
	n := m.shown
	if n > len(runes) {
		n = len(runes)
	}
	box := bubbleStyle.Width(30).Render(string(runes[:n]))
	lines := strings.Split(box, "\n")
	for i := range lines {
		if i == len(lines)/2 {
			lines[i] = dim.Render("◂ ") + lines[i]
		} else {
			lines[i] = "  " + lines[i]
		}
	}
	top := "\n\n\n"
	return lipgloss.NewStyle().Height(height).Render(top + strings.Join(lines, "\n"))
}
