package gui

import (
	"fmt"
	"time"

	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/lazygit/pkg/gui/style"
)

// Currently there is a bug where if we switch to a subprocess from within
// WithWaitingStatus we get stuck there and can't return to lazygit. We could
// fix this bug, or just stop running subprocesses from within there, given that
// we don't need to see a loading status if we're in a subprocess.
func (gui *Gui) withGpgHandling(cmdStr string, waitingStatus string, onSuccess func() error) error {
	useSubprocess := gui.GitCommand.UsingGpg()
	if useSubprocess {
		// Need to remember why we use the shell for the subprocess but not in the other case
		// Maybe there's no good reason
		success, err := gui.runSubprocessWithSuspense(gui.OSCommand.ShellCommandFromString(cmdStr))
		if success && onSuccess != nil {
			if err := onSuccess(); err != nil {
				return err
			}
		}
		if err := gui.refreshSidePanels(refreshOptions{mode: ASYNC}); err != nil {
			return err
		}

		return err
	} else {
		return gui.RunAndStream(cmdStr, waitingStatus, onSuccess)
	}
}

func (gui *Gui) RunAndStream(cmdStr string, waitingStatus string, onSuccess func() error) error {
	return gui.WithWaitingStatus(waitingStatus, func() error {
		cmd := gui.OSCommand.ShellCommandFromString(cmdStr)
		cmd.Env = append(cmd.Env, "TERM=dumb")
		gui.Views.Extras.WriteString(fmt.Sprintf("\n\n%s\n", style.FgMagenta.Sprint(gui.Tr.GitOutput)))
		cmd.Stdout = gui.Views.Extras
		cmd.Stderr = gui.Views.Extras

		done := make(chan struct{})
		if err := cmd.Start(); err != nil {
			return gui.surfaceError(err)
		}

		gui.goEvery(time.Millisecond*50, done, func() error {
			gui.g.Update(func(*gocui.Gui) error { return nil })
			return nil
		})

		if err := cmd.Wait(); err != nil {
			if _, err := cmd.Stdout.Write([]byte(fmt.Sprintf("%s\n", style.FgRed.Sprint(err.Error())))); err != nil {
				gui.Log.Error(err)
			}
			_ = gui.refreshSidePanels(refreshOptions{mode: ASYNC})
			return gui.surfaceError(
				fmt.Errorf(
					gui.Tr.GitCommandFailed, gui.Config.GetUserConfig().Keybinding.Universal.ExtrasMenu,
				),
			)
		}

		done <- struct{}{}

		if err := onSuccess(); err != nil {
			return err
		}

		return gui.refreshSidePanels(refreshOptions{mode: ASYNC})
	})
}
