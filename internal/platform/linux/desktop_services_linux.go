package linux

import "github.com/vivaui/viva/internal/platform"

// desktopServices are the connections to the session bus an application keeps for its windows,
// whichever shell draws them: the icons in the status area, the watch on the desktop's settings
// and the accessibility bridge. Both shells embed it; the shell's loop polls the descriptors the
// services expose and calls their serve functions when one is readable.
type desktopServices struct {
	host busHost

	status        *statusNotifier
	desktop       *desktopWatch
	desktopFailed bool
	atspi         *atspiHost
}

// busHost is the shell a desktopServices belongs to.
type busHost interface {
	atspiOwner
	// post wakes the loop from another goroutine to run a command (commandStatus,
	// commandDesktop) on it.
	post(command uint32)
	// refreshText draws every window's text again, with the fonts now in force.
	refreshText()
}

// services returns the application's services with their host set.
func (application *linuxApplication) services() *desktopServices {
	application.desktopServices.host = application
	return &application.desktopServices
}

func (application *linuxApplication) refreshText() {
	for _, state := range application.windows {
		state.refreshText()
	}
}

func (application *linuxApplication) AddStatusItem(config platform.StatusItemConfig) (platform.StatusItem, error) {
	return application.services().AddStatusItem(config)
}
func (application *linuxApplication) syncDesktopWatch()         { application.services().syncDesktopWatch() }
func (application *linuxApplication) serveDesktop()             { application.services().serveDesktop() }
func (application *linuxApplication) closeDesktop()             { application.services().closeDesktop() }
func (application *linuxApplication) announce(c settingsChange) { application.services().announce(c) }
func (application *linuxApplication) closeStatus()              { application.services().closeStatus() }
func (application *linuxApplication) serveStatus()              { application.services().serveStatus() }
func (application *linuxApplication) drainStatusRemovals() {
	application.services().drainStatusRemovals()
}
func (application *linuxApplication) atspiOpened(state *linuxWindowState) {
	application.services().atspiOpened(state)
}
func (application *linuxApplication) atspiFrame(state *linuxWindowState, nodes []platform.AccessibilityNode) {
	application.services().atspiFrame(state, nodes)
}
func (application *linuxApplication) atspiActive(state *linuxWindowState, active bool) {
	application.services().atspiActive(state, active)
}
func (application *linuxApplication) atspiClosed(state *linuxWindowState) {
	application.services().atspiClosed(state)
}
func (application *linuxApplication) atspiClose() { application.services().atspiClose() }
func (application *linuxApplication) serveAtspi() { application.services().serveAtspi() }
