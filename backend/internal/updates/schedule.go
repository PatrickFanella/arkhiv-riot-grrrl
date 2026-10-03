package updates

import (
	"context"
	"encoding/json"
	"time"

	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/catalog"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/config"
	"github.com/jackc/pgx/v5"
)

// Due checks the collection cadence and persisted source backoff. Manual refreshes
// can ignore cadence, but cannot override a source's Retry-After boundary.
func Due(ctx context.Context, c config.Collection, db *catalog.DB, scheduled bool) (bool, string, error) {
	if scheduled && !c.Updates.Enabled {
		return false, "disabled", nil
	}
	var raw []byte
	err := db.Pool.QueryRow(ctx, `SELECT result FROM source_refresh_runs WHERE collection_id=$1 ORDER BY started_at DESC LIMIT 1`, c.ID).Scan(&raw)
	if err != nil && err != pgx.ErrNoRows {
		return false, "", err
	}
	if err == nil {
		var last Result
		if err = json.Unmarshal(raw, &last); err != nil {
			return false, "", err
		}
		if last.RetryAt != nil && time.Now().Before(*last.RetryAt) {
			return false, "source-backoff", nil
		}
		if last.Status == "failed" && last.Phase == "acquisition" {
			return true, "resume-acquisition", nil
		}

	}
	if !scheduled {
		return true, "due", nil
	}
	hours := c.Updates.CadenceHours
	if hours == 0 {
		hours = 24
	}
	var finished time.Time
	err = db.Pool.QueryRow(ctx, `SELECT finished_at FROM source_refresh_runs WHERE collection_id=$1 AND status='complete' ORDER BY finished_at DESC LIMIT 1`, c.ID).Scan(&finished)
	if err == pgx.ErrNoRows {
		return true, "due", nil
	}
	if err != nil {
		return false, "", err
	}
	if time.Now().Before(finished.Add(time.Duration(hours) * time.Hour)) {
		return false, "not-due", nil
	}
	return true, "due", nil
}
