//go:build darwin || windows || linux

package headless

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type applicationWindow struct {
	fullscreen        atomic.Bool
	fullscreenRequest atomic.Int32
	invalidator       *app.Invalidator
	id                uint64
	title             string
	panel             bool
	hidden            bool
	session           *Session
	closed            func()
	// closeRequest is the application's OnCloseRequest, if it set one.
	closeRequest func(allow func())
}

// applicationStatusItem is one status item of a headless application.
type applicationStatusItem struct {
	id      uint64
	tooltip string
	icon    app.StatusIcon
	onClick func(app.StatusClick)
	menu    func() []app.MenuItem
	removed bool
}

type applicationSession struct {
	mu       sync.Mutex
	options  Options
	nextID   uint64
	active   uint64
	windows  map[uint64]*applicationWindow
	items    map[uint64]*applicationStatusItem
	nextItem uint64
	quit     bool
	// badge is the label the Dock was asked to show beside the icon, which
	// SetDockBadge records and the "dockbadge" command reports.
	badge string
	// openURL receives the links the "openurl" command delivers, as the system
	// would deliver a registered scheme's link.
	openURL func(string)
}

func newApplicationSession(options Options) *applicationSession {
	return &applicationSession{
		options: options, nextID: 1, nextItem: 1,
		windows: make(map[uint64]*applicationWindow), items: make(map[uint64]*applicationStatusItem),
	}
}

// OpenWindow implements app.Host.
func (application *applicationSession) OpenWindow(config app.Config, canvas *ui.Canvas, closed func()) (app.WindowControl, error) {
	entry := &applicationWindow{}
	if config.Invalidator == nil {
		config.Invalidator = &app.Invalidator{}
	}
	entry.invalidator = config.Invalidator
	onWake, onFullscreen := config.OnWake, config.OnFullscreen
	config.OnFullscreen = func(fullscreen bool) {
		entry.fullscreen.Store(fullscreen)
		if onFullscreen != nil {
			onFullscreen(fullscreen)
		}
	}
	config.OnWake = func() {
		if request := entry.fullscreenRequest.Swap(0); request != 0 {
			if err := entry.session.driver.SetFullscreen(request == 2); err != nil {
				panic(err)
			}
		}
		if onWake != nil {
			onWake()
		}
	}
	session, err := Open(config, canvas, application.options)
	if err != nil {
		return nil, err
	}
	application.mu.Lock()
	id := application.nextID
	application.nextID++
	entry.id, entry.title, entry.panel, entry.session, entry.closed, entry.closeRequest = id, config.Title, config.WindowMode == app.WindowPanel, session, closed, config.OnCloseRequest
	application.windows[id] = entry
	application.active = id
	application.mu.Unlock()
	return &headlessWindow{application: application, id: id}, nil
}

// headlessWindow is the app.WindowControl of one headless window.
type headlessWindow struct {
	application *applicationSession
	id          uint64
	once        sync.Once
}

func (window *headlessWindow) Close() { window.once.Do(func() { window.application.close(window.id) }) }
func (window *headlessWindow) Hide()  { window.application.setHidden(window.id, true) }

// Frame is the window's content area where its Config.Origin put it.
func (window *headlessWindow) Frame() (ui.Rect, bool) {
	window.application.mu.Lock()
	entry := window.application.windows[window.id]
	window.application.mu.Unlock()
	if entry == nil {
		return ui.Rect{}, false
	}
	return entry.session.driver.Frame(), true
}
func (window *headlessWindow) Show() { window.application.setHidden(window.id, false) }

func (window *headlessWindow) SetFullscreen(fullscreen bool) error {
	window.application.mu.Lock()
	entry := window.application.windows[window.id]
	window.application.mu.Unlock()
	if entry == nil {
		return fmt.Errorf("headless: window is closed")
	}
	request := int32(1)
	if fullscreen {
		request = 2
	}
	entry.fullscreenRequest.Store(request)
	entry.invalidator.Invalidate()
	return nil
}

