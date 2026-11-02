//go:build darwin || windows || linux

package app

import "os/exec"

// startOpenWith reaps the child without delaying the caller. A released process
// handle would leave short-lived launchers such as macOS open unreaped.
func startOpenWith(command *exec.Cmd) error {
	if err := command.Start(); err != nil {
		return err
	}
	go func() {
		_ = command.Wait()
	}()
	return nil
}
