package control

import (
	"fmt"
	"time"
)

// Desired intent is independent of observed phase. Deleted intent is terminal;
// retrying running/failed starts a fresh attempt without replacing identity or data.
// Generation invalidates observations from earlier intent.
func setDesired(i *Instance, desired string) error {
	if desired != "running" && desired != "stopped" && desired != "deleted" {
		return fmt.Errorf("invalid desired state")
	}
	if i.Desired == "deleted" {
		if desired != "deleted" {
			return fmt.Errorf("instance has been deleted")
		}
		if i.Phase != "deleted" && i.Error != "" {
			i.Generation++
			i.Error = ""
		}
		return nil
	}
	if i.Desired == desired {
		if desired == "running" && i.Phase == "failed" {
			i.Generation++
			i.Error = ""
			i.Phase = "queued"
			i.EmptySince = time.Time{}
			i.Players = nil
			i.StartedAt = time.Now().UTC()
		}
		return nil
	}
	i.Generation++
	i.Desired = desired
	i.Error = ""
	i.Players = nil
	i.StopReason = ""
	i.EmptySince = time.Time{}
	if desired == "running" {
		i.StartedAt = time.Now().UTC()
		i.Phase = "queued"
	}
	return nil
}

func stopForTimeout(i *Instance, reason string) {
	_ = setDesired(i, "stopped")
	i.StopReason = reason
}
