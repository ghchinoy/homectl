

package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	ea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ghchinoy/control/pkg/leap"
	"github.com/ghchinoy/control/pkg/sonos"
)

type sessionMode int

const (
	modeLights sessionMode = iota
	modeMusic
)

var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(1, 0)
	detailStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("241")),
		Padding(1, 2).
		MarginLeft(2)
	
	act iveTabStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")),
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color("205")),
		Padding(0, 1)
	inactiveTabStyle = lipgloss.NewStyle().
		Padding(0, 1)
)

type item struct {
	title, desc string
	zoneHref    string
	zoneName    string
	isAll       bool
	level       float64
	device      leap.Device
	
	// Sonos specific
	isSonos bool
	ip      string
	status  string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type model struct {
	mode        sessionMode
	lightsList  list.Model
	musicList   list.Model
	leapClient  *leap.Client
	err         error
	status      string
	progress    progress.Model
	width       int
	height      int
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.refreshLights(), m.refreshMusic())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			if m.mode == modeLights {
				m.mode = modeMusic
			} else {
				m.mode = modeLights
			}
			return m, nil
		case "+", "=":
			cmds = append(cmds, m.adjustLevel(10))
		case "-", "_":
			cmds = append(cmds, m.adjustLevel(-10))
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			level := float64((msg.String()[0] - '0') * 10)
			cmds = append(cmds, m.setLevel(level))
		case "0":
			cmds = append(cmds, m.setLevel(0))
		case "r":
			cmds = append(cmds, m.refreshLights(), m.refreshMusic())
		case " ": // Play/Pause for Sonos
			if m.mode == modeMusic {
				cmds = append(cmds, m.togglePlayback())
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		h, v := docStyle.GetFrameSize()
		listWidth := int(float64(msg.Width)*0.6) - h
		listHeight := msg.Height - v - 6
		m.lightsList.SetSize(listWidth, listHeight)
		m.musicList.SetSize(listWidth, listHeight)
	case statusMsg:
		m.status = string(msg)
	case refreshLightsMsg:
		for idx, itm := range m.lightsList.Items() {
			i := itm.(item)
			if lvl, ok := msg[i.zoneHref]; ok {
				i.level = lvl
				m.lightsList.SetItem(idx, i)
			}
		}
	case refreshMusicMsg:
		for idx, itm := range m.musicList.Items() {
			i := itm.(item)
			if stat, ok := msg[i.ip]; ok {
				i.level = float64(stat.volume)
				i.status = stat.status
				m.musicList.SetItem(idx, i)
			}
		}
	}

	var listCmd tea.Cmd
	if m.mode == modeLights {
		m.lightsList, listCmd = m.lightsList.Update(msg)
	} else {
		m.musicList, listCmd = m.musicList.Update(msg)
	}
	cmds = append(cmds, listCmd)

	return m, tea.Batch(cmds...)
}

type statusMsg string
type refreshLightsMsg map[string]float64
type musicStatus struct {
	volume int
	status string
}
type refreshMusicMsg map[string]musicStatus

func (m model) refreshLights() tea.Cmd {
	return func() tea.Msg {
		statuses, err := m.leapClient.GetAllZoneStatuses()
		if err != nil {
			return statusMsg(fmt.Sprintf("Lutron Refresh Error: %v", err))
		}
		results := make(refreshLightsMsg)
		for _, s := range statuses {
			results[s.Zone.Href] = s.Level
		}
		return results
	}
}

func (m model) refreshMusic() tea.Cmd {
	return func() tea.Msg {
		results := make(refreshMusicMsg)
		for _, itm := range m.musicList.Items() {
			i := itm.(item)
			client := sonos.NewClient(i.ip)
			vol, _ := client.GetVolume()
			info, _ := client.GetTransportInfo()
			results[i.ip] = musicStatus{volume: vol, status: info.CurrentTransportState}
		}
		return results
	}
}

func (m model) setLevel(level float64) tea.Cmd {
	var activeIdx int
	var itm list.Item
	
	if m.mode == modeLights {
		act iveIdx = m.lightsList.Index()
		itm = m.lightsList.SelectedItem()
	} else {
		act iveIdx = m.musicList.Index()
		itm = m.musicList.SelectedItem()
	}

	if itm == nil { return nil }
	i := itm.(item)
	i.level = level
	
	if m.mode == modeLights {
		m.lightsList.SetItem(activeIdx, i)
	} else {
		m.musicList.SetItem(activeIdx, i)
	}

	return func() tea.Msg {
		var err error
		if m.mode == modeLights {
			if i.isAll {
				err = m.leapClient.SetAllLevels(level)
			} else if i.zoneHref != "" {
				err = m.leapClient.SetLevel(i.zoneHref, level)
			}
		} else {
			client := sonos.NewClient(i.ip)
			err = client.SetVolume(int(level))
		}

		if err != nil {
			return statusMsg(fmt.Sprintf("Error: %v", err))
		}
		return statusMsg(fmt.Sprintf("Set %s to %.0f%%", i.title, level))
	}
}

