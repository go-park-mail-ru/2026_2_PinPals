package onlineconf

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var checks = promauto.NewCounter(prometheus.CounterOpts{Name: "onlineconf_flag_checks_total", Help: "Feature decisions."})
var missing = promauto.NewCounter(prometheus.CounterOpts{Name: "onlineconf_flag_fallback_total", Help: "Feature decisions using a fallback."})
var failures = promauto.NewCounter(prometheus.CounterOpts{Name: "onlineconf_sync_errors_total", Help: "Failed snapshot refreshes."})
var revision = promauto.NewGauge(prometheus.GaugeOpts{Name: "onlineconf_cache_revision", Help: "Last snapshot revision."})
var lastSync atomic.Int64
var _ = promauto.NewGaugeFunc(prometheus.GaugeOpts{Name: "onlineconf_last_sync_timestamp_seconds", Help: "Last successful refresh."}, func() float64 { return float64(lastSync.Load()) })
var _ = promauto.NewGaugeFunc(prometheus.GaugeOpts{Name: "onlineconf_cache_age_seconds", Help: "Seconds since the last refresh, or -1 before startup."}, func() float64 {
	t := lastSync.Load()
	if t == 0 {
		return -1
	}
	return float64(time.Now().Unix() - t)
})

func MarkSynced(v int64) { lastSync.Store(time.Now().Unix()); revision.Set(float64(v)) }
func CountDecision()     { checks.Inc() }
func SyncFailed()        { failures.Inc() }

type Cache struct{ current atomic.Pointer[Snapshot] }

func (c *Cache) PageSize() int { return c.current.Load().PageSize() }
func (c *Cache) Enabled(key string, user int) bool {
	CountDecision()
	if s := c.current.Load(); s != nil {
		if f, ok := s.Flags[key]; ok {
			return f.Evaluate(user).Enabled
		}
	}
	missing.Inc()
	return false
}
func (c *Cache) Refresh(ctx context.Context, client *http.Client, base, scope, token string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", base+"/internal/v1/onlineconf/snapshot?scope="+url.QueryEscape(scope), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return errors.New("snapshot request refused")
	}
	var s Snapshot
	decoder := json.NewDecoder(res.Body)
	if err = decoder.Decode(&s); err != nil {
		return err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return errors.New("unexpected snapshot content")
	}
	if s.Scope != scope || s.Revision < 1 || s.Flags == nil || s.Configs == nil {
		return errors.New("incomplete snapshot")
	}
	if value, ok := s.Configs[PageSizeKey]; ok {
		var n int
		if json.Unmarshal(value, &n) != nil || n < 1 || n > 100 {
			return errors.New("invalid page size")
		}
	}
	for _, f := range s.Flags {
		if f.ID < 1 || f.Percent < 0 || f.Percent > 100 {
			return errors.New("invalid flag")
		}
	}
	c.current.Store(&s)
	MarkSynced(s.Revision)
	return nil
}
func (c *Cache) Run(ctx context.Context, base, scope, token string, interval time.Duration) {
	if interval <= 0 || interval > 20*time.Second {
		interval = 15 * time.Second
	}
	client := &http.Client{Timeout: 4 * time.Second}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := c.Refresh(ctx, client, base, scope, token); err != nil && ctx.Err() == nil {
			SyncFailed()
			slog.Warn("snapshot refresh failed", "scope", scope, "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
