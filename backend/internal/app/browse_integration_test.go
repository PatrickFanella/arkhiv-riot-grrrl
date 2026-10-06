package app

import (
	"context"
	"fmt"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/catalog"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/config"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/fixtures"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/storage"
	"github.com/jackc/pgx/v5"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestBrowseFiltersAcrossPages(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires disposable TEST_DATABASE_URL")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil || cfg.Host != "db" || cfg.User != "dev" || cfg.Password != "disposable-dev-only" {
		t.Fatal("requires isolated launcher database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	name := fmt.Sprintf("arkhiv_browse_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
	u, _ := url.Parse(dsn)
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
	t.Setenv("ARKHIV_FIXTURE", "1")
	if err = fixtures.Seed(ctx, db, store); err != nil {
		t.Fatal(err)
	}
	fixture, err := db.Get(ctx, catalog.ID("fixture-zines", "synthetic", "Mutual aid fixture"), false)
	if err != nil {
		t.Fatal(err)
	}
	var snap string
	if err = db.Pool.QueryRow(ctx, "SELECT source_snapshot FROM variants WHERE id=$1", fixture.Variants[0].ID).Scan(&snap); err != nil {
		t.Fatal(err)
	}
	yes := true
	for i := 0; i < 30; i++ {
		collection := "fixture-zines"
		if i == 28 {
			collection = "fixture-second"
		}
		date := "1977-05-08"
		if i == 26 {
			date = "1977"
		}
		creator := "Fixture author"
		if i == 29 {
			creator = "Other creator"
		}
		key := fmt.Sprintf("browse-%02d", i)
		c := config.Collection{ID: collection, Item: "synthetic"}
		s := config.Selection{Key: key, File: key + ".pdf", Title: key, Creator: creator, Date: date}
		a := fixture.Variants[0]
		if err = db.Import(ctx, c, s, a.SHA, a.Size, snap); err != nil {
			t.Fatal(err)
		}
		if i != 27 {
			if err = db.Edit(ctx, catalog.ID(collection, "synthetic", key), nil, "", "Synthetic browse review", &yes); err != nil {
				t.Fatal(err)
			}
		}
	}
	f := catalog.BrowseFilters{Creator: "FIXTURE AUTHOR", DateFrom: "1977-01-01", DateTo: "1977-12-31"}
	docs, total, err := db.List(ctx, "fixture-zines", "browse", "", "", 20, 20, true, f)
	if err != nil || total != 26 || len(docs) != 6 {
		t.Fatalf("full catalog bounds: total=%d page=%d error=%v", total, len(docs), err)
	}
	_, total, err = db.List(ctx, "fixture-zines", "browse", "", "", 20, 20, true, catalog.BrowseFilters{Creator: "fixture author"})
	if err != nil || total != 27 {
		t.Fatalf("partial date retained without bounds: %d %v", total, err)
	}
	id := catalog.ID("fixture-zines", "synthetic", "browse-00")
	if err = db.Edit(ctx, id, map[string]any{"date": "2050-01-01"}, "", "Editorial date correction", nil); err != nil {
		t.Fatal(err)
	}
	doc, err := db.Get(ctx, id, true)
	if err != nil || doc.OriginalDate != "1977-05-08" || doc.Metadata["date"] != "2050-01-01" {
		t.Fatal("source date independence", doc, err)
	}
	_, total, err = db.List(ctx, "fixture-zines", "browse", "", "", 20, 0, true, f)
	if err != nil || total != 26 {
		t.Fatal("editorial date changed original-date filter", total, err)
	}
	handler := ArchiveHandler(db, store)
	for _, q := range []string{"date_from=2024-02-30", "date_from=2025-01-01&date_to=2024-01-01"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/documents?"+q, nil))
		if w.Code != 400 {
			t.Fatal("invalid date accepted", q, w.Code)
		}
	}
	db.PublicOnly = true
	_, total, err = db.List(ctx, "fixture-zines", "browse", "", "", 20, 0, true, f)
	if err != nil || total != 0 {
		t.Fatal("unapproved variants exposed by filters", total, err)
	}
}
