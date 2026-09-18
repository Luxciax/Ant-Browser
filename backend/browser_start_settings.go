package backend

import (
	"ant-chrome/backend/internal/config"
	"errors"
	"time"
)

const (
	defaultBrowserStartReadyTimeout  = 3 * time.Second
	defaultBrowserStartStableWindow  = 1200 * time.Millisecond
	defaultBrowserStartMaxAttempts   = 5
	defaultBrowserStartPrepareBudget = 30 * time.Second
)

func browserStartReadyTimeoutMillis(cfg *config.Config) int {
	fallback := int(defaultBrowserStartReadyTimeout / time.Millisecond)
	if cfg == nil {
		return fallback
	}
	if cfg.Browser.StartReadyTimeoutMs > 0 {
		return cfg.Browser.StartReadyTimeoutMs
	}
	return fallback
}

func browserStartStableWindowMillis(cfg *config.Config) int {
	fallback := int(defaultBrowserStartStableWindow / time.Millisecond)
	if cfg == nil {
		return fallback
	}
	if cfg.Browser.StartStableWindowMs > 0 {
		return cfg.Browser.StartStableWindowMs
	}
	return fallback
}

func (a *App) browserStartTimingSettings() (time.Duration, time.Duration) {
	return time.Duration(browserStartReadyTimeoutMillis(a.config)) * time.Millisecond,
		time.Duration(browserStartStableWindowMillis(a.config)) * time.Millisecond
}

func browserStartAttemptCount() int {
	return defaultBrowserStartMaxAttempts
}

func browserStartTotalTimeout(plan *browserStartPlan) time.Duration {
	if plan == nil {
		return 20 * time.Second
	}
	attempts := plan.maxStartAttempts
	if attempts <= 0 {
		attempts = 1
	}
	total := time.Duration(attempts) * (plan.startReadyTimeout + plan.startStableWindow)
	if total < 5*time.Second {
		total = 5 * time.Second
	}
	// Small bounded allowance for process creation and bookkeeping. The slow
	// readiness loop itself is still capped by the context deadline.
	return total + 2*time.Second
}

func browserStartTransactionTimeout(cfg *config.Config) time.Duration {
	ready := time.Duration(browserStartReadyTimeoutMillis(cfg)) * time.Millisecond
	stable := time.Duration(browserStartStableWindowMillis(cfg)) * time.Millisecond
	attempts := browserStartAttemptCount()
	if attempts <= 0 {
		attempts = 1
	}
	total := defaultBrowserStartPrepareBudget + time.Duration(attempts)*(ready+stable) + 2*time.Second
	if total < 10*time.Second {
		return 10 * time.Second
	}
	return total
}

func shouldRetryBrowserReadyFailure(err error) bool {
	if err == nil {
		return false
	}

	var exitErr *browserStartupExitError
	return !errors.As(err, &exitErr)
}
