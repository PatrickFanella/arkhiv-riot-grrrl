package updates

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/catalog"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/config"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/fixtures"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/ingest"
	ia "git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/source/internetarchive"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/storage"
	"github.com/jackc/pgx/v5"
)

func TestRefreshIntegration(t *testing.T) {
	sourceURL := os.Getenv("TEST_DATABASE_URL")
	if sourceURL == "" {
		t.Skip("requires disposable launcher PostgreSQL")
	}
	cfg, err := pgx.ParseConfig(sourceURL)
	if err != nil {
		t.Fatal("invalid test configuration")
	}
	if cfg.Host != "db" || cfg.User != "dev" || cfg.Password != "disposable-dev-only" {
		t.Fatal("isolated test database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	name := fmt.Sprintf("arkhiv_updates_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
	u, _ := url.Parse(sourceURL)
	u.Path = "/" + name
	db, err := catalog.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Pool.Close()
	if err = db.Migrate(ctx, "../../../migrations"); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	c := config.Collection{ID: "updates", Item: "zines", Name: "Fixture zines"}
	metadata := ia.Metadata{Metadata: map[string]any{"title": "Fixture"}, Files: []ia.File{{Name: "first.pdf", Size: "12", MD5: "first"}, {Name: "remove.pdf", Size: "20", MD5: "remove"}}}
	payload := fixtures.PDF("synthetic update test")
	identifiers := []string{"a", "b"}
	broken := false
	throttled := false
	downloadRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/advancedsearch.php" {
			page := 1
			if r.URL.Query().Get("page") == "2" {
				page = 2
			}
			if r.URL.Query().Get("page") == "3" {
				page = 3
			}
			docs := []map[string]string{}
			if page <= len(identifiers) {
				docs = append(docs, map[string]string{"identifier": identifiers[page-1]})
			}
			if broken {
				docs = nil
			}
			json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{"numFound": len(identifiers), "start": page - 1, "docs": docs}})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/download/") {
			downloadRequests++
			if throttled {
				w.Header().Set("Retry-After", "3600")
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Write(payload)
			return
		}
		if broken {
			w.Write([]byte(`{"metadata":`))
			return
		}
		json.NewEncoder(w).Encode(metadata)
	}))
	defer server.Close()
	client := &ia.Client{Base: server.URL, HTTP: server.Client()}
	first, err := Refresh(ctx, c, client, store, db)
	if err != nil || first.Added != 2 {
		t.Fatal(first, err)
	}
	repeated, err := Refresh(ctx, c, client, store, db)
	if err != nil || repeated.Added != 0 || repeated.Content != 0 || repeated.Removed != 0 || repeated.Metadata != 0 || repeated.Pending != first.Pending {
		t.Fatal("no-change", repeated, err)
	}
	queue, err := Queue(ctx, db, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	var addition map[string]any
	for _, b := range queue {
		var v map[string]any
		json.Unmarshal(b, &v)
		if v["file"] == "first.pdf" {
			addition = v
		}
	}
	if err = Review(ctx, db, addition["id"].(string), "reject", "Deliberate exclusion"); err != nil {
		t.Fatal(err)
	}
	if _, err = Refresh(ctx, c, client, store, db); err != nil {
		t.Fatal(err)
	}
	var decision string
	if err = db.Pool.QueryRow(ctx, `SELECT decision FROM source_changes WHERE id=$1`, addition["id"]).Scan(&decision); err != nil || decision != "reject" {
		t.Fatal("exclusion lost", err)
	}
	var baseline string
	db.Pool.QueryRow(ctx, `SELECT snapshot_id FROM source_inventories WHERE collection_id=$1`, c.ID).Scan(&baseline)
	broken = true
	failed, err := Refresh(ctx, c, client, store, db)
	if err == nil || failed.Status != "failed" {
		t.Fatal("accepted partial response", failed, err)
	}
	var after string
	db.Pool.QueryRow(ctx, `SELECT snapshot_id FROM source_inventories WHERE collection_id=$1`, c.ID).Scan(&after)
	if baseline != after {
		t.Fatal("failed refresh advanced baseline")
	}
	broken = false
	metadata.Metadata["title"] = "Updated fixture"
	metadata.Files = []ia.File{{Name: "first.pdf", Size: "12", MD5: "changed"}, {Name: "new.pdf", Size: "13", MD5: "new"}}
	changed, err := Refresh(ctx, c, client, store, db)
	if err != nil || changed.Added != 1 || changed.Content != 1 || changed.Removed != 1 || changed.Metadata != 1 {
		t.Fatal("evolving source", changed, err)
	}
	// A reviewed addition is acquired once and remains unpublished. Byte changes
	// produce a separate edition, retaining the first original and editorial state.
	metadata.Files = []ia.File{{Name: "edition.pdf", Size: fmt.Sprint(len(payload)), MD5: func() string { h := md5.Sum(payload); return hex.EncodeToString(h[:]) }()}}
	if _, err = Refresh(ctx, c, client, store, db); err != nil {
		t.Fatal(err)
	}
	queue, _ = Queue(ctx, db, c.ID)
	var eventID string
	for _, b := range queue {
		var v map[string]any
		json.Unmarshal(b, &v)
		if v["file"] == "edition.pdf" {
			eventID = v["id"].(string)
		}
	}
	if err = Review(ctx, db, eventID, "approve", "Fixture acquisition review"); err != nil {
		t.Fatal(err)
	}
	c.Limits = config.Limits{Documents: 10, TotalBytes: 1 << 20, FileBytes: 1 << 20, Concurrency: 1}
	c.Updates.MinimumFreeBytes = 1
	acquired, err := Acquire(ctx, c, client, store, db)
	if err != nil || acquired.Acquired != 1 {
		t.Fatal("reviewed acquisition", acquired, err)
	}
	var originalID string
	db.Pool.QueryRow(ctx, `SELECT document_id FROM source_changes WHERE id=$1`, eventID).Scan(&originalID)
	if err = db.Edit(ctx, originalID, map[string]any{"title": "Curator title"}, "", "Curator review", nil); err != nil {
		t.Fatal(err)
	}
	yes := true
	if err = db.Edit(ctx, originalID, nil, "", "Reviewed fixture publication", &yes); err != nil {
		t.Fatal(err)
	}
	firstHash, firstSize, _ := storage.Digest(bytes.NewReader(payload))
	payload = fixtures.PDF("changed synthetic update test")
	hash := md5.Sum(payload)
	metadata.Files[0].Size = fmt.Sprint(len(payload))
	metadata.Files[0].MD5 = hex.EncodeToString(hash[:])
	if _, err = Refresh(ctx, c, client, store, db); err != nil {
		t.Fatal(err)
	}
	queue, _ = Queue(ctx, db, c.ID)
	for _, b := range queue {
		var v map[string]any
		json.Unmarshal(b, &v)
		if v["file"] == "edition.pdf" && v["kind"] == "content" {
			if err = Review(ctx, db, v["id"].(string), "approve", "New edition review"); err != nil {
				t.Fatal(err)
			}
		}
	}
	second, err := Acquire(ctx, c, client, store, db)
	if err != nil || second.Acquired != 1 {
		t.Fatal("new edition", second, err)
	}
	again, err := Acquire(ctx, c, client, store, db)
	if err != nil || again.Acquired != 0 {
		t.Fatal("duplicate acquisition", again, err)
	}
	original, err := db.Get(ctx, originalID, false)
	if err != nil || original.Metadata["title"] != "Curator title" || len(original.Variants) != 1 || !original.Published {
		t.Fatal("edition changed editorial original", err)
	}
	if err = store.Verify(firstHash, firstSize); err != nil {
		t.Fatal("old original lost", err)
	}
	var published int
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM documents WHERE published`).Scan(&published)
	if published != 1 {
		t.Fatal("refresh published records")
	}
	// A second collection discovers multiple items, preserving complete query
	// baselines through failures and retaining distinct provenance for shared bytes.
	secondConfig := config.Collection{ID: "second-zines", Name: "Second fixture brand", Accent: "#123456", Discovery: config.Discovery{Query: "collection:fixture", MaxItems: 10, PageSize: 1, MaxRequests: 64}, Limits: c.Limits, Selection: []config.Selection{{Item: "a", Key: "issue", File: "edition.pdf", Title: "Second A"}, {Item: "b", Key: "issue", File: "edition.pdf", Title: "Second B"}}}
	if err = secondConfig.Validate(); err != nil {
		t.Fatal(err)
	}
	multi, err := Refresh(ctx, secondConfig, client, store, db)
	if err != nil || multi.Added != 2 {
		t.Fatal("multi-item discovery", multi, err)
	}
	batch, err := ingest.Acquire(ctx, secondConfig, client, store, db)
	if err != nil || len(batch.Snapshots) != 2 || len(batch.Files) != 2 {
		t.Fatal("multi-item batch", batch, err)
	}
	secondDocs, total, err := db.List(ctx, secondConfig.ID, "", "", "", 50, 0, false)
	if err != nil || total != 2 || secondDocs[0].Item == secondDocs[1].Item || secondDocs[0].Variants[0].SHA != secondDocs[1].Variants[0].SHA {
		t.Fatal("distinct provenance and deduplication", err)
	}
	for _, doc := range secondDocs {
		if doc.Published {
			t.Fatal("second archive auto-published")
		}
	}
	broken = true
	if _, err = Refresh(ctx, secondConfig, client, store, db); err == nil {
		t.Fatal("partial discovery advanced baseline")
	}
	var discovered []byte
	db.Pool.QueryRow(ctx, `SELECT raw FROM source_discoveries WHERE collection_id=$1`, secondConfig.ID).Scan(&discovered)
	var completeDiscovery ia.Discovery
	if err = json.Unmarshal(discovered, &completeDiscovery); err != nil || len(completeDiscovery.Items) != 2 {
		t.Fatal("lost complete discovery baseline", err)
	}
	broken = false
	identifiers = []string{"a", "b", "c"}
	newItem, err := Refresh(ctx, secondConfig, client, store, db)
	if err != nil || newItem.Added != 1 {
		t.Fatal("new item detection", newItem, err)
	}
	secondConfig.Updates.Policy = "acquire"
	additionBatch, err := Acquire(ctx, secondConfig, client, store, db)
	if err != nil || additionBatch.Acquired != 1 || additionBatch.Reused != 2 {
		t.Fatal("new discovered item acquisition", additionBatch, err)
	}
	withAddition, total, err := db.List(ctx, secondConfig.ID, "", "", "", 50, 0, false)
	if err != nil || total != 3 {
		t.Fatal("new item catalog", total, err)
	}
	for _, doc := range withAddition {
		if doc.Published || doc.Variants[0].PublicApproved {
			t.Fatal("discovered item bypassed private review")
		}
	}
	if replay, err := Acquire(ctx, secondConfig, client, store, db); err != nil || replay.Acquired != 0 || replay.Reused != 0 {
		t.Fatal("new item replay duplicated acquisition", replay, err)
	}
	identifiers = []string{"a", "c"}
	removedItem, err := Refresh(ctx, secondConfig, client, store, db)
	if err != nil || removedItem.Removed != 1 {
		t.Fatal("removed item event", removedItem, err)
	}
	retained, err := db.Get(ctx, secondDocs[1].ID, false)
	if err != nil || len(retained.Variants) != 1 {
		t.Fatal("source removal deleted preserved record", err)
	}
	c.Updates.Enabled = false
	if due, status, err := Due(ctx, c, db, true); err != nil || due || status != "disabled" {
		t.Fatal("disabled collection ran", due, status, err)
	}
	c.Updates.Enabled = true
	if due, status, err := Due(ctx, c, db, true); err != nil || due || status != "not-due" {
		t.Fatal("ignored collection cadence", due, status, err)
	}
	backoff := time.Now().Add(time.Hour)
	b, _ := json.Marshal(Result{Status: "failed", RetryAt: &backoff})
	if _, err = db.Pool.Exec(ctx, `INSERT INTO source_refresh_runs(id,collection_id,status,finished_at,result) VALUES('backoff',$1,'failed',now(),$2)`, c.ID, b); err != nil {
		t.Fatal(err)
	}
	if due, status, err := Due(ctx, c, db, false); err != nil || due || status != "source-backoff" {
		t.Fatal("manual request bypassed source backoff", due, status, err)
	}

	// A payload Retry-After survives the process and prevents another payload call.
	rateConfig := config.Collection{ID: "source-backoff", Item: "zines", Name: "Rate-limited fixture", Limits: c.Limits, Updates: config.Updates{Enabled: true, Policy: "acquire", CadenceHours: 24}}
	payload = fixtures.PDF("new rate limited source file")
	md := md5.Sum(payload)
	metadata.Files = []ia.File{{Name: "backoff.pdf", Size: fmt.Sprint(len(payload)), MD5: hex.EncodeToString(md[:])}}
	if _, err := Refresh(ctx, rateConfig, client, store, db); err != nil {
		t.Fatal(err)
	}
	throttled = true
	beforeRequests := downloadRequests
	if _, err := Acquire(ctx, rateConfig, client, store, db); err == nil {
		t.Fatal("payload backoff was ignored")
	}
	if downloadRequests != beforeRequests+1 {
		t.Fatal("requests continued after source backoff")
	}
	if due, status, err := Due(ctx, rateConfig, db, false); err != nil || due || status != "source-backoff" {
		t.Fatal("payload backoff not durable", due, status, err)
	}
	if _, err := Acquire(ctx, rateConfig, client, store, db); err == nil || downloadRequests != beforeRequests+1 {
		t.Fatal("new acquisition bypassed backoff")
	}
	// Expired payload backoff resumes pending acquisition instead of waiting another cadence.
	if _, err := db.Pool.Exec(ctx, `UPDATE source_refresh_runs SET result=jsonb_set(result,'{retry_at}',to_jsonb(now()-interval '1 second')) WHERE collection_id=$1 AND status='failed'`, rateConfig.ID); err != nil {
		t.Fatal(err)
	}
	if due, status, err := Due(ctx, rateConfig, db, true); err != nil || !due || status != "resume-acquisition" {
		t.Fatal("payload retry lost behind cadence", due, status, err)
	}
	throttled = false
	if acquired, err := Acquire(ctx, rateConfig, client, store, db); err != nil || acquired.Acquired != 1 {
		t.Fatal("payload retry did not resume", acquired, err)
	}
	var failedRuns int
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM source_refresh_runs WHERE status='failed'`).Scan(&failedRuns)
	if failedRuns != 4 {
		t.Fatal("missing durable failure receipt")
	}
}
