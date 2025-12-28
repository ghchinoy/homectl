package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ghchinoy/control/pkg/config"
	"github.com/ghchinoy/control/pkg/leap"
	"github.com/ghchinoy/control/pkg/sonos"
)

type sessionMode int

const (
	modeLights sessionMode = iota
	modeMusic
	modeAreas
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
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type model struct {

	mode       sessionMode

	lightsList list.Model

	musicList  list.Model

	areasList  list.Model

	leapClient *leap.Client
	err        error
	status     string
	progress   progress.Model
	width      int
	height     int

	// Nickname editing
	textInput textinput.Model
	editing   bool
}

func nicknamesFile() string {
	return config.GetPath("nicknames.json")
}

func (m model) saveNicknames() {
	nicknames := make(map[string]string)
	for _, itm := range m.lightsList.Items() {
		i := itm.(item)
		if i.nickname != "" && i.zoneHref != "" {
			nicknames[i.zoneHref] = i.nickname
		}
	}
	for _, itm := range m.musicList.Items() {
		i := itm.(item)
		if i.nickname != "" && i.ip != "" {
			nicknames[i.ip] = i.nickname
		}
	}
	for _, itm := range m.areasList.Items() {
		i := itm.(item)
		if i.nickname != "" && i.zoneHref != "" {
			nicknames[i.zoneHref] = i.nickname
		}
	}

	data, _ := json.MarshalIndent(nicknames, "", "  ")
	os.WriteFile(nicknamesFile(), data, 0644)
}

func loadNicknames() map[string]string {
	data, err := os.ReadFile(nicknamesFile())
	if err != nil {
		return make(map[string]string)
	}
	var nicknames map[string]string
	json.Unmarshal(data, &nicknames)
	return nicknames
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.refreshLights(), m.refreshMusic())
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
				} else {
					activeIdx = m.areasList.Index()
					itm = m.areasList.SelectedItem()
				}

				if itm != nil {
					i := itm.(item)
					i.nickname = m.textInput.Value()
					if m.mode == modeLights {
						m.lightsList.SetItem(activeIdx, i)
					} else if m.mode == modeMusic {
						m.musicList.SetItem(activeIdx, i)
					} else {
						m.areasList.SetItem(activeIdx, i)
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
			cmds = append(cmds, m.refreshLights(), m.refreshMusic(), m.rediscoverMusic())
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
		areaItems := m.areasList.Items()
		for idx, itm := range areaItems {
			i := itm.(item)
			m.areasList.SetItem(idx, i)
		}
	case refreshMusicMsg:
		for idx, itm := range m.musicList.Items() {
			i := itm.(item)
			if stat, ok := msg[i.ip]; ok {
				i.level = float64(stat.volume)
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
		m.musicList.SetItems(msg)
	}

	var listCmd tea.Cmd
	if m.mode == modeLights {
		m.lightsList, listCmd = m.lightsList.Update(msg)
	} else if m.mode == modeMusic {
		m.musicList, listCmd = m.musicList.Update(msg)
	} else {
		m.areasList, listCmd = m.areasList.Update(msg)
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
			vol, _ := client.GetVolume()
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

type rediscoverMusicMsg []list.Item

func (m model) rediscoverMusic() tea.Cmd {
	return func() tea.Msg {
		var musicItems []list.Item
		speakers, _ := sonos.Discover(5 * time.Second)
		if len(speakers) > 0 {
			sonos.SaveCache(speakers)
		}
		for _, s := range speakers {
			musicItems = append(musicItems, item{
				title:    s.Name,
				ip:       s.IP,
				isSonos:  true,
				rinconID: s.RinconID,
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
	} else {
		activeIdx = m.areasList.Index()
		itm = m.areasList.SelectedItem()
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
	} else {
		m.areasList.SetItem(activeIdx, i)
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
		
		// If it was a bulk command, trigger a refresh to sync levels
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
	}
	return m.areasList.SelectedItem()
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	// Tabs with clear markers
	var lightsTab, musicTab, areasTab string
	if m.mode == modeLights {
		lightsTab = activeTabStyle.Render("[ LIGHTS ]")
		musicTab = inactiveTabStyle.Render("  MUSIC  ")
		areasTab = inactiveTabStyle.Render("  AREAS  ")
	} else if m.mode == modeMusic {
		lightsTab = inactiveTabStyle.Render("  LIGHTS ")
		musicTab = activeTabStyle.Render("[  MUSIC ]")
		areasTab = inactiveTabStyle.Render("  AREAS  ")
	} else {
		lightsTab = inactiveTabStyle.Render("  LIGHTS ")
		musicTab = inactiveTabStyle.Render("  MUSIC  ")
		areasTab = activeTabStyle.Render("[  AREAS ]")
	}
	tabsRow := lipgloss.JoinHorizontal(lipgloss.Top, lightsTab, musicTab, areasTab)
	tabs := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color("241")).
		Render(tabsRow)

	// Main Content
	var currentList list.Model
	if m.mode == modeLights {
		currentList = m.lightsList
	} else if m.mode == modeMusic {
		currentList = m.musicList
	} else {
		currentList = m.areasList
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
		} else if m.mode == modeMusic {
			detailText = fmt.Sprintf(
				"SPEAKER DETAILS\n\nName: %s\nIP: %s\nID: %s\nStatus: %s\nVolume: %.0f%%\nQueue: %d\n\nNOW PLAYING\n\nTrack:  %s\nArtist: %s\nAlbum:  %s",
				i.title, i.ip, i.rinconID, i.status, i.level, i.queueLen, i.trackTitle, i.artist, i.album,
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
		} else {
			detailText = fmt.Sprintf(
				"AREA DETAILS\n\nName: %s\nHref: %s",
				i.title, i.zoneHref,
			)
		}
		detailView = detailStyle.
			Height(currentList.Height()).
			Width(int(float64(m.width)*0.3)).
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
	nicknames := loadNicknames()

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
			title:    s.Name,
			ip:       s.IP,
			isSonos:  true,
			nickname: nicknames[s.IP],
			rinconID: s.RinconID,
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
		leapClient: leapClient,
		progress:   prog,
		textInput:  ti,
	}
	m.lightsList.Title = "Lutron Control"
	m.musicList.Title = "Sonos Control"
	m.areasList.Title = "Lutron Areas"
	m.lightsList.SetShowTitle(true)
	m.musicList.SetShowTitle(true)
	m.areasList.SetShowTitle(true)

	p := tea.NewProgram(m, tea.WithAltScreen())
	// Trigger discovery after startup
	go func() {
		speakers, _ := sonos.Discover(5 * time.Second)
		if len(speakers) > 0 {
			sonos.SaveCache(speakers)
			var items []list.Item
			for _, s := range speakers {
				items = append(items, item{
					title:    s.Name,
					ip:       s.IP,
					isSonos:  true,
					nickname: nicknames[s.IP],
					rinconID: s.RinconID,
				})
			}
			p.Send(rediscoverMusicMsg(items))
		}
	}()

	_, err = p.Run()
	return err
}