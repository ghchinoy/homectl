package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ghchinoy/control/pkg/leap"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type item struct {
	title, desc string
	zoneHref    string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type model struct {
	list   list.Model
	client *leap.Client
	err    error
	status string
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "+", "=":
			return m, m.setLevel(100)
		case "-", "_":
			return m, m.setLevel(0)
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			level := float64((msg.String()[0] - '0') * 10)
			return m, m.setLevel(level)
		case "0":
			return m, m.setLevel(0)
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	case statusMsg:
		m.status = string(msg)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

type statusMsg string

func (m model) setLevel(level float64) tea.Cmd {
	i, ok := m.list.SelectedItem().(item)
	if !ok || i.zoneHref == "" {
		return nil
	}

	return func() tea.Msg {
		err := m.client.SetLevel(i.zoneHref, level)
		if err != nil {
			return statusMsg(fmt.Sprintf("Error setting level: %v", err))
		}
		return statusMsg(fmt.Sprintf("Set %s to %.0f%%", i.title, level))
	}
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}
	
s := docStyle.Render(m.list.View())
	if m.status != "" {
		s += "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(m.status)
	}
	s += "\n\nControls: 1-9 (10-90%), 0 (Off), + (Full), q (Quit)"
	return s
}

// Start launches the TUI
func Start(client *leap.Client) error {

devices, err := client.GetDevices()
	if err != nil {
		return err
	}

	items := []list.Item{}
	for _, d := range devices {
		var zoneHref string
		if len(d.LocalZones) > 0 {
			zoneHref = d.LocalZones[0].Href
		}

		items = append(items, item{
			title:    d.Name,
			desc:     fmt.Sprintf("%s (%s)", d.DeviceType, d.ModelNumber),
			zoneHref: zoneHref,
		})
	}

	m := model{
		list:   list.New(items, list.NewDefaultDelegate(), 0, 0),
		client: client,
	}
	m.list.Title = "Lutron Devices"

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}