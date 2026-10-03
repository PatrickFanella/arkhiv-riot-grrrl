package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/storage"
)

// PublicReview binds owner access decisions to immutable variant identities.
// It deliberately makes no determination of a reuse license.
type PublicReview struct {
	Format       int              `json:"format"`
	Collection   string           `json:"collection"`
	PublicURL    string           `json:"public_url"`
	Contact      string           `json:"correction_contact"`
	Approval     string           `json:"public_approval"`
	ReuseLicense bool             `json:"reuse_license_asserted"`
	Documents    []PublicDocument `json:"documents"`
}
type PublicDocument struct {
	ID       string    `json:"id"`
	Source   string    `json:"source_item"`
	Approved bool      `json:"public_approved"`
	Rights   string    `json:"rights_status"`
	Variants []Variant `json:"variants"`
}

func (d *DB) ApprovePublic(ctx context.Context, store *storage.Store, raw []byte, review string) (int, error) {
	var manifest PublicReview
	if len(raw) > 1<<20 || json.Unmarshal(raw, &manifest) != nil || manifest.Format != 1 || manifest.Collection == "" || manifest.Contact == "" || manifest.PublicURL == "" || manifest.Approval == "" || manifest.ReuseLicense || len(manifest.Documents) == 0 || strings.TrimSpace(review) == "" {
		return 0, fmt.Errorf("owner access review, exact manifest and no asserted reuse license required")
	}
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	seen := map[string]bool{}
	count := 0
	sum := sha256.Sum256(raw)
	manifestHash := hex.EncodeToString(sum[:])
	for _, doc := range manifest.Documents {
		if !doc.Approved || doc.Rights == "" || len(doc.Variants) == 0 {
			return 0, fmt.Errorf("manifest contains an unapproved or incomplete document")
		}
		var collection, item, rights string
		var published bool
		if err = tx.QueryRow(ctx, `SELECT collection_id,source_item,rights,published FROM documents WHERE id=$1 FOR UPDATE`, doc.ID).Scan(&collection, &item, &rights, &published); err != nil {
			return 0, err
		}
		if collection != manifest.Collection || item != doc.Source || rights != doc.Rights || !published {
			return 0, fmt.Errorf("review differs from published catalog record")
		}
		for _, v := range doc.Variants {
			if seen[v.ID] {
				return 0, fmt.Errorf("duplicate reviewed variant")
			}
			seen[v.ID] = true
			var sha, name string
			var size int64
			var pages int
			var status string
			err = tx.QueryRow(ctx, `SELECT v.asset_sha,v.filename,a.size,x.pages,x.status FROM variants v JOIN assets a ON a.sha256=v.asset_sha JOIN derivatives x ON x.asset_sha=v.asset_sha WHERE v.id=$1 AND v.document_id=$2 FOR UPDATE OF v`, v.ID, doc.ID).Scan(&sha, &name, &size, &pages, &status)
			if err != nil {
				return 0, err
			}
			if sha != v.SHA || name != v.Filename || size != v.Size || pages < 1 || status == "failed" {
				return 0, fmt.Errorf("reviewed original or readable derivative differs")
			}
			if err = store.Verify(sha, size); err != nil {
				return 0, err
			}
			var derivative struct {
				Input  string `json:"input_sha256"`
				Pages  int    `json:"pages"`
				Status string `json:"status"`
			}
			if err = store.JSON("derivatives/"+sha+"/manifest.json", &derivative); err != nil || derivative.Input != sha || derivative.Pages != pages || derivative.Status != status {
				return 0, fmt.Errorf("verified readable derivative manifest required")
			}

			if _, err = tx.Exec(ctx, `UPDATE variants SET public_approved=true WHERE id=$1`, v.ID); err != nil {
				return 0, err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO public_access_events(variant_id,asset_sha,approved,review,manifest_sha256) VALUES($1,$2,true,$3,$4)`, v.ID, sha, review, manifestHash); err != nil {
				return 0, err
			}
			count++
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}

func (d *DB) RevokePublic(ctx context.Context, variant, review string) error {
	if variant == "" || strings.TrimSpace(review) == "" {
		return fmt.Errorf("variant and revocation review required")
	}
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var sha string
	if err = tx.QueryRow(ctx, `UPDATE variants SET public_approved=false WHERE id=$1 RETURNING asset_sha`, variant).Scan(&sha); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public_access_events(variant_id,asset_sha,approved,review) VALUES($1,$2,false,$3)`, variant, sha, review); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
