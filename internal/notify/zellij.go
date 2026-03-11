package notify

import (
	"fmt"
	"os/exec"
)

// TabRenamer renames the current Zellij tab.
type TabRenamer interface {
	RenameTab(name string) error
}

// ZellijTabRenamer renames tabs via `zellij action rename-tab`.
type ZellijTabRenamer struct{}

func (z *ZellijTabRenamer) RenameTab(name string) error {
	cmd := exec.Command("zellij", "action", "rename-tab", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zellij rename-tab failed: %w: %s", err, out)
	}
	return nil
}
