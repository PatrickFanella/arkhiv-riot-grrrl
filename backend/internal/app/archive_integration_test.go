package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/catalog"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/config"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/fixtures"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/index"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/recovery"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/storage"
	"github.com/jackc/pgx/v5"
	"net/http/httptest"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestArchiveIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("disposable PostgreSQL companion gate requires TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cfg, e := pgx.ParseConfig(url)
	if e != nil {
		t.Fatal("invalid test database URL")
	}
	if cfg.Host != "db" || cfg.User != "dev" || cfg.Password != "disposable-dev-only" {
		t.Fatal("test database must be the isolated launcher service")
	}
	admin, e := pgx.ConnectConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close(context.Background())
	create := func(label string) string {
		name := fmt.Sprintf("arkhiv_test_%s_%d", label, time.Now().UnixNano())
		if _, e := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() {
			admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		})
		u, _ := urlpkg.Parse(url)
		u.Path = "/" + name
		return u.String()
	}
	sourceURL := create("source")
	db, e := catalog.Open(ctx, sourceURL)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Pool.Close()
	migrations := "../../../migrations"
	if e = db.Migrate(ctx, migrations); e != nil {
		t.Fatal(e)
	}
	var concurrent sync.WaitGroup
	errors := make(chan error, 2)
	for range 2 {
		concurrent.Go(func() { errors <- db.Migrate(ctx, migrations) })
	}
	concurrent.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal("concurrent migration", err)
		}
	}
	if e = db.Migrate(ctx, migrations); e != nil {
		t.Fatal(e)
	}
	store, e := storage.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	t.Setenv("ARKHIV_FIXTURE", "1")
	if e = fixtures.Seed(ctx, db, store); e != nil {
		t.Fatal(e)
	}
	// Generated with pypdf: a blank page locked with a synthetic test password.
	encrypted, e := base64.StdEncoding.DecodeString("JVBERi0xLjMKJeLjz9MKMSAwIG9iago8PAovUHJvZHVjZXIgPGVlMTEyNjg1ZWQ+Cj4+CmVuZG9iagoyIDAgb2JqCjw8Ci9UeXBlIC9QYWdlcwovQ291bnQgMQovS2lkcyBbIDQgMCBSIF0KPj4KZW5kb2JqCjMgMCBvYmoKPDwKL1R5cGUgL0NhdGFsb2cKL1BhZ2VzIDIgMCBSCj4+CmVuZG9iago0IDAgb2JqCjw8Ci9UeXBlIC9QYWdlCi9SZXNvdXJjZXMgPDwKPj4KL01lZGlhQm94IFsgMC4wIDAuMCA2MTIgNzkyIF0KL1BhcmVudCAyIDAgUgo+PgplbmRvYmoKNSAwIG9iago8PAovViAyCi9SIDMKL0xlbmd0aCAxMjgKL1AgNDI5NDk2NzI5MgovRmlsdGVyIC9TdGFuZGFyZAovTyA8YWJhNjE5NGY5ODI5ZWRhOTM3MTk0NDJjMjE5NTUwMTU3ZWQwNTBjMDRhZjRiYWMyZTEwOTkyMzY2MDcyZGU0MD4KL1UgPDFhNDk2NzU4YTg3MzQ2YzY5ZTU2YjkxM2U0NjQxM2ZkMjhiZjRlNWU0ZTc1OGE0MTY0MDA0ZTU2ZmZmYTAxMDg+Cj4+CmVuZG9iagp4cmVmCjAgNgowMDAwMDAwMDAwIDY1NTM1IGYgCjAwMDAwMDAwMTUgMDAwMDAgbiAKMDAwMDAwMDA1OSAwMDAwMCBuIAowMDAwMDAwMTE4IDAwMDAwIG4gCjAwMDAwMDAxNjcgMDAwMDAgbiAKMDAwMDAwMDI2MSAwMDAwMCBuIAp0cmFpbGVyCjw8Ci9TaXplIDYKL1Jvb3QgMyAwIFIKL0luZm8gMSAwIFIKL0lEIFsgPDM1NjEzMTMyNjIzNzY0MzczODM1NjEzNjY0MzUzNTM3MzUzNjM5NjI2MjM3MzA2NDMyMzQzMjMyNjEzNzMwMzk+IDwzNTYxMzEzMjYyMzc2NDM3MzgzNTYxMzY2NDM1MzUzNzM1MzYzOTYyNjIzNzMwNjQzMjM0MzIzMjYxMzczMDM5PiBdCi9FbmNyeXB0IDUgMCBSCj4+CnN0YXJ0eHJlZgo0NzYKJSVFT0YK")
	if e != nil {
		t.Fatal(e)
	}
	for _, malformed := range [][]byte{[]byte("%PDF-1.4\ninvalid synthetic document"), encrypted} {
		mh, mn, _ := storage.Digest(bytes.NewReader(malformed))
		if e = os.WriteFile(filepath.Join(store.Path, "malformed.part"), malformed, 0600); e != nil {
			t.Fatal(e)
		}
		if e = store.Promote("malformed.part", mh, mn); e != nil {
			t.Fatal(e)
		}
		ms, _ := db.Snapshot(ctx, "synthetic", []byte(`{"malformed":true}`))
		if e = db.Import(ctx, config.Collection{ID: "fixture-zines", Item: "synthetic"}, config.Selection{Key: "Unreadable fixture " + mh, File: "malformed.pdf", Title: "Malformed fixture"}, mh, mn, ms); e != nil {
			t.Fatal(e)
		}
		if index.Extract(ctx, db, store, mh) == nil {
			t.Fatal("malformed PDF extracted successfully")
		}
		var failedStatus string
		if e = db.Pool.QueryRow(ctx, `SELECT status FROM derivatives WHERE asset_sha=$1`, mh).Scan(&failedStatus); e != nil || failedStatus != "failed" {
			t.Fatal("missing explicit failed extraction state", e)
		}
		if e = store.Verify(mh, mn); e != nil {
			t.Fatal("failed extraction changed original", e)
		}

	}

	blank, e := db.Get(ctx, catalog.ID("fixture-zines", "synthetic", "No-text fixture"), false)
	if e != nil || blank.Variants[0].TextStatus != "no-text" {
		t.Fatal("missing text state", e)
	}
	companion := []byte("OCR fallback unpagedword")
	ch, cn, _ := storage.Digest(bytes.NewReader(companion))
	os.WriteFile(filepath.Join(store.Path, "ocr.part"), companion, 0600)
	if e = store.Promote("ocr.part", ch, cn); e != nil {
		t.Fatal(e)
	}
	_, e = db.Pool.Exec(ctx, `INSERT INTO assets(sha256,size) VALUES($1,$2)`, ch, cn)
	if e != nil {
		t.Fatal(e)
	}
	snapOCR, _ := db.Snapshot(ctx, "synthetic", []byte(`{"ocr":true}`))
	_, e = db.Pool.Exec(ctx, `INSERT INTO asset_companions(asset_sha,companion_sha,filename,source_snapshot) VALUES($1,$2,'fixture_djvu.txt',$3)`, blank.Variants[0].SHA, ch, snapOCR)
	if e != nil {
		t.Fatal(e)
	}
	if e = index.Extract(ctx, db, store, blank.Variants[0].SHA); e != nil {
		t.Fatal(e)
	}
	var page int
	if e = db.Pool.QueryRow(ctx, `SELECT page FROM page_text WHERE asset_sha=$1`, blank.Variants[0].SHA).Scan(&page); e != nil || page != 0 {
		t.Fatal("OCR invented a page mapping", e)
	}
	handler := ArchiveHandler(db, store)
	request := func(method, path, rangeValue string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		if rangeValue != "" {
			r.Header.Set("Range", rangeValue)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	id := catalog.ID("fixture-zines", "synthetic", "Mutual aid fixture")
	doc, e := db.Get(ctx, id, true)
	if e != nil {
		t.Fatal(e)
	}
	asset := doc.Variants[0]
	w := request("GET", "/api/documents?collection=fixture-zines&q=knownword", "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var result struct {
		Total     int
		Documents []catalog.Document
	}
	if e = json.Unmarshal(w.Body.Bytes(), &result); e != nil {
		t.Fatal(e)
	}
	if result.Total != 1 || len(result.Documents[0].Hits) == 0 || result.Documents[0].Hits[0].Page != 2 {
		t.Fatalf("page-search result: %s", w.Body.String())
	}
	privateID := catalog.ID("fixture-zines", "synthetic", "Private fixture")
	private, e := db.Get(ctx, privateID, false)
	if e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{"/api/documents/" + privateID, "/api/assets/" + private.Variants[0].ID, "/api/assets/invalid"} {
		if request("GET", path, "").Code != 404 {
			t.Fatal("unpublished asset leak", path)
		}
	}
	for _, path := range []string{"/api/documents?limit=51", "/api/documents?offset=-1"} {
		if request("GET", path, "").Code != 400 {
			t.Fatal("invalid query accepted")
		}
	}
	w = request("GET", "/api/assets/"+asset.ID, "bytes=0-4")
	if w.Code != 206 || w.Body.String() != "%PDF-" || !strings.HasPrefix(w.Header().Get("Content-Range"), "bytes 0-4/") {
		t.Fatal("range contract", w.Code, w.Body.String())
	}
	w = request("HEAD", "/api/assets/"+asset.ID, "")
	if w.Code != 200 || w.Body.Len() != 0 || w.Header().Get("Content-Type") != "application/pdf" || w.Header().Get("Content-Length") != fmt.Sprint(asset.Size) || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("HEAD contract")
	}
	etag := w.Header().Get("ETag")
	if etag != `"`+asset.SHA+`"` {
		t.Fatal("original ETag contract")
	}
	conditional := httptest.NewRequest("GET", "/api/assets/"+asset.ID, nil)
	conditional.Header.Set("If-None-Match", etag)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, conditional)
	if w.Code != 304 || w.Body.Len() != 0 {
		t.Fatal("conditional request contract")
	}
	w = request("GET", "/api/assets/"+asset.ID, fmt.Sprintf("bytes=%d-", asset.Size))
	if w.Code != 416 || w.Header().Get("Content-Range") != fmt.Sprintf("bytes */%d", asset.Size) {
		t.Fatal("invalid range contract")
	}
	w = request("GET", "/api/assets/"+asset.ID+"?download=1", "")
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("download contract")
	}
	h, n, _ := storage.Digest(strings.NewReader(w.Body.String()))
	if h != asset.SHA || n != asset.Size {
		t.Fatal("download differs from original")
	}
	for _, query := range []string{"collection=fixture-zines&q=absentunique", "collection=missing", "collection=fixture-zines&offset=1"} {
		w = request("GET", "/api/documents?"+query, "")
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Documents) != 0 {
			t.Fatal("empty/scoped/paginated catalog contract", query, w.Body.String())
		}
	}
	if e = db.Edit(ctx, privateID, map[string]any{"subjects": []string{"private-only"}}, "", "", nil); e != nil {
		t.Fatal(e)
	}
	w = request("GET", "/api/facets?collection=fixture-zines", "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "private-only") || !strings.Contains(w.Body.String(), "mutual aid") {
		t.Fatal("published-only facets contract")
	}
	if e = db.Edit(ctx, id, map[string]any{"creators": "Émilie Solidarität", "date": "unknown"}, "", "", nil); e != nil {
		t.Fatal(e)
	}
	w = request("GET", "/api/documents?collection=fixture-zines&q="+urlpkg.QueryEscape("Solidarität"), "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Total != 1 {
		t.Fatal("Unicode metadata search contract", w.Body.String())
	}
	// Rebuild the searchable derivative from the preserved original.
	if _, e = db.Pool.Exec(ctx, `DELETE FROM page_text WHERE asset_sha=$1`, asset.SHA); e != nil {
		t.Fatal(e)
	}
	if e = index.Extract(ctx, db, store, asset.SHA); e != nil {
		t.Fatal(e)
	}
	w = request("GET", "/api/assets/"+asset.ID+"/search?q=knownword", "")
	var hits []catalog.Hit
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &hits) != nil || len(hits) != 1 || hits[0].Page != 2 {
		t.Fatal("rebuilt page search contract")
	}
	if e = store.Verify(asset.SHA, asset.Size); e != nil {
		t.Fatal("rebuild changed original", e)
	}
	// The scheduled audit must detect missing reader manifests, not only originals.
	if result, err := recovery.Inspect(ctx, db, store); err != nil || result.Published != 2 {
		t.Fatal("readable audit", result, err)
	}
	manifestPath := filepath.Join(store.Path, "derivatives", asset.SHA, "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(manifestPath, []byte(`{"input_sha256":"wrong","pages":2}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = recovery.Inspect(ctx, db, store); err == nil {
		t.Fatal("audit accepted wrong derivative input")
	}
	if err = os.Remove(manifestPath); err != nil {
		t.Fatal(err)
	}
	if _, err = recovery.Inspect(ctx, db, store); err == nil {
		t.Fatal("audit accepted missing reader manifest")
	}
	if err = os.WriteFile(manifestPath, manifestBytes, 0600); err != nil {
		t.Fatal(err)
	}
	if e = db.Edit(ctx, id, map[string]any{"title": "Reviewed title"}, "", "", nil); e != nil {
		t.Fatal(e)
	}
	c := config.Collection{ID: "fixture-zines", Item: "synthetic"}
	snap, _ := db.Snapshot(ctx, "synthetic", []byte(`{"refresh":true}`))
	if e = db.Import(ctx, c, config.Selection{Key: "Mutual aid fixture", File: asset.Filename, Title: "Source refreshed"}, asset.SHA, asset.Size, snap); e != nil {
		t.Fatal(e)
	}
	refreshed, e := db.Get(ctx, id, true)
	if e != nil || refreshed.Metadata["title"] != "Reviewed title" || !refreshed.Published || len(refreshed.Variants) != 1 {
		t.Fatal("refresh lost editorial state", e)
	}

	// Public access is a separate exact-variant decision, never inferred from rights or private publication.
	if err := db.Edit(ctx, id, nil, "unknown", "owner access review; no reuse license", nil); err != nil {
		t.Fatal(err)
	}
	publicDB := &catalog.DB{Pool: db.Pool, PublicOnly: true}
	publicHandler := ArchiveHandler(publicDB, store)
	publicRequest := func(method, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		r.Header.Set("If-None-Match", etag)
		r.Header.Set("Range", "bytes=0-4")
		w := httptest.NewRecorder()
		publicHandler.ServeHTTP(w, r)
		return w
	}
	deny := func(v string) {
		for _, suffix := range []string{"", "?download=1", "/cover", "/search?q=knownword"} {
			for _, method := range []string{"GET", "HEAD"} {
				if w := publicRequest(method, "/api/assets/"+v+suffix); w.Code != 404 || w.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("public asset leak", method, suffix, w.Code)
				}
			}
		}
	}
	deny(asset.ID)
	deny(private.Variants[0].ID)
	deny("unknown")
	if w := publicRequest("GET", "/api/collections"); w.Body.String() != "[]\n" {
		t.Fatal("unapproved collection leak", w.Body.String())
	}
	if w := publicRequest("GET", "/api/documents?collection=fixture-zines&q=knownword"); !strings.Contains(w.Body.String(), `"total":0`) {
		t.Fatal("unapproved search leak", w.Body.String())
	}
	review := catalog.PublicReview{Format: 1, Collection: "fixture-zines", PublicURL: "https://example.invalid", Contact: "info@example.invalid", Approval: "owner approved archival access; no reuse license", Documents: []catalog.PublicDocument{{ID: id, Source: "synthetic", Approved: true, Rights: "unknown", Variants: []catalog.Variant{asset}}}}
	raw, _ := json.Marshal(review)
	wrong := review
	wrong.Documents = append([]catalog.PublicDocument(nil), review.Documents...)
	wrong.Documents[0].Variants = append([]catalog.Variant(nil), review.Documents[0].Variants...)
	wrong.Documents[0].Variants[0].SHA = strings.Repeat("0", 64)
	bad, _ := json.Marshal(wrong)
	if _, err := db.ApprovePublic(ctx, store, bad, "owner review"); err == nil {
		t.Fatal("approved a mismatched original")
	}
	deny(asset.ID)
	if err := os.Remove(manifestPath); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ApprovePublic(ctx, store, raw, "owner review"); err == nil {
		t.Fatal("approved missing readable derivative")
	}
	if err := os.WriteFile(manifestPath, manifestBytes, 0600); err != nil {
		t.Fatal(err)
	}
	if count, err := db.ApprovePublic(ctx, store, raw, "owner reviewed fixture"); err != nil || count != 1 {
		t.Fatal("public approval", count, err)
	}
	if w := publicRequest("GET", "/api/documents/"+id); w.Code != 200 || strings.Contains(w.Body.String(), "owner reviewed fixture") || strings.Contains(w.Body.String(), `"file":`) || strings.Contains(w.Body.String(), "synthetic test fixture") {
		t.Fatal("approved public document", w.Body.String())
	}
	if w := publicRequest("GET", "/api/documents?collection=fixture-zines&q=knownword"); !strings.Contains(w.Body.String(), `"total":1`) {
		t.Fatal("approved search", w.Body.String())
	}
	if w := publicRequest("GET", "/api/collections"); strings.Contains(w.Body.String(), "limits") || strings.Contains(w.Body.String(), "updates") || strings.Contains(w.Body.String(), "discovery") || strings.Contains(w.Body.String(), "selection") {
		t.Fatal("private config leak")
	}
	// An unapproved sibling can share bytes, but has no public variant identity.
	sibling := catalog.ID(id, "unapproved-sibling")
	if _, err := db.Pool.Exec(ctx, `INSERT INTO variants(id,document_id,filename,asset_sha,source_snapshot) VALUES($1,$2,'unapproved.pdf',$3,$4)`, sibling, id, private.Variants[0].SHA, snap); err != nil {
		t.Fatal(err)
	}
	deny(sibling)
	if w := publicRequest("GET", "/api/documents?collection=fixture-zines&q=Private"); !strings.Contains(w.Body.String(), `"total":0`) {
		t.Fatal("unapproved sibling text leak", w.Body.String())
	}
	if w := publicRequest("GET", "/api/documents/"+id); strings.Contains(w.Body.String(), sibling) {
		t.Fatal("unapproved sibling leak")
	}
	if err := db.RevokePublic(ctx, asset.ID, "owner removal request"); err != nil {
		t.Fatal(err)
	}
	deny(asset.ID)
	for _, path := range []string{"/api/documents/" + id, "/api/documents?collection=fixture-zines&q=knownword", "/api/facets?collection=fixture-zines", "/api/collections"} {
		w := publicRequest("GET", path)
		if strings.Contains(w.Body.String(), "Reviewed title") || strings.Contains(w.Body.String(), "mutual aid") || strings.Contains(w.Body.String(), "fixture-zines") {
			t.Fatal("revoked catalog leak", path, w.Body.String())
		}
	}
	if err := store.Verify(asset.SHA, asset.Size); err != nil {
		t.Fatal("revocation changed preserved original", err)
	}
	var decisions int
	if err := db.Pool.QueryRow(ctx, `SELECT count(*) FROM public_access_events WHERE variant_id=$1`, asset.ID).Scan(&decisions); err != nil || decisions != 2 {
		t.Fatal("missing access decision history", decisions, err)
	}
	if _, err := db.Pool.Exec(ctx, `DELETE FROM variants WHERE id=$1`, sibling); err != nil {
		t.Fatal(err)
	}

	// Missing derivatives must prevent publication even with a review note.
	_, e = db.Pool.Exec(ctx, `DELETE FROM derivatives WHERE asset_sha=$1`, private.Variants[0].SHA)
	if e != nil {
		t.Fatal(e)
	}
	yes := true
	if db.Edit(ctx, privateID, nil, "", "review", &yes) == nil {
		t.Fatal("published incomplete document")
	}
	if db.Group(ctx, private.Variants[0].ID, id, "review") == nil {
		t.Fatal("grouped into a published document")
	}
	no := false
	if e = db.Edit(ctx, id, nil, "", "", &no); e != nil {
		t.Fatal(e)
	}
	if e = db.Group(ctx, private.Variants[0].ID, id, "variant review"); e != nil {
		t.Fatal(e)
	}
	backup := filepath.Join(t.TempDir(), "backup")
	if e = recovery.Backup(ctx, db, store, sourceURL, backup); e != nil {
		t.Fatal(e)
	}
	if err := filepath.WalkDir(backup, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		return os.Chmod(path, 0400)
	}); err != nil {
		t.Fatal(err)
	}
	// Restore a real PostgreSQL dump from a deduplicated snapshot, while keeping
	// the earlier immutable snapshot and rejecting a bounded partial dump.
	var sourceBytes int64
	if err := filepath.WalkDir(store.Path, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || path == filepath.Join(store.Path, ".writer.lock") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		sourceBytes += info.Size()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	partial := filepath.Join(t.TempDir(), "bounded-failure")
	if err := recovery.Backup(ctx, db, store, sourceURL, partial, recovery.BackupOptions{Basis: backup, MaximumBytes: sourceBytes + 1}); err == nil {
		t.Fatal("accepted an over-budget catalog dump")
	}
	if _, err := os.Stat(filepath.Join(partial, "complete.json")); !os.IsNotExist(err) {
		t.Fatal("partial backup became complete", err)
	}
	if info, err := os.Stat(filepath.Join(partial, "catalog.dump")); err != nil || info.Size() > 1 {
		t.Fatal("catalog dump exceeded its budget", err)
	}
	deduplicated := filepath.Join(t.TempDir(), "deduplicated")
	if err := recovery.Backup(ctx, db, store, sourceURL, deduplicated, recovery.BackupOptions{Basis: backup, MaximumBytes: sourceBytes + 64<<20}); err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(filepath.Join(backup, "archive"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(filepath.Join(backup, "archive"), path)
		if err != nil {
			return err
		}
		first, err := os.Stat(path)
		if err != nil {
			return err
		}
		second, err := os.Stat(filepath.Join(deduplicated, "archive", relative))
		if err != nil {
			return err
		}
		if !os.SameFile(first, second) {
			t.Fatal("unchanged preserved file was recopied", relative)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	backup = deduplicated
	restoredURL := create("restore")
	destination := filepath.Join(t.TempDir(), "restored")
	if e = recovery.Restore(ctx, restoredURL, backup, destination); e != nil {
		t.Fatal(e)
	}
	restored, e := catalog.Open(ctx, restoredURL)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Pool.Close()
	d, e := restored.Get(ctx, id, false)
	if e != nil || d.Metadata["title"] != "Reviewed title" || len(d.Variants) != 2 {
		t.Fatal("restore lost records", e)
	}
	if recovery.Restore(ctx, restoredURL, backup, destination) == nil {
		t.Fatal("restored over existing state")
	}
	os.WriteFile(filepath.Join(backup, "catalog.dump"), []byte("corrupt"), 0600)
	if recovery.Restore(ctx, create("corrupt"), backup, filepath.Join(t.TempDir(), "restore")) == nil {
		t.Fatal("accepted corrupt catalog backup")
	}
}
