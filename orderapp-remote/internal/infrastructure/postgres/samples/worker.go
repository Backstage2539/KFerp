package samples

import (
	"context"
	"log"
	"time"
)

func (r Repository) RunReminders(ctx context.Context) {
	run := func() {
		job, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if n, err := r.Remind(job, time.Now().UTC()); err != nil {
			log.Printf("sample reminders failed: %v", err)
		} else if n > 0 {
			log.Printf("sample follow-up reminders: %d", n)
		}
	}
	run()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
