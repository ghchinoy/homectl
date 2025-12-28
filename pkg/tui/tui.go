
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
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type model struct {
	list   list.Model
	client *leap.Client
	err    error
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}
	return docStyle.Render(m.list.View())
}

// Start launches the TUI
func Start(client *leap.Client) error {
	devices, err := client.GetDevices()
	if err != nil {
		return err
	}

	items := []list.Item{}
	for _, d := range devices {
		items = append(items, item{
			title: d.Name,
			desc:  fmt.Sprintf("%s (%s)", d.DeviceType, d.ModelNumber),
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