func (window *headlessWindow) Fullscreen() bool {
	window.application.mu.Lock()
	entry := window.application.windows[window.id]
	window.application.mu.Unlock()
	return entry != nil && entry.fullscreen.Load()
}

func (application *applicationSession) setHidden(id uint64, hidden bool) {
	application.mu.Lock()
	defer application.mu.Unlock()
	if window := application.windows[id]; window != nil {
		window.hidden = hidden
		if !hidden {
			application.active = id
		}
	}
}

// AddStatusItem implements app.Host.
func (application *applicationSession) AddStatusItem(config app.StatusItemConfig) (app.StatusItemControl, error) {
	application.mu.Lock()
	defer application.mu.Unlock()
	id := application.nextItem
	application.nextItem++
	application.items[id] = &applicationStatusItem{id: id, tooltip: config.Tooltip, icon: config.Icon, onClick: config.OnClick, menu: config.Menu}
	return &headlessStatusItem{application: application, id: id}, nil
}

// SetOpenURLHandler implements the optional host capability that passes
// registered-scheme links to the application.
func (application *applicationSession) SetOpenURLHandler(handler func(string)) {
	application.mu.Lock()
	application.openURL = handler
	application.mu.Unlock()
}

// SetDockBadge implements app.Host: it keeps the label the Dock was asked to
// show, which the "dockbadge" command reports.
func (application *applicationSession) SetDockBadge(text string) {
	application.mu.Lock()
	application.badge = text
	application.mu.Unlock()
}

// Quit implements app.Host: every window closes and the protocol loop ends.
func (application *applicationSession) Quit() {
	application.mu.Lock()
	application.quit = true
	application.mu.Unlock()
	application.closeAll()
}

func (application *applicationSession) quitRequested() bool {
	application.mu.Lock()
	defer application.mu.Unlock()
	return application.quit
}

type headlessStatusItem struct {
	application *applicationSession
	id          uint64
}

func (item *headlessStatusItem) SetIcon(icon app.StatusIcon) error {
	item.application.mu.Lock()
	defer item.application.mu.Unlock()
	if native := item.application.items[item.id]; native != nil {
		native.icon = icon
	}
	return nil
}

func (item *headlessStatusItem) SetTooltip(tooltip string) {
	item.application.mu.Lock()
	defer item.application.mu.Unlock()
	if native := item.application.items[item.id]; native != nil {
		native.tooltip = tooltip
	}
}

func (item *headlessStatusItem) Remove() {
	item.application.mu.Lock()
	defer item.application.mu.Unlock()
	delete(item.application.items, item.id)
}

// clickStatusItem presses a status item the way the platform would: the click
// handler runs, here on the caller's goroutine.
func (application *applicationSession) clickStatusItem(id uint64, button ui.PointerButton) error {
	application.mu.Lock()
	item := application.items[id]
	application.mu.Unlock()
	if item == nil {
		return fmt.Errorf("headless: no status item %d", id)
	}
	// With a menu the secondary press opens it, and so does the primary unless
	// the application handles it, as a Linux desktop's tray reports it.
	if item.menu != nil && (button != ui.PointerButtonPrimary || item.onClick == nil) {
		return fmt.Errorf("headless: status item %d opens a menu; use statusmenu and statusselect", id)
	}
	if item.onClick != nil {
		item.onClick(app.NewStatusClick(button))
	}
	return nil
}

// statusMenu is what a status item's menu holds at this moment, as the platform
// would build it when the menu opens.
func (application *applicationSession) statusMenu(id uint64) ([]app.MenuItem, error) {
	application.mu.Lock()
	item := application.items[id]
	application.mu.Unlock()
	if item == nil {
		return nil, fmt.Errorf("headless: no status item %d", id)
	}
	if item.menu == nil {
		return nil, fmt.Errorf("headless: status item %d has no menu", id)
	}
	return item.menu(), nil
}

