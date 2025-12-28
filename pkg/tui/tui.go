

package tui

import (
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ghchinoy/control/pkg/leap"
)

var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(1, 0)
)

type item struct {
	title, desc string
	zoneHref    string
	isAll       bool
	level       float64
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type model struct {
	list     list.Model
	client   *leap.Client
	err      error
	status   string
	progress progress.Model
}

func (m model) Init() tea.Cmd {
	return m.refreshStatus()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "+", "=":
			cmds = append(cmds, m.setLevel(100))
		case "-", "_":
			cmds = append(cmds, m.setLevel(0))
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			level := float64((msg.String()[0] - '0') * 10)
			cmds = append(cmds, m.setLevel(level))
		case "0":
			cmds = append(cmds, m.setLevel(0))
		case "r":
			cmds = append(cmds, m.refreshStatus())
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	case statusMsg:
		m.status = string(msg)
	case refreshMsg:
		for idx, lvl := range msg {
			if idx < len(m.list.Items()) {
				itm := m.list.Items()[idx].(item)
				itm.level = lvl
				m.list.SetItem(idx, itm)
			}
		}
		cmds = append(cmds, tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
			return m.refreshStatus()()
		}))
	}

	var listCmd tea.Cmd
	m.list, listCmd = m.list.Update(msg)
	cmds = append(cmds, listCmd)

	return m, tea.Batch(cmds...)
}

type statusMsg string
type refreshMsg map[int]float64

func (m model) refreshStatus() tea.Cmd {
	return func() tea.Msg {
		results := make(refreshMsg)
		for idx, itm := range m.list.Items() {
			i := itm.(item)
			if i.zoneHref != "" {
				status, err := m.client.GetZoneStatus(i.zoneHref)
				if err == nil {
					results[idx] = status.Level
				}
			}
		}
		return results
	}
}

func (m model) setLevel(level float64) tea.Cmd {
	i, ok := m.list.SelectedItem().(item)
	if !ok {
		return nil
	}

	return func() tea.Msg {
		var err error
		if i.isAll {
			err = m.client.SetAllLevels(level)
		} else if i.zoneHref != "" {
			err = m.client.SetLevel(i.zoneHref, level)
		} else {
			return nil
		}

		if err != nil {
			return statusMsg(fmt.Sprintf("Error: %v", err))
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
		s += "\n" + statusStyle.Render(m.status)
	}
	s += helpStyle.Render("\nControls: 1-9 (10-90%), 0 (Off), + (Full), r (Refresh), q (Quit)")
	return s
}

type itemDelegate struct {
	progress progress.Model
}

func (d itemDelegate) Height() int                               { return 3 }
func (d itemDelegate) Spacing() int                              { return 1 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	itm, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s\n", index+1, itm.title)
	if itm.zoneHref != "" || itm.isAll {
		d.progress.Width = 30
		str += d.progress.ViewAs(itm.level / 100.0)
	} else {
		str += lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(itm.desc)
	}

	style := lipgloss.NewStyle().PaddingLeft(2)
	if index == m.Index() {
		style = style.
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("205"))
	}

	fmt.Fprintf(w, style.Render(str))
}

func Start(client *leap.Client) error {
	devices, err := client.GetDevices()
	if err != nil {
		return err
	}

	prog := progress.New(progress.WithDefaultGradient())
	
	items := []list.Item{
		item{title: "ALL LIGHTS", desc: "Master control", isAll: true},
	}
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
		list:     list.New(items, itemDelegate{progress: prog}, 0, 0),
		client:   client,
		progress: prog,
	}
	m.list.Title = "Lutron Control"

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
