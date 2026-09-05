// Package tui provides an interactive terminal user interface built with Bubble Tea.
package tui

import (
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ghchinoy/homectl/pkg/config"
	"github.com/ghchinoy/homectl/pkg/leap"
	"github.com/ghchinoy/homectl/modules/sonos"
)

type sessionMode int

const (
	modeLights sessionMode = iota
	modeMusic
	modeAreas
	modeSonosGroups
)

var (
	docStyle    = lipgloss.NewStyle().Margin(1, 2)
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(1, 0)
	detailStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("241")).
			Padding(1, 2).
			MarginLeft(2)

	activeTabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("205")).
			Padding(0, 1).
			Bold(true)
	inactiveTabStyle = lipgloss.NewStyle().
				Padding(0, 1)
)

type item struct {
	title, desc string
	zoneHref    string
	zoneName    string
	isAll       bool
	isArea      bool
	level       float64
	device      leap.Device
	nickname    string

	// Sonos specific
	isSonos    bool
	ip         string
	status     string
	trackTitle string
	artist     string
	album      string
	stream     string
	format     string
	nextTrack  string
	queueLen   int
	rinconID   string
	modelName  string
	modelNum   string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type model struct {
	mode          sessionMode
	lightsList    list.Model
	musicList     list.Model
	areasList     list.Model
	groupsList    list.Model
	leapClient    *leap.Client
	sonosListener *sonos.GENAListener
	err           error
	status        string
	progress      progress.Model
	width         int
	height        int

	// Nickname editing
	textInput textinput.Model
	editing   bool
}

func (m model) saveNicknames() {
	nicknames := config.LoadNicknames()
	if nicknames == nil {
		nicknames = make(map[string]string)
	}
	for _, itm := range m.lightsList.Items() {
		if i, ok := itm.(item); ok && i.zoneHref != "" {
			if i.nickname != "" {
				nicknames[i.zoneHref] = i.nickname
			} else {
				delete(nicknames, i.zoneHref)
			}
		}
	}
	for _, itm := range m.musicList.Items() {
		if i, ok := itm.(item); ok && i.ip != "" {
			if i.nickname != "" {
				nicknames[i.ip] = i.nickname
			} else {
				delete(nicknames, i.ip)
			}
		}
	}
	for _, itm := range m.areasList.Items() {
		if i, ok := itm.(item); ok && i.zoneHref != "" {
			if i.nickname != "" {
				nicknames[i.zoneHref] = i.nickname
			} else {
				delete(nicknames, i.zoneHref)
			}
		}
	}
	_ = config.SaveNicknames(nicknames)
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.refreshLights(), m.refreshMusic(), m.refreshGroups())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.editing {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				m.editing = false
				var activeIdx int
				var itm list.Item
				if m.mode == modeLights {
					activeIdx = m.lightsList.Index()
					itm = m.lightsList.SelectedItem()
				} else if m.mode == modeMusic {
					activeIdx = m.musicList.Index()
					itm = m.musicList.SelectedItem()
				} else if m.mode == modeAreas {
					activeIdx = m.areasList.Index()
					itm = m.areasList.SelectedItem()
				} else {
					activeIdx = m.groupsList.Index()
					itm = m.groupsList.SelectedItem()
				}

				if itm != nil {
					i := itm.(item)
					i.nickname = m.textInput.Value()
					if m.mode == modeLights {
						m.lightsList.SetItem(activeIdx, i)
					} else if m.mode == modeMusic {
						m.musicList.SetItem(activeIdx, i)
					} else if m.mode == modeAreas {
						m.areasList.SetItem(activeIdx, i)
					} else {
						m.groupsList.SetItem(activeIdx, i)
					}
					m.saveNicknames()
				}
				return m, nil
			case "esc":
				m.editing = false
				return m, nil
			}
		}

		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			if m.mode == modeLights {
				m.mode = modeMusic
			} else if m.mode == modeMusic {
				m.mode = modeAreas
			} else if m.mode == modeAreas {
				m.mode = modeSonosGroups
			} else {
				m.mode = modeLights
			}
			return m, nil
		case "+", "=":
			cmds = append(cmds, m.adjustLevel(1))
		case "-", "_":
			cmds = append(cmds, m.adjustLevel(-1))
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			level := float64((msg.String()[0] - '0') * 10)
			cmds = append(cmds, m.setLevel(level))
		case "0":
			cmds = append(cmds, m.setLevel(0))
		case "r":
			cmds = append(cmds, m.refreshLights(), m.refreshMusic(), m.rediscoverMusic(), m.refreshGroups())
		case " ": // Play/Pause for Sonos
			if m.mode == modeMusic {
				cmds = append(cmds, m.togglePlayback())
			}
		case "n": // Next for Sonos
			if m.mode == modeMusic {
				cmds = append(cmds, m.nextTrack())
			}
		case "p": // Previous for Sonos
			if m.mode == modeMusic {
				cmds = append(cmds, m.prevTrack())
			}
		case "e": // Edit nickname
			itm := m.getCurrentItem()
			if itm != nil {
				i := itm.(item)
				if !i.isAll {
					m.editing = true
					m.textInput.SetValue(i.nickname)
					if m.textInput.Value() == "" {
						m.textInput.SetValue(i.title)
					}
					m.textInput.Focus()
					return m, textinput.Blink
				}
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		h, v := docStyle.GetFrameSize()
		listWidth := int(float64(msg.Width)*0.6) - h
		listHeight := msg.Height - v - 4
		m.lightsList.SetSize(listWidth, listHeight)
		m.musicList.SetSize(listWidth, listHeight)
		m.areasList.SetSize(listWidth, listHeight)
		m.groupsList.SetSize(listWidth, listHeight)
	case statusMsg:
		m.status = string(msg)
	case refreshLightsMsg:
		items := m.lightsList.Items()
		count := 0
		for idx, itm := range items {
			i := itm.(item)
			if lvl, ok := msg[i.zoneHref]; ok {
				i.level = lvl
				m.lightsList.SetItem(idx, i)
				count++
			}
		}
		m.status = fmt.Sprintf("Refreshed %d light statuses", count)
		// Also refresh Areas list
		areaItems := m.areasList.Items()
		for idx, itm := range areaItems {
			i := itm.(item)
			m.areasList.SetItem(idx, i)
		}
	case refreshMusicMsg:
		for idx, itm := range m.musicList.Items() {
			i := itm.(item)
			if stat, ok := msg[i.ip]; ok {
				if stat.volume >= 0 {
					i.level = float64(stat.volume)
				}
				i.status = stat.status
				i.trackTitle = stat.trackTitle
				i.artist = stat.artist
				i.album = stat.album
				i.stream = stat.stream
				i.format = stat.format
				i.nextTrack = stat.nextTrack
				i.queueLen = stat.queueLen
				m.musicList.SetItem(idx, i)
			}
		}
	case rediscoverMusicMsg:
		// Merge with existing items to preserve volume and metadata
		existingItems := m.musicList.Items()
		metadata := make(map[string]item)
		for _, itm := range existingItems {
			i := itm.(item)
			if i.ip != "" {
				metadata[i.ip] = i
			}
		}

		for idx, itm := range msg {
			i := itm.(item)
			if old, ok := metadata[i.ip]; ok {
				// Preserve existing state not provided by discovery
				i.level = old.level
				i.status = old.status
				i.trackTitle = old.trackTitle
				i.artist = old.artist
				i.album = old.album
				i.stream = old.stream
				i.format = old.format
				i.nextTrack = old.nextTrack
				i.queueLen = old.queueLen
				msg[idx] = i
			}
		}
		m.musicList.SetItems(msg)
	case sonos.EventMsg:
		// Handle real-time update from Sonos
		for idx, itm := range m.musicList.Items() {
			i := itm.(item)
			if i.ip == msg.IP {
				if msg.Volume >= 0 {
					i.level = float64(msg.Volume)
				}
				if msg.Status != "" {
					i.status = msg.Status
				}
				if msg.Metadata.Title != "" {
					i.trackTitle = msg.Metadata.Title
					i.artist = msg.Metadata.Artist
					i.album = msg.Metadata.Album
					i.stream = msg.Metadata.StreamContent
					i.format = msg.Metadata.AudioFormat
				}
				if msg.NextMetadata.Title != "" {
					i.nextTrack = fmt.Sprintf("%s by %s", msg.NextMetadata.Title, msg.NextMetadata.Artist)
				}
				m.musicList.SetItem(idx, i)
			}
		}
	case refreshGroupsMsg:
		m.groupsList.SetItems(msg)
	}

	var listCmd tea.Cmd
	if m.mode == modeLights {
		m.lightsList, listCmd = m.lightsList.Update(msg)
	} else if m.mode == modeMusic {
		m.musicList, listCmd = m.musicList.Update(msg)
	} else if m.mode == modeAreas {
		m.areasList, listCmd = m.areasList.Update(msg)
	} else {
		m.groupsList, listCmd = m.groupsList.Update(msg)
	}
	cmds = append(cmds, listCmd)

	return m, tea.Batch(cmds...)
}

