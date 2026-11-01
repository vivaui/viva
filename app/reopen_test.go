//go:build darwin || windows || linux

package app

import "testing"

type reopenHost struct {
	fakeHost
	handler func()
}

func (host *reopenHost) SetReopenHandler(handler func()) { host.handler = handler }

func TestApplicationReopenCanSelectMainWindowWithoutRestoringSettings(t *testing.T) {
	host := &reopenHost{}
	application := &Application{host: host}
	main, err := application.OpenWindow(Config{Title: "main"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(main.Close)
	settings, err := application.OpenWindow(Config{Title: "settings"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(settings.Close)
	application.OnReopen(func() {
		settings.Hide()
		main.Show()
	})
	for _, mainHidden := range []bool{false, true} {
		settings.Show()
		if mainHidden {
			main.Hide()
		}
		host.handler()
		if main.Hidden() || !settings.Hidden() {
			t.Fatal("application reopen did not select the main window")
		}
	}
	application.OnReopen(nil)
	if host.handler != nil {
		t.Fatal("removing the application handler did not restore default routing")
	}
}
