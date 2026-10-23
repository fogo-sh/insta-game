package command

import (
	"context"
	"fmt"
	"time"

	"github.com/fogo-sh/insta-game/internal/control"
)

func idleSince(instances []control.Instance, previous, now time.Time) time.Time {
	if !control.InstancesIdle(instances) {
		return time.Time{}
	}
	if previous.IsZero() {
		return now
	}
	return previous
}

func poweroffWhenIdle(ctx context.Context, duration time.Duration, list func() ([]control.Instance, error), poweroff func(context.Context) (bool, error)) error {
	if duration <= 0 {
		return nil
	}
	ticker := time.NewTicker(min(duration, time.Second))
	defer ticker.Stop()
	var since time.Time
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		instances, err := list()
		if err != nil {
			return fmt.Errorf("check host idle state: %w", err)
		}
		now := time.Now()
		since = idleSince(instances, since, now)
		if !since.IsZero() && now.Sub(since) >= duration {
			if err := ctx.Err(); err != nil {
				return err
			}
			committed, err := poweroff(ctx)
			if err != nil || committed {
				return err
			}
			since = time.Time{}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
