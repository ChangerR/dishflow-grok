package worker

import (
	"context"
	"time"

	"github.com/changerr/dishflow-grok/internal/app"
)

func Run(ctx context.Context, a *app.App) {
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	hourly := time.NewTicker(time.Hour)
	defer hourly.Stop()
	runOnce(ctx, a)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			runOnce(ctx, a)
		case <-hourly.C:
			_ = a.CloseExpiredRestores(ctx)
		}
	}
}

func runOnce(ctx context.Context, a *app.App) {
	if a.Redis != nil {
		_ = a.Redis.Set(ctx, "worker:heartbeat", a.Now().UTC().Format(time.RFC3339), 45*time.Second).Err()
	}
	_ = a.ReleaseExpiredHolds(ctx)
	_ = a.ReconcilePayments(ctx)
	_ = a.ReconcileRefunds(ctx)
	_ = a.DispatchOutbox(ctx)
	_ = a.ProcessPrintJobs(ctx)
}