// selectStatusMenu chooses the entry at path (1-based indexes joined by dots)
// in a status item's menu, as a click on it would.
func (application *applicationSession) selectStatusMenu(id uint64, path string) error {
	items, err := application.statusMenu(id)
	if err != nil {
		return err
	}
	var chosen *app.MenuItem
	for _, part := range strings.Split(path, ".") {
		index, err := strconv.Atoi(part)
		if err != nil || index < 1 || index > len(items) {
			return fmt.Errorf("headless: no menu entry %q", path)
		}
		chosen = &items[index-1]
		items = chosen.Submenu
	}
	switch {
	case chosen.Separator || chosen.Disabled || len(chosen.Submenu) > 0 || chosen.OnSelect == nil:
		return fmt.Errorf("headless: menu entry %q cannot be chosen", path)
	}
	chosen.OnSelect()
	return nil
}

// writeStatusMenu lists a menu one entry per line, indented by depth.
func writeStatusMenu(reply *strings.Builder, items []app.MenuItem, prefix string) {
	for index, item := range items {
		path := prefix + strconv.Itoa(index+1)
		depth := strings.Count(path, ".")
		indent := strings.Repeat("  ", depth)
		if item.Separator {
			fmt.Fprintf(reply, "%spath=%s separator\n", indent, path)
			continue
		}
		fmt.Fprintf(reply, "%spath=%s title=%q disabled=%t checked=%t submenu=%t\n", indent, path, item.Title, item.Disabled, item.Checked, len(item.Submenu) > 0)
		writeStatusMenu(reply, item.Submenu, path+".")
	}
}

// blurWindow takes keyboard focus from a window. A panel closes itself then, as
// it does on a desktop.
func (application *applicationSession) blurWindow(id uint64) error {
	application.mu.Lock()
	window := application.windows[id]
	application.mu.Unlock()
	if window == nil {
		return fmt.Errorf("headless: no window %d", id)
	}
	if window.panel {
		application.close(id)
		return nil
	}
	return window.session.LoseFocus()
}

func (application *applicationSession) close(id uint64) {
	application.mu.Lock()
	window := application.windows[id]
	if window == nil {
		application.mu.Unlock()
		return
	}
	delete(application.windows, id)
	if application.active == id {
		application.active = 0
		for candidate := range application.windows {
			if application.active == 0 || candidate < application.active {
				application.active = candidate
			}
		}
	}
	application.mu.Unlock()
	window.session.Close()
	window.closed()
}

func (application *applicationSession) closeAll() {
	application.mu.Lock()
	ids := make([]uint64, 0, len(application.windows))
	for id := range application.windows {
		ids = append(ids, id)
	}
	application.mu.Unlock()
	for _, id := range ids {
		application.close(id)
	}
}

func (application *applicationSession) current() (*applicationWindow, bool) {
	application.mu.Lock()
	defer application.mu.Unlock()
	window := application.windows[application.active]
	return window, window != nil
}

func (application *applicationSession) count() int {
	application.mu.Lock()
	defer application.mu.Unlock()
	return len(application.windows)
}

func (application *applicationSession) selectWindow(id uint64) error {
	application.mu.Lock()
	defer application.mu.Unlock()
	if application.windows[id] == nil {
		return fmt.Errorf("headless: no window %d", id)
	}
	application.active = id
	return nil
}

type applicationWindowDescription struct {
	id            uint64
	title         string
	panel, hidden bool
}

func (application *applicationSession) describe() (uint64, []applicationWindowDescription) {
	application.mu.Lock()
	defer application.mu.Unlock()
	windows := make([]applicationWindowDescription, 0, len(application.windows))
	for _, window := range application.windows {
		windows = append(windows, applicationWindowDescription{id: window.id, title: window.title, panel: window.panel, hidden: window.hidden})
	}
	sort.Slice(windows, func(left, right int) bool { return windows[left].id < windows[right].id })
	return application.active, windows
}