type statusMsg string
type refreshLightsMsg map[string]float64
type musicStatus struct {
	volume     int
	status     string
	trackTitle string
	artist     string
	album      string
	stream     string
	format     string
	nextTrack  string
	queueLen   int
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
			vol, err := client.GetVolume()
			if err != nil {
				vol = -1
			}
			transport, _ := client.GetTransportInfo()
			pos, _ := client.GetPositionInfo()
			media, _ := client.GetMediaInfo()

			meta, _ := client.ParseTrackMetadata(pos.TrackMetaData)
			nextMeta, _ := client.ParseTrackMetadata(media.NextURIMetaData)

			nextStr := ""
			if nextMeta.Title != "" {
				nextStr = fmt.Sprintf("%s by %s", nextMeta.Title, nextMeta.Artist)
			}

			results[i.ip] = musicStatus{
				volume:     vol,
				status:     transport.CurrentTransportState,
				trackTitle: meta.Title,
				artist:     meta.Artist,
				album:      meta.Album,
				stream:     meta.StreamContent,
				format:     meta.AudioFormat,
				nextTrack:  nextStr,
				queueLen:   media.NrTracks,
			}
		}
		return results
	}
}

type refreshGroupsMsg []list.Item

func (m model) refreshGroups() tea.Cmd {
	return func() tea.Msg {
		var groupItems []list.Item
		speakers, _ := sonos.LoadCache()
		if len(speakers) == 0 {
			return refreshGroupsMsg(groupItems)
		}

		client := sonos.NewClient(speakers[0].IP)
		state, err := client.GetZoneGroupState()
		if err != nil {
			return statusMsg(fmt.Sprintf("Groups Error: %v", err))
		}

		for _, g := range state.Groups {
			members := ""
			groupName := ""
			for _, mm := range g.Members {
				if mm.UUID == g.Coordinator {
					groupName = mm.RoomName
				}
				if members != "" {
					members += ", "
				}
				members += mm.RoomName
			}

			if len(g.Members) > 1 {
				groupName = fmt.Sprintf("%s + %d", groupName, len(g.Members)-1)
			}

			groupItems = append(groupItems, item{
				title:    groupName,
				desc:     members,
				isSonos:  true,
				zoneHref: g.ID,
			})
		}
		return refreshGroupsMsg(groupItems)
	}
}

