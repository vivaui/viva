package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func main() {
	err := app.RunApplication(func(application *app.Application) error {
		var window *app.Window
		var latest *app.NotificationHandle
		var err error
		window, err = ui.OpenWindow(application, app.Config{Title: "System notifications", Width: 600, Height: 300}, ui.PageConfig{View: func(c ui.C) {
			ui.Column(c).Fill().Pad(24).Gap(12).Children(func() {
				if !app.Supports(app.CapabilityNotificationActivation) {
					ui.Text(c, "Interactive notifications are not supported on this target.")
					return
				}
				ui.Text(c, "On macOS, run the packaged application and allow system notifications.")
				if ui.Button(c, "Show notification").Clicked() {
					latest.Close()
					latest, err = application.ShowNotification(app.Notification{Title: "Viva", Body: "Click to restore the application window."}, app.NotificationEvents{
						OnActivate: func() { window.Show() },
						OnError:    func(err error) { log.Printf("notification delivery: %v", err) },
					})
					if err != nil {
						log.Printf("request notification: %v", err)
					}
				}
				if ui.Button(c, "Withdraw notification").Clicked() {
					latest.Close()
					latest = nil
				}
			})
		}})
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
}
