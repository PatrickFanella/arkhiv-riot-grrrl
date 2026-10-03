package updates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
	"syscall"
	"time"

	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/catalog"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/config"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/index"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/ingest"
	ia "git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/source/internetarchive"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/storage"
)

type Acquisition struct {
	Acquired int   `json:"acquired"`
	Reused   int   `json:"reused"`
	Pending  int   `json:"pending"`
	Failed   int   `json:"failed"`
	Bytes    int64 `json:"bytes"`
}

// Acquire creates a separate unpublished edition for each reviewed content change.
// Its deterministic source key resumes an interrupted import without changing an
// existing published document or losing its older preserved edition.
func Acquire(ctx context.Context, c config.Collection, client *ia.Client, store *storage.Store, db *catalog.DB) (result Acquisition, err error) {
	due, status, e := Due(ctx, c, db, false)
	if e != nil {
		return result, e
	}
	if !due {
		return result, fmt.Errorf("source acquisition deferred: %s", status)
	}
	defer func() {
		var backoff *ia.RetryError
		if errors.As(err, &backoff) {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err = errors.Join(err, db.SourceBackoff(cleanup, c.ID, "acquisition", backoff.At))
		}
	}()

	rows, e := db.Pool.Query(ctx, `SELECT id,source_item,snapshot_id,detail FROM source_changes WHERE collection_id=$1 AND kind IN ('added','content') AND (decision='approve' OR ($2 AND decision='pending')) ORDER BY created_at,id LIMIT 500`, c.ID, c.Updates.Policy == "acquire")
	if e != nil {
		return result, e
	}
	type candidate struct {
		id, item, snapshot string
		change             Change
	}
	candidates := []candidate{}
	for rows.Next() {
		var v candidate
		var b []byte
		if e = rows.Scan(&v.id, &v.item, &v.snapshot, &b); e != nil {
			rows.Close()
			return result, e
		}
		if e = json.Unmarshal(b, &v.change); e != nil {
			rows.Close()
			return result, e
		}
		candidates = append(candidates, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return result, e
	}
	var count int
	if e = db.Pool.QueryRow(ctx, `SELECT count(*) FROM documents WHERE collection_id=$1`, c.ID).Scan(&count); e != nil {
		return result, e
	}
	remaining := c.Limits.Documents - count
	for _, v := range candidates {
		f := v.change.After
		if f == nil || !strings.HasSuffix(strings.ToLower(f.Name), ".pdf") || f.Private == "true" || f.Bytes() <= 0 || f.Bytes() > c.Limits.FileBytes || (f.MD5 == "" && f.SHA1 == "") {
			continue
		}
		key := "update-" + v.id
		documentID := catalog.ID(c.ID, v.item, key)
		var exists bool
		if e = db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM documents WHERE id=$1)`, documentID).Scan(&exists); e != nil {
			return result, e
		}
		if (!exists && remaining <= 0) || result.Bytes+f.Bytes() > c.Limits.TotalBytes {
			break
		}
		// Source changes since review require another inventory/review, not a silent swap.
		var current []byte
		if e = db.Pool.QueryRow(ctx, `SELECT inventory FROM source_inventories WHERE collection_id=$1 AND source_item=$2`, c.ID, v.item).Scan(&current); e != nil {
			return result, e
		}
		var inventory Inventory
		if e = json.Unmarshal(current, &inventory); e != nil {
			return result, e
		}
		if actual, ok := inventory.Files[f.Name]; !ok || !equal(actual, *f) {
			continue
		}
		companion, hasCompanion := inventory.Files[strings.TrimSuffix(f.Name, ".pdf")+"_djvu.txt"]
		pairBytes := f.Bytes()
		if hasCompanion {
			if companion.Private == "true" || companion.Bytes() <= 0 || companion.Bytes() > c.Limits.FileBytes || (companion.MD5 == "" && companion.SHA1 == "") {
				continue
			}
			pairBytes += companion.Bytes()
		}
		if result.Bytes+pairBytes > c.Limits.TotalBytes {
			break
		}
		var disk syscall.Statfs_t
		if e = syscall.Statfs(store.Path, &disk); e != nil {
			return result, e
		}
		floor := c.Updates.MinimumFreeBytes
		if floor == 0 {
			floor = 2 << 30
		}
		if uint64(disk.Bavail)*uint64(disk.Bsize) < uint64(floor)+uint64(pairBytes) {
			return result, fmt.Errorf("update stopped: low disk capacity")
		}
		hash, n, reused, downloadErr := ingest.Download(ctx, client, store, v.item, *f, c.Limits.FileBytes)
		if downloadErr != nil {
			result.Failed++
			err = fmt.Errorf("source update acquisition failed; preserved checkpoint retained: %w", downloadErr)
			var backoff *ia.RetryError
			if errors.As(downloadErr, &backoff) {
				return result, err
			}
			continue
		}
		var existing string
		lookupErr := db.Pool.QueryRow(ctx, `SELECT d.id FROM variants v JOIN documents d ON d.id=v.document_id WHERE d.collection_id=$1 AND d.source_item=$2 AND v.filename=$3 AND v.asset_sha=$4 ORDER BY d.id LIMIT 1`, c.ID, v.item, f.Name, hash).Scan(&existing)
		if lookupErr == nil {
			if existing == documentID && hasCompanion {
				if e = preserveCompanion(ctx, c, client, store, db, v.item, hash, companion, v.snapshot); e != nil {
					result.Failed++
					err = fmt.Errorf("source OCR acquisition failed; preserved checkpoint retained: %w", e)
					var backoff *ia.RetryError
					if errors.As(e, &backoff) {
						return result, err
					}
					continue
				}
				result.Bytes += companion.Bytes()
			}
			if e = index.Extract(ctx, db, store, hash); e != nil {
				result.Failed++
				err = fmt.Errorf("source update extraction failed; unpublished edition retained")
				continue
			}
			if _, e = db.Pool.Exec(ctx, `UPDATE source_changes SET decision='acquired',document_id=$2,reviewed_at=now() WHERE id=$1`, v.id, existing); e != nil {
				return result, e
			}
			result.Reused++
			result.Bytes += n
			continue
		}
		if lookupErr != pgx.ErrNoRows {
			return result, lookupErr
		}
		title := strings.TrimSuffix(f.Name, ".pdf")
		for _, selected := range c.Selection {
			if selected.File == f.Name && (selected.Item == v.item || (selected.Item == "" && c.Item == v.item)) {
				title = selected.Title
				break
			}
		}
		selection := config.Selection{File: f.Name, Key: key, Title: title, Rights: "unknown"}
		itemConfig := c
		itemConfig.Item = v.item
		if e = db.Import(ctx, itemConfig, selection, hash, n, v.snapshot); e != nil {
			return result, e
		}
		if hasCompanion {
			if e = preserveCompanion(ctx, c, client, store, db, v.item, hash, companion, v.snapshot); e != nil {
				result.Failed++
				err = fmt.Errorf("source OCR acquisition failed; preserved checkpoint retained: %w", e)
				var backoff *ia.RetryError
				if errors.As(e, &backoff) {
					return result, err
				}
				continue
			}
			result.Bytes += companion.Bytes()
		}
		if e = index.Extract(ctx, db, store, hash); e != nil {
			result.Failed++
			err = fmt.Errorf("source update extraction failed; unpublished edition retained")
			continue
		}
		if _, e = db.Pool.Exec(ctx, `UPDATE source_changes SET decision='acquired',document_id=$2,reviewed_at=now() WHERE id=$1`, v.id, documentID); e != nil {
			return result, e
		}
		result.Acquired++
		result.Bytes += n
		if reused {
			result.Reused++
		}
		if !exists {
			remaining--
		}
	}
	if e = db.Pool.QueryRow(ctx, `SELECT count(*) FROM source_changes WHERE collection_id=$1 AND decision IN ('pending','approve','hold')`, c.ID).Scan(&result.Pending); e != nil {
		return result, e
	}
	return
}

func preserveCompanion(ctx context.Context, c config.Collection, client *ia.Client, store *storage.Store, db *catalog.DB, item, asset string, file ia.File, snapshot string) error {
	hash, size, _, err := ingest.Download(ctx, client, store, item, file, c.Limits.FileBytes)
	if err != nil {
		return err
	}
	if _, err = db.Pool.Exec(ctx, `INSERT INTO assets(sha256,size) VALUES($1,$2) ON CONFLICT DO NOTHING`, hash, size); err != nil {
		return err
	}
	_, err = db.Pool.Exec(ctx, `INSERT INTO asset_companions(asset_sha,companion_sha,filename,source_snapshot) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, asset, hash, file.Name, snapshot)
	return err
}