type rediscoverMusicMsg []list.Item

func (m model) rediscoverMusic() tea.Cmd {
	// Capture current volumes to prevent reset
	volumes := make(map[string]float64)
	for _, itm := range m.musicList.Items() {
		i := itm.(item)
		if i.ip != "" {
			volumes[i.ip] = i.level
		}
	}

	return func() tea.Msg {
		var musicItems []list.Item
		speakers, _ := sonos.Discover(5 * time.Second)
		if len(speakers) > 0 {
			sonos.SaveCache(speakers)
		}
		// Load potentially merged cache
		speakers, _ = sonos.LoadCache()
		for _, s := range speakers {
			level := 0.0
			if v, ok := volumes[s.IP]; ok {
				level = v
			}
			musicItems = append(musicItems, item{
				title:     s.Name,
				ip:        s.IP,
				isSonos:   true,
				rinconID:  s.RinconID,
				modelName: s.ModelName,
				modelNum:  s.ModelNumber,
				level:     level,
			})
		}
		return rediscoverMusicMsg(musicItems)
	}
}

func (m model) setLevel(level float64) tea.Cmd {
	var activeIdx int
	var itm list.Item

	if m.mode == modeLights {
		activeIdx = m.lightsList.Index()
		itm = m.lightsList.SelectedItem()
	} else if m.mode == modeMusic {
		activeIdx = m.musicList.Index()
		itm = m.musicList.SelectedItem()
	} else if m.mode == modeAreas {
		activeIdx = m.areasList.Index()
		itm = m.areasList.SelectedItem()
	} else {
		activeIdx = m.groupsList.Index()
		itm = m.groupsList.SelectedItem()
	}

	if itm == nil {
		return nil
	}
	i := itm.(item)
	i.level = level

	if m.mode == modeLights {
		m.lightsList.SetItem(activeIdx, i)
	} else if m.mode == modeMusic {
		m.musicList.SetItem(activeIdx, i)
	} else if m.mode == modeAreas {
		m.areasList.SetItem(activeIdx, i)
	} else {
		m.groupsList.SetItem(activeIdx, i)
	}

	return func() tea.Msg {
		var err error
		if m.mode == modeLights {
			if i.isAll {
				err = m.leapClient.SetAllLevels(level)
			} else if i.zoneHref != "" {
				err = m.leapClient.SetLevel(i.zoneHref, level)
			}
		} else if m.mode == modeMusic {
			client := sonos.NewClient(i.ip)
			err = client.SetVolume(int(level))
		} else {
			if i.zoneHref != "" {
				err = m.leapClient.SetAreaLevel(i.zoneHref, level)
			}
		}

		if err != nil {
			return statusMsg(fmt.Sprintf("Error: %v", err))
		}

		if m.mode == modeAreas || (m.mode == modeLights && i.isAll) {
			return m.refreshLights()()
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
	if newLevel < 0 {
		newLevel = 0
	}
	if newLevel > 100 {
		newLevel = 100
	}
	return m.setLevel(newLevel)
}

func (m model) togglePlayback() tea.Cmd {
	itm := m.musicList.SelectedItem()
	if itm == nil {
		return nil
	}
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

func (m model) nextTrack() tea.Cmd {
	itm := m.musicList.SelectedItem()
	if itm == nil {
		return nil
	}
	i := itm.(item)

	return func() tea.Msg {
		client := sonos.NewClient(i.ip)
		if err := client.Next(); err != nil {
			return statusMsg(fmt.Sprintf("Next error: %v", err))
		}
		return m.refreshMusic()()
	}
}

func (m model) prevTrack() tea.Cmd {
	itm := m.musicList.SelectedItem()
	if itm == nil {
		return nil
	}
	i := itm.(item)

	return func() tea.Msg {
		client := sonos.NewClient(i.ip)
		if err := client.Previous(); err != nil {
			return statusMsg(fmt.Sprintf("Previous error: %v", err))
		}
		return m.refreshMusic()()
	}
}

func (m model) getCurrentItem() list.Item {
	if m.mode == modeLights {
		return m.lightsList.SelectedItem()
	} else if m.mode == modeMusic {
		return m.musicList.SelectedItem()
	} else if m.mode == modeAreas {
		return m.areasList.SelectedItem()
	}
	return m.groupsList.SelectedItem()
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	// Tabs with clear markers
	var lightsTab, musicTab, areasTab, groupsTab string
	if m.mode == modeLights {
		lightsTab = activeTabStyle.Render("[ LIGHTS ]")
		musicTab = inactiveTabStyle.Render("  MUSIC  ")
		areasTab = inactiveTabStyle.Render("  AREAS  ")
		groupsTab = inactiveTabStyle.Render("  GROUPS ")
	} else if m.mode == modeMusic {
		lightsTab = inactiveTabStyle.Render("  LIGHTS ")
		musicTab = activeTabStyle.Render("[  MUSIC ]")
		areasTab = inactiveTabStyle.Render("  AREAS  ")
		groupsTab = inactiveTabStyle.Render("  GROUPS ")
	} else if m.mode == modeAreas {
		lightsTab = inactiveTabStyle.Render("  LIGHTS ")
		musicTab = inactiveTabStyle.Render("  MUSIC  ")
		areasTab = activeTabStyle.Render("[  AREAS ]")
		groupsTab = inactiveTabStyle.Render("  GROUPS ")
	} else {
		lightsTab = inactiveTabStyle.Render("  LIGHTS ")
		musicTab = inactiveTabStyle.Render("  MUSIC  ")
		areasTab = inactiveTabStyle.Render("  AREAS  ")
		groupsTab = activeTabStyle.Render("[  GROUPS ]")
	}
	tabsRow := lipgloss.JoinHorizontal(lipgloss.Top, lightsTab, musicTab, areasTab, groupsTab)
	tabs := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color("241")).
		Render(tabsRow)

	var currentList list.Model
	if m.mode == modeLights {
		currentList = m.lightsList
	} else if m.mode == modeMusic {
		currentList = m.musicList
	} else if m.mode == modeAreas {
		currentList = m.areasList
	} else {
		currentList = m.groupsList
	}

	listView := docStyle.Render(currentList.View())

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
		} else if m.mode == modeMusic {
			detailText = fmt.Sprintf(
				"SPEAKER DETAILS\n\nName: %s\nIP: %s\nModel: %s (%s)\nID: %s\nStatus: %s\nVolume: %.0f%%\nQueue: %d\n\nNOW PLAYING\n\nTrack:  %s\nArtist: %s\nAlbum:  %s",
				i.title, i.ip, i.modelName, i.modelNum, i.rinconID, i.status, i.level, i.queueLen, i.trackTitle, i.artist, i.album,
			)
			if i.stream != "" {
				detailText += fmt.Sprintf("\nStream: %s", i.stream)
			}
			if i.format != "" {
				detailText += fmt.Sprintf("\nFormat: %s", i.format)
			}
			if i.nextTrack != "" {
				detailText += fmt.Sprintf("\n\nNEXT\n%s", i.nextTrack)
			}
		} else if m.mode == modeAreas {
			detailText = fmt.Sprintf(
				"AREA DETAILS\n\nName: %s\nHref: %s",
				i.title, i.zoneHref,
			)
		} else {
			detailText = fmt.Sprintf(
				"SONOS GROUP\n\nID: %s\nMembers: %s",
				i.zoneHref, i.desc,
			)
		}
		detailView = detailStyle.
			Height(currentList.Height()).
			Width(int(float64(m.width) * 0.3)).
			Render(detailText)
	}

	mainView := lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)

	if m.editing {
		mainView = lipgloss.JoinVertical(lipgloss.Left,
			mainView,
			lipgloss.NewStyle().Margin(1, 2).Render("Edit Nickname: "+m.textInput.View()),
		)
	}

	footer := ""
	if m.status != "" {
		footer += "\n" + statusStyle.Render(m.status)
	}

	if m.mode == modeMusic && m.sonosListener != nil {
		footer += fmt.Sprintf("\nEvent Callback: %s", m.sonosListener.GetLocalIP())
		footer += fmt.Sprintf("\nLog Path: %s", config.GetPath("sonos.log"))
	}

	controls := "1-9 (Level), 0 (Off), +/- (Adjust), Tab (Switch Mode), r (Refresh), q (Quit)"
	if m.mode == modeMusic {
		controls += ", Space (P/P), n (Next), p (Prev)"
	}
	controls += ", e (Edit Nickname)"
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
	if !ok {
		return
	}

	displayName := itm.title
	if itm.nickname != "" {
		displayName = itm.nickname
	}

	str := fmt.Sprintf("%d. %s\n", index+1, displayName)
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
	nicknames := config.LoadNicknames()

	// Fetch Data
	devices, err := leapClient.GetDevices()
	if err != nil {
		return fmt.Errorf("failed to get devices: %w", err)
	}
	zones, err := leapClient.GetZones()
	if err != nil {
		return fmt.Errorf("failed to get zones: %w", err)
	}
	areas, err := leapClient.GetAreas()
	if err != nil {
		return fmt.Errorf("failed to get areas: %w", err)
	}
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
				nickname: nicknames[zHref],
			})
		}
	}

	// Music Items (loaded from cache, then discovered)
	musicItems := []list.Item{}
	cached, _ := sonos.LoadCache()
	for _, s := range cached {
		musicItems = append(musicItems, item{
			title:     s.Name,
			ip:        s.IP,
			isSonos:   true,
			nickname:  nicknames[s.IP],
			rinconID:  s.RinconID,
			modelName: s.ModelName,
			modelNum:  s.ModelNumber,
		})
	}

	// Areas Items
	areaItems := []list.Item{}
	for _, a := range areas {
		areaItems = append(areaItems, item{
			title:    a.Name,
			zoneHref: a.Href,
			isArea:   true,
			nickname: nicknames[a.Href],
		})
	}

	ti := textinput.New()
	ti.Placeholder = "Nickname"
	ti.CharLimit = 32
	ti.Width = 30

	m := model{
		mode:       modeLights,
		lightsList: list.New(lightItems, delegate, 0, 0),
		musicList:  list.New(musicItems, delegate, 0, 0),
		areasList:  list.New(areaItems, delegate, 0, 0),
		groupsList: list.New([]list.Item{}, delegate, 0, 0),
		leapClient: leapClient,
		progress:   prog,
		textInput:  ti,
	}
	m.lightsList.Title = "Lutron Control"
	m.musicList.Title = "Sonos Control"
	m.areasList.Title = "Lutron Areas"
	m.groupsList.Title = "Sonos Groups"
	m.lightsList.SetShowTitle(true)
	m.musicList.SetShowTitle(true)
	m.areasList.SetShowTitle(true)
	m.groupsList.SetShowTitle(true)

	p := tea.NewProgram(m, tea.WithAltScreen())

	// Initialize GENA listener

	listener := &sonos.GENAListener{

		Handler: func(event sonos.EventMsg) {

			p.Send(event)

		},
	}

	callbackURL, err := listener.Start()

	if err == nil {

		m.sonosListener = listener

		// Initial subscriptions for cached items

		go func() {

			for _, itm := range musicItems {

				i := itm.(item)

				if i.ip != "" {

					client := sonos.NewClient(i.ip)

					client.Subscribe("/MediaRenderer/AVTransport/Event", callbackURL, 300)

					client.Subscribe("/MediaRenderer/RenderingControl/Event", callbackURL, 300)

				}

			}

		}()

	}

	// Trigger discovery after startup

	go func() {

		speakers, _ := sonos.Discover(5 * time.Second)

		if len(speakers) > 0 {

			sonos.SaveCache(speakers)

			var items []list.Item

			merged, _ := sonos.LoadCache()

			for _, s := range merged {

				if callbackURL != "" {

					client := sonos.NewClient(s.IP)

					client.Subscribe("/MediaRenderer/AVTransport/Event", callbackURL, 300)

					client.Subscribe("/MediaRenderer/RenderingControl/Event", callbackURL, 300)

				}

				items = append(items, item{

					title: s.Name,

					ip: s.IP,

					desc: fmt.Sprintf("%s (%s)", s.ModelName, s.ModelNumber),

					nickname: nicknames[s.IP],
				})

			}

			p.Send(musicItemsMsg(items))

		}

	}()

	_, err = p.Run()
	return err
}

type musicItemsMsg []list.Item
