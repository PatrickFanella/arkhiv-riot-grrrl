// Package catalog owns persistent metadata, publication and editorial history.
package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool       *pgxpool.Pool
	PublicOnly bool
}

func Open(ctx context.Context, url string) (*DB, error) {
	p, e := pgxpool.New(ctx, url)
	if e != nil {
		return nil, e
	}
	if e = p.Ping(ctx); e != nil {
		p.Close()
		return nil, fmt.Errorf("database unavailable")
	}
	return &DB{Pool: p}, nil
}
func ID(parts ...string) string {
	b, _ := json.Marshal(parts)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
func (d *DB) Migrate(ctx context.Context, dir string) error {
	conn, e := d.Pool.Acquire(ctx)
	if e != nil {
		return e
	}
	defer conn.Release()
	if _, e = conn.Exec(ctx, `SELECT pg_advisory_lock(74209113)`); e != nil {
		return e
	}
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock(74209113)`)
	if _, e = conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_versions(name text PRIMARY KEY, sha256 text NOT NULL)`); e != nil {
		return e
	}
	paths, e := filepath.Glob(filepath.Join(dir, "*.sql"))
	if e != nil {
		return e
	}
	if len(paths) == 0 {
		return fmt.Errorf("no migrations in %s", dir)
	}
	sort.Strings(paths)
	for _, path := range paths {
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		sum := sha256.Sum256(b)
		hash := hex.EncodeToString(sum[:])
		name := filepath.Base(path)
		var old string
		e = conn.QueryRow(ctx, `SELECT sha256 FROM schema_versions WHERE name=$1`, name).Scan(&old)
		if e == nil {
			if old != hash {
				return fmt.Errorf("applied migration changed: %s", name)
			}
			continue
		}
		if e != pgx.ErrNoRows {
			return e
		}
		tx, e := conn.Begin(ctx)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, string(b))
		if e == nil {
			_, e = tx.Exec(ctx, `INSERT INTO schema_versions VALUES($1,$2)`, name, hash)
		}
		if e != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", name, e)
		}
		if e = tx.Commit(ctx); e != nil {
			return e
		}
	}
	return nil
}
func (d *DB) Collection(ctx context.Context, c config.Collection) error {
	b, e := json.Marshal(c)
	if e != nil {
		return e
	}
	_, e = d.Pool.Exec(ctx, `INSERT INTO collections(id,config) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET config=excluded.config,updated_at=now()`, c.ID, b)
	return e
}
func (d *DB) Snapshot(ctx context.Context, item string, raw []byte) (string, error) {
	id := ID(item, string(raw))
	_, e := d.Pool.Exec(ctx, `INSERT INTO source_snapshots(id,item_id,raw) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, id, item, raw)
	return id, e
}
func (d *DB) Attempt(ctx context.Context, c, item, file, status, detail string) error {
	_, e := d.Pool.Exec(ctx, `INSERT INTO acquisition_attempts(collection_id,source_item,filename,status,detail) VALUES($1,$2,$3,$4,$5)`, c, item, file, status, detail)
	return e
}
func (d *DB) Import(ctx context.Context, c config.Collection, s config.Selection, sha string, size int64, snapshot string) error {
	tx, e := d.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	id := ID(c.ID, c.Item, s.Key)
	var published, same bool
	e = tx.QueryRow(ctx, `SELECT published, EXISTS(SELECT 1 FROM variants WHERE document_id=$1 AND filename=$2 AND asset_sha=$3) FROM documents WHERE id=$1 FOR UPDATE`, id, s.File, sha).Scan(&published, &same)
	if e != nil && e != pgx.ErrNoRows {
		return e
	}
	if published && !same {
		return fmt.Errorf("published document changes require a separate unpublished edition")
	}
	b, e := json.Marshal(s)
	if e != nil {
		return e
	}
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO assets(sha256,size) VALUES($1,$2) ON CONFLICT DO NOTHING`, []any{sha, size}},
		{`INSERT INTO documents(id,collection_id,source_item,source_key,metadata,rights) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(collection_id,source_item,source_key) DO UPDATE SET metadata=excluded.metadata,updated_at=now()`, []any{id, c.ID, c.Item, s.Key, b, defaultRights(s.Rights)}},
		{`INSERT INTO variants(id,document_id,filename,asset_sha,source_snapshot) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, []any{ID(id, s.File, sha), id, s.File, sha, snapshot}},
	} {
		if _, e = tx.Exec(ctx, q.sql, q.args...); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
func defaultRights(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

type Variant struct {
	PublicApproved bool   `json:"public_approved"`
	ID             string `json:"id"`
	Filename       string `json:"filename"`
	SHA            string `json:"sha256"`
	Size           int64  `json:"size"`
	Pages          int    `json:"pages"`
	TextStatus     string `json:"text_status"`
}
type Document struct {
	ID           string         `json:"id"`
	Collection   string         `json:"collection"`
	Item         string         `json:"source_item"`
	Key          string         `json:"key"`
	Metadata     map[string]any `json:"metadata"`
	OriginalDate string         `json:"original_date"`
	Published    bool           `json:"published"`
	Rights       string         `json:"rights"`
	Review       string         `json:"review,omitempty"`
	Variants     []Variant      `json:"variants"`
	Hits         []Hit          `json:"hits"`
}
type Hit struct {
	Page    int    `json:"page"`
	Variant string `json:"variant"`
	Snippet string `json:"snippet"`
}

func (d *DB) Get(ctx context.Context, id string, public bool) (Document, error) {
	var doc Document
	var b []byte
	e := d.Pool.QueryRow(ctx, `SELECT id,collection_id,source_item,source_key,metadata||overrides,CASE WHEN jsonb_typeof(metadata->'date')='string' THEN metadata->>'date' ELSE '' END,published,rights,review_note FROM documents WHERE id=$1 AND (NOT $2 OR published) AND (NOT $3 OR (published AND EXISTS(SELECT 1 FROM variants WHERE document_id=documents.id AND public_approved)))`, id, public, d.PublicOnly).Scan(&doc.ID, &doc.Collection, &doc.Item, &doc.Key, &b, &doc.OriginalDate, &doc.Published, &doc.Rights, &doc.Review)
	if e != nil {
		return doc, e
	}
	if e = json.Unmarshal(b, &doc.Metadata); e != nil {
		return doc, e
	}
	if public {
		doc.Review = ""
	}
	if d.PublicOnly {
		metadata := map[string]any{}
		for _, key := range []string{"title", "creator", "date", "language", "subjects"} {
			if value, ok := doc.Metadata[key]; ok {
				metadata[key] = value
			}
		}
		doc.Metadata = metadata
	}

	doc.Variants = []Variant{}
	doc.Hits = []Hit{}
	rows, e := d.Pool.Query(ctx, `SELECT v.id,v.filename,v.asset_sha,a.size,COALESCE(x.pages,0),COALESCE(x.status,'pending'),v.public_approved FROM variants v JOIN assets a ON a.sha256=v.asset_sha LEFT JOIN derivatives x ON x.asset_sha=v.asset_sha WHERE document_id=$1 AND (NOT $2 OR v.public_approved) ORDER BY v.filename,v.id`, id, d.PublicOnly)
	if e != nil {
		return doc, e
	}
	defer rows.Close()
	for rows.Next() {
		var v Variant
		if e = rows.Scan(&v.ID, &v.Filename, &v.SHA, &v.Size, &v.Pages, &v.TextStatus, &v.PublicApproved); e != nil {
			return doc, e
		}
		doc.Variants = append(doc.Variants, v)
	}
	return doc, rows.Err()
}
func (d *DB) Edit(ctx context.Context, id string, overrides map[string]any, rights, review string, publish *bool) error {
	tx, e := d.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var oldReview, oldRights string
	if e = tx.QueryRow(ctx, `SELECT review_note,rights FROM documents WHERE id=$1 FOR UPDATE`, id).Scan(&oldReview, &oldRights); e != nil {
		return e
	}
	if rights == "" {
		rights = oldRights
	}
	if review == "" {
		review = oldReview
	}
	if publish != nil && *publish {
		var n int
		if e = tx.QueryRow(ctx, `SELECT count(*) FROM variants v JOIN derivatives x ON x.asset_sha=v.asset_sha WHERE v.document_id=$1 AND x.pages>0 AND x.status<>'failed'`, id).Scan(&n); e != nil {
			return e
		}
		var all int
		if e = tx.QueryRow(ctx, `SELECT count(*) FROM variants WHERE document_id=$1`, id).Scan(&all); e != nil {
			return e
		}
		if n == 0 || n != all || review == "" {
			return fmt.Errorf("verified asset and review note required")
		}
	}
	b, e := json.Marshal(overrides)
	if e != nil {
		return e
	}
	if overrides == nil {
		b = []byte(`{}`)
	}
	_, e = tx.Exec(ctx, `UPDATE documents SET overrides=overrides||$2::jsonb,rights=$3,review_note=$4,published=COALESCE($5,published),updated_at=now() WHERE id=$1`, id, b, rights, review, publish)
	if e != nil {
		return e
	}
	event, _ := json.Marshal(map[string]any{"overrides": overrides, "rights": rights, "review": review, "published": publish})
	if _, e = tx.Exec(ctx, `INSERT INTO review_events(document_id,action,detail) VALUES($1,'edit',$2)`, id, event); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (d *DB) List(ctx context.Context, c, q, subject, language string, limit, offset int, public bool, filters ...BrowseFilters) ([]Document, int, error) {
	var filter BrowseFilters
	if len(filters) > 0 {
		filter = filters[0]
	}
	if err := filter.Validate(); err != nil {
		return nil, 0, err
	}
	const where = `FROM documents d WHERE ($1='' OR collection_id=$1) AND (NOT $2 OR published) AND (NOT $6 OR (published AND EXISTS(SELECT 1 FROM variants WHERE document_id=d.id AND public_approved))) AND ($3='' OR to_tsvector('simple',(CASE WHEN $6 THEN jsonb_build_object('title',(metadata||overrides)->'title','creator',(metadata||overrides)->'creator','date',(metadata||overrides)->'date','language',(metadata||overrides)->'language','subjects',(metadata||overrides)->'subjects') ELSE metadata||overrides END)::text) @@ plainto_tsquery('simple',$3) OR EXISTS(SELECT 1 FROM variants v JOIN page_text t ON t.asset_sha=v.asset_sha WHERE v.document_id=d.id AND (NOT $6 OR v.public_approved) AND t.search @@ plainto_tsquery('simple',$3))) AND ($4='' OR (metadata||overrides)->'subjects' ? $4) AND ($5='' OR (metadata||overrides)->>'language'=$5)`
	const additional = ` AND ($7='' OR position(lower($7) in lower(COALESCE((metadata||overrides)->>'creator','')))>0) AND ($8='' OR (metadata->>'date' ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$' AND metadata->>'date'>=$8)) AND ($9='' OR (metadata->>'date' ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$' AND metadata->>'date'<=$9))`
	args := []any{c, public, q, subject, language, d.PublicOnly, filter.Creator, filter.DateFrom, filter.DateTo}
	var total int
	if e := d.Pool.QueryRow(ctx, `SELECT count(*) `+where+additional, args...).Scan(&total); e != nil {
		return nil, 0, e
	}
	rows, e := d.Pool.Query(ctx, `SELECT id `+where+additional+` ORDER BY lower(COALESCE((metadata||overrides)->>'title','')),id LIMIT $10 OFFSET $11`, append(args, limit, offset)...)
	if e != nil {
		return nil, 0, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, 0, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, 0, e
	}
	docs := []Document{}
	for _, id := range ids {
		doc, e := d.Get(ctx, id, public)
		if e != nil {
			return nil, 0, e
		}
		if q != "" {
			rs, e := d.Pool.Query(ctx, `SELECT t.page,v.id,left(t.text,350) FROM variants v JOIN page_text t ON t.asset_sha=v.asset_sha WHERE v.document_id=$1 AND (NOT $3 OR v.public_approved) AND t.search @@ plainto_tsquery('simple',$2) ORDER BY t.page,v.id LIMIT 5`, id, q, d.PublicOnly)
			if e != nil {
				return nil, 0, e
			}
			for rs.Next() {
				var h Hit
				if e = rs.Scan(&h.Page, &h.Variant, &h.Snippet); e != nil {
					rs.Close()
					return nil, 0, e
				}
				doc.Hits = append(doc.Hits, h)
			}
			e = rs.Err()
			rs.Close()
			if e != nil {
				return nil, 0, e
			}
		}
		docs = append(docs, doc)
	}
	return docs, total, nil
}
func (d *DB) Collections(ctx context.Context) ([]json.RawMessage, error) {
	rows, e := d.Pool.Query(ctx, `SELECT jsonb_build_object('id',id,'name',config->'name','introduction',config->'introduction','accent',config->'accent') FROM collections c WHERE NOT $1 OR EXISTS(SELECT 1 FROM documents d JOIN variants v ON v.document_id=d.id WHERE d.collection_id=c.id AND d.published AND v.public_approved) ORDER BY id`, d.PublicOnly)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b []byte
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		out = append(out, json.RawMessage(b))
	}
	return out, rows.Err()
}

// Group moves an existing variant within a collection. Both records must be
// unpublished so an editorial change cannot silently alter a published document.
func (d *DB) Group(ctx context.Context, variant, target, review string) error {
	if review == "" {
		return fmt.Errorf("grouping review required")
	}
	tx, e := d.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var source string
	if e = tx.QueryRow(ctx, `SELECT document_id FROM variants WHERE id=$1 FOR UPDATE`, variant).Scan(&source); e != nil {
		return e
	}
	rows, e := tx.Query(ctx, `SELECT id,collection_id,published FROM documents WHERE id=ANY($1) ORDER BY id FOR UPDATE`, []string{source, target})
	if e != nil {
		return e
	}
	collections := map[string]string{}
	published := false
	for rows.Next() {
		var id, c string
		var p bool
		if e = rows.Scan(&id, &c, &p); e != nil {
			rows.Close()
			return e
		}
		collections[id] = c
		published = published || p
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if published || collections[target] == "" || collections[source] != collections[target] {
		return fmt.Errorf("grouping requires unpublished documents in the same collection")
	}
	if _, e = tx.Exec(ctx, `UPDATE variants SET document_id=$2 WHERE id=$1`, variant, target); e != nil {
		return e
	}
	b, _ := json.Marshal(map[string]string{"variant": variant, "from": source, "review": review})
	if _, e = tx.Exec(ctx, `INSERT INTO review_events(document_id,action,detail) VALUES($1,'group',$2)`, target, b); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