func (m model) adjustLevel(delta float64) tea.Cmd {
	itm := m.getCurrentItem()
	if itm == nil {
		return nil
	}
	i := itm.(item)
	newLevel := i.level + delta
	if newLevel < 0 { newLevel = 0 }
	if newLevel > 100 { newLevel = 100 }
	return m.setLevel(newLevel)
}

func (m model) togglePlayback() tea.Cmd {
	itm := m.musicList.SelectedItem()
	if itm == nil { return nil }
	i := itm.(item)
	
	return func() tea.Msg {
		client := sonos.NewClient(i.ip)
		var err error
		if i.status == "PLAYING" {
			err = client.Pause()
		} else {
			err = client.Play()
		}
		if err != nil {
			return statusMsg(fmt.Sprintf("Playback error: %v", err))
		}
		return m.refreshMusic()()
	}
}

func (m model) getCurrentItem() list.Item {
	if m.mode == modeLights {
		return m.lightsList.SelectedItem()
	}
	return m.musicList.SelectedItem()
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	// Tabs
	lightsTab := inactiveTabStyle.Render("LIGHTS")
	musicTab := inactiveTabStyle.Render("MUSIC")
	if m.mode == modeLights {
		lightsTab = activeTabStyle.Render("LIGHTS")
	} else {
		musicTab = activeTabStyle.Render("MUSIC")
	}
	tabs := lipgloss.JoinHorizontal(lipgloss.Top, lightsTab, musicTab)

	// Main Content
	var currentList list.Model
	if m.mode == modeLights {
		currentList = m.lightsList
	} else {
		currentList = m.musicList
	}
	
	listView := docStyle.Render(currentList.View())
	
	// Details
	var detailView string
	selected := currentList.SelectedItem()
	if selected != nil {
		i := selected.(item)
		var detailText string
		if m.mode == modeLights {
			if i.isAll {
				detailText = "Master Control\n\nAffects all dimmable zones on the bridge."
			} else {
				detailText = fmt.Sprintf(
					"LIGHT DETAILS\n\nName: %s\nType: %s\nModel: %s\nZone: %s\nLevel: %.0f%%",
					i.title, i.device.DeviceType, i.device.ModelNumber, i.zoneName, i.level,
				)
			}
		} else {
			detailText = fmt.Sprintf(
				"SPEAKER DETAILS\n\nName: %s\nIP: %s\nStatus: %s\nVolume: %.0f%%",
				i.title, i.ip, i.status, i.level,
			)
		}
		detailView = detailStyle.
			Height(currentList.Height()).
			Width(int(float64(m.width)*0.3)).
			Render(detailText)
	}

	mainView := lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
	
	footer := ""
	if m.status != "" {
		footer += "\n" + statusStyle.Render(m.status)
	}
	
	controls := "1-9 (Level), 0 (Off), +/- (Adjust), Tab (Switch Mode), r (Refresh), q (Quit)"
	if m.mode == modeMusic {
		controls += ", Space (Play/Pause)"
	}
	footer += helpStyle.Render("\n" + controls)
	
	return tabs + "\n" + mainView + footer
}

type itemDelegate struct {
	progress progress.Model
}

func (d itemDelegate) Height() int                               { return 3 }
func (d itemDelegate) Spacing() int                              { return 1 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	itm, ok := listItem.(item)
	if !ok { return }

	str := fmt.Sprintf("%d. %s\n", index+1, itm.title)
	d.progress.Width = 30
	str += d.progress.ViewAs(itm.level / 100.0)

	style := lipgloss.NewStyle().PaddingLeft(2)
	if index == m.Index() {
		style = style.
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("205"))
	}

	fmt.Fprint(w, style.Render(str))
}

func Start(leapClient *leap.Client) error {
	// Initialize Lists
	prog := progress.New(progress.WithDefaultGradient())
	delegate := itemDelegate{progress: prog}

	// Fetch Data
	devices, _ := leapClient.GetDevices()
	zones, _ := leapClient.GetZones()
	zoneNames := make(map[string]string)
	for _, z := range zones {
		zoneNames[z.Href] = z.Name
	}

	// Lights Items
	lightItems := []list.Item{
		item{title: "ALL LIGHTS", desc: "Master control", isAll: true},
	}
	for _, d := range devices {
		if len(d.LocalZones) > 0 {
			zHref := d.LocalZones[0].Href
			lightItems = append(lightItems, item{
				title:    d.Name,
				desc:     fmt.Sprintf("%s (%s)", d.DeviceType, d.ModelNumber),
				zoneHref: zHref,
				zoneName: zoneNames[zHref],
				device:   d,
			})
		}
	}

	// Music Items (Hardcoded for now)
	musicItems := []list.Item{
		item{title: "TV Room", ip: "192.168.4.100", isSonos: true},
		item{title: "Move 2", ip: "192.168.4.120", isSonos: true},
		item{title: "Whole House", ip: "192.168.4.101", isSonos: true},
		item{title: "Office", ip: "192.168.4.99", isSonos: true},
	}

	m := model{
		mode:        modeLights,
		lightsList:  list.New(lightItems, delegate, 0, 0),
		musicList:   list.New(musicItems, delegate, 0, 0),
		leapClient:  leapClient,
		progress:    prog,
	}
	m.lightsList.Title = "Lutron Control"
	m.musicList.Title = "Sonos Control"

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