// RunApplication is the app.ApplicationRunner installed by headless/auto. It
// preserves every window as a separate Driver and exposes explicit selection
// and close commands while reusing the ordinary single-window protocol.
func RunApplication(start func(host app.Host) error) error {
	options := Options{}
	if value := os.Getenv("VIVA_HEADLESS_SCALE"); value != "" {
		scale, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("headless: VIVA_HEADLESS_SCALE=%q is not a number", value)
		}
		options.Scale = scale
	}
	application := newApplicationSession(options)
	if err := start(application); err != nil {
		application.closeAll()
		return err
	}
	defer application.closeAll()
	active, windows := application.describe()
	if len(windows) == 0 {
		return app.ErrNoWindows
	}
	window, _ := application.current()
	snapshot := window.session.Snapshot()
	fmt.Fprintf(os.Stdout, "ok ready %dx%d scale=%g windows=%d active=%d\n", snapshot.Width, snapshot.Height, window.session.options.Scale, len(windows), active)
	return ServeApplication(application, os.Stdin, os.Stdout)
}

// ServeApplication drives a multi-window headless application. windows lists
// live windows, window ID selects the target for ordinary commands, and
// requestclose [ID] sends one window its close button, which an application with
// an OnCloseRequest may turn into hiding the window. closewindow [ID] closes one window. Closing the final window ends the loop.
func ServeApplication(application *applicationSession, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(line) != "" {
			reply, quit := executeApplication(application, line)
			if _, writeErr := io.WriteString(out, reply); writeErr != nil {
				return writeErr
			}
			if quit {
				return nil
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func executeApplication(application *applicationSession, line string) (string, bool) {
	command, rest, _ := strings.Cut(strings.TrimSpace(line), " ")
	rest = strings.TrimSpace(rest)
	switch command {
	case "windows":
		if rest != "" {
			return "error windows takes no arguments\n", false
		}
		active, windows := application.describe()
		var reply strings.Builder
		for _, window := range windows {
			fmt.Fprintf(&reply, "id=%d active=%t panel=%t hidden=%t title=%q\n", window.id, window.id == active, window.panel, window.hidden, window.title)
		}
		fmt.Fprintf(&reply, "ok windows=%d active=%d\n", len(windows), active)
		return reply.String(), false
	case "dockbadge":
		if rest != "" {
			return "error dockbadge takes no arguments\n", false
		}
		application.mu.Lock()
		label := application.badge
		application.mu.Unlock()
		return fmt.Sprintf("ok label=%q\n", label), false
	case "openurl":
		application.mu.Lock()
		handler := application.openURL
		application.mu.Unlock()
		switch {
		case rest == "":
			return "error openurl needs a URL\n", false
		case handler == nil:
			return "error the application does not take links\n", false
		}
		handler(rest)
		return application.afterCommand("ok\n")
	case "statusitems":
		if rest != "" {
			return "error statusitems takes no arguments\n", false
		}
		application.mu.Lock()
		ids := make([]uint64, 0, len(application.items))
		for id := range application.items {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(left, right int) bool { return ids[left] < ids[right] })
		var reply strings.Builder
		for _, id := range ids {
			item := application.items[id]
			fmt.Fprintf(&reply, "id=%d tooltip=%q icon=%dx%d template=%t", id, item.tooltip, item.icon.Size, item.icon.Size, item.icon.Template)
			if item.menu != nil {
				reply.WriteString(" menu=true")
			}
			reply.WriteByte('\n')
		}
		fmt.Fprintf(&reply, "ok statusitems=%d\n", len(ids))
		application.mu.Unlock()
		return reply.String(), false
	case "statusmenu":
		id, err := strconv.ParseUint(rest, 10, 64)
		if err != nil || id == 0 {
			return "error statusmenu needs a positive numeric item ID\n", false
		}
		items, err := application.statusMenu(id)
		if err != nil {
			return "error " + err.Error() + "\n", false
		}
		var reply strings.Builder
		writeStatusMenu(&reply, items, "")
		fmt.Fprintf(&reply, "ok entries=%d\n", len(items))
		return reply.String(), false
	case "statusselect":
		fields := strings.Fields(rest)
		if len(fields) != 2 {
			return "error statusselect needs an item ID and a menu path such as 2.1\n", false
		}
		id, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil || id == 0 {
			return "error statusselect needs a positive numeric item ID\n", false
		}
		if err := application.selectStatusMenu(id, fields[1]); err != nil {
			return "error " + err.Error() + "\n", false
		}
		return application.afterCommand("ok\n")
	case "statusclick":
		fields := strings.Fields(rest)
		if len(fields) == 0 || len(fields) > 2 {
			return "error statusclick needs an item ID and optionally primary or secondary\n", false
		}
		id, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil || id == 0 {
			return "error statusclick needs a positive numeric item ID\n", false
		}
		button := ui.PointerButtonPrimary
		if len(fields) == 2 {
			switch fields[1] {
			case "primary":
			case "secondary":
				button = ui.PointerButtonSecondary
			default:
				return "error statusclick button is primary or secondary\n", false
			}
		}
		if err := application.clickStatusItem(id, button); err != nil {
			return "error " + err.Error() + "\n", false
		}
		return application.afterCommand("ok\n")
	case "requestclose":
		// The window's close button: an application that handles close requests
		// decides, as it would natively, and one that does not is closed.
		id := application.activeID()
		if rest != "" {
			parsed, err := strconv.ParseUint(rest, 10, 64)
			if err != nil || parsed == 0 {
				return "error requestclose needs a positive numeric window ID\n", false
			}
			id = parsed
		}
		application.mu.Lock()
		window := application.windows[id]
		application.mu.Unlock()
		if window == nil || window.panel {
			return "error headless: no such window\n", false
		}
		if window.closeRequest == nil {
			application.close(id)
		} else {
			window.closeRequest(func() { application.close(id) })
		}
		return application.afterCommand(fmt.Sprintf("ok requested=%d windows=%d\n", id, application.count()))
	case "blur":
		id, err := strconv.ParseUint(rest, 10, 64)
		if rest == "" {
			id, err = application.activeID(), nil
		}
		if err != nil || id == 0 {
			return "error blur needs a positive numeric window ID\n", false
		}
		if err := application.blurWindow(id); err != nil {
			return "error " + err.Error() + "\n", false
		}
		return application.afterCommand("ok\n")
	case "window":
		id, err := strconv.ParseUint(rest, 10, 64)
		if err != nil || id == 0 {
			return "error window needs a positive numeric ID\n", false
		}
		if err := application.selectWindow(id); err != nil {
			return "error " + err.Error() + "\n", false
		}
		return fmt.Sprintf("ok active=%d\n", id), false
	case "closewindow":
		window, ok := application.current()
		if rest != "" {
			id, err := strconv.ParseUint(rest, 10, 64)
			if err != nil || id == 0 {
				return "error closewindow needs a positive numeric ID\n", false
			}
			application.mu.Lock()
			window = application.windows[id]
			application.mu.Unlock()
			ok = window != nil
		}
		if !ok {
			return "error headless: no such window\n", false
		}
		application.close(window.id)
		remaining := application.count()
		return fmt.Sprintf("ok closed=%d windows=%d\n", window.id, remaining), remaining == 0
	case "quit":
		application.closeAll()
		return "ok\n", true
	}
	window, ok := application.current()
	if !ok {
		return "error headless: no active window\n", true
	}
	reply, quit := execute(window.session, line)
	if quit {
		return reply, true
	}
	return application.afterCommand(reply)
}

func (application *applicationSession) activeID() uint64 {
	application.mu.Lock()
	defer application.mu.Unlock()
	return application.active
}

// afterCommand ends the run when the application asked to quit while handling
// the command, as a native application ends once its last window has closed.
// The final line then reports windows=0, which is how a driver knows.
func (application *applicationSession) afterCommand(reply string) (string, bool) {
	if !application.quitRequested() {
		return reply, false
	}
	return strings.TrimSuffix(reply, "\n") + " windows=0\n", true
}
