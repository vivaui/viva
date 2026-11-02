package platform

import "os/exec"

// StartReaped starts command and reaps it in the background once it exits, so a
// caller launching a long-lived program is never delayed by it and the child
// never zombies.
func StartReaped(command *exec.Cmd) error {
	if err := command.Start(); err != nil {
		return err
	}
	go func() {
		_ = command.Wait()
	}()
	return nil
}
