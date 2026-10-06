package app

import (
	"encoding/json"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/catalog"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/index"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/storage"
)

func ArchiveHandler(db *catalog.DB, store *storage.Store) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/health", Handler())
	mux.HandleFunc("GET /api/ready", func(w http.ResponseWriter, r *http.Request) {
		if e := db.Pool.Ping(r.Context()); e != nil {
			fail(w, 503, "catalog unavailable")
			return
		}
		respond(w, map[string]any{"status": "ok", "stage": "archive", "public": db.PublicOnly, "revision": os.Getenv("ARKHIV_REVISION")})
	})
	mux.HandleFunc("GET /api/collections", func(w http.ResponseWriter, r *http.Request) {
		c, e := db.Collections(r.Context())
		if e != nil {
			fail(w, 503, "catalog unavailable")
			return
		}
		respond(w, c)
	})
	mux.HandleFunc("GET /api/documents", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limit, offset := 20, 0
		var e error
		if q.Get("limit") != "" {
			limit, e = strconv.Atoi(q.Get("limit"))
			if e != nil {
				fail(w, 400, "invalid limit")
				return
			}
		}
		if q.Get("offset") != "" {
			offset, e = strconv.Atoi(q.Get("offset"))
			if e != nil {
				fail(w, 400, "invalid offset")
				return
			}
		}
		if limit < 1 || limit > 50 || offset < 0 || offset > 100000 || len(q.Get("q")) > 200 || len(q.Get("subject")) > 100 || len(q.Get("language")) > 50 {
			fail(w, 400, "query exceeds bounds")
			return
		}
		filters := catalog.BrowseFilters{Creator: q.Get("creator"), DateFrom: q.Get("date_from"), DateTo: q.Get("date_to")}
		if err := filters.Validate(); err != nil {
			fail(w, 400, err.Error())
			return
		}
		docs, total, e := db.List(r.Context(), q.Get("collection"), q.Get("q"), q.Get("subject"), q.Get("language"), limit, offset, true, filters)
		if e != nil {
			fail(w, 503, "catalog unavailable")
			return
		}
		respond(w, map[string]any{"documents": docs, "total": total, "limit": limit, "offset": offset})
	})
	mux.HandleFunc("GET /api/documents/{id}", func(w http.ResponseWriter, r *http.Request) {
		d, e := db.Get(r.Context(), r.PathValue("id"), true)
		if e != nil {
			fail(w, 404, "document unavailable")
			return
		}
		respond(w, d)
	})
	mux.HandleFunc("GET /api/facets", func(w http.ResponseWriter, r *http.Request) {
		rows, e := db.Pool.Query(r.Context(), `SELECT DISTINCT jsonb_array_elements_text(CASE WHEN jsonb_typeof((metadata||overrides)->'subjects')='array' THEN (metadata||overrides)->'subjects' ELSE '[]'::jsonb END) FROM documents WHERE published AND collection_id=$1 AND (NOT $2 OR EXISTS(SELECT 1 FROM variants WHERE document_id=documents.id AND public_approved)) ORDER BY 1`, r.URL.Query().Get("collection"), db.PublicOnly)
		if e != nil {
			fail(w, 503, "catalog unavailable")
			return
		}
		defer rows.Close()
		subjects := []string{}
		for rows.Next() {
			var s string
			if e = rows.Scan(&s); e != nil {
				fail(w, 503, "catalog unavailable")
				return
			}
			subjects = append(subjects, s)
		}
		if rows.Err() != nil {
			fail(w, 503, "catalog unavailable")
			return
		}
		respond(w, map[string]any{"subjects": subjects})
	})
	assets := func(w http.ResponseWriter, r *http.Request) {
		var hash, name string
		var size int64
		e := db.Pool.QueryRow(r.Context(), `SELECT v.asset_sha,v.filename,a.size FROM variants v JOIN documents d ON d.id=v.document_id JOIN assets a ON a.sha256=v.asset_sha WHERE v.id=$1 AND d.published AND (NOT $2 OR v.public_approved)`, r.PathValue("id"), db.PublicOnly).Scan(&hash, &name, &size)
		if e != nil {
			fail(w, 404, "asset unavailable")
			return
		}
		cover := strings.HasSuffix(r.URL.Path, "/cover")
		f, e := store.Read(hash)
		if cover {
			if f != nil {
				f.Close()
			}
			f, e = index.Cover(store, hash)
		}
		if e != nil {
			fail(w, 404, "asset unavailable")
			return
		}
		defer f.Close()
		info, e := f.Stat()
		if e != nil || !info.Mode().IsRegular() || (!cover && info.Size() != size) {
			fail(w, 404, "asset unavailable")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if cover {
			w.Header().Set("Content-Type", "image/png")
		} else {
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("ETag", `"`+hash+`"`)
			disposition := "inline"
			if r.URL.Query().Get("download") == "1" {
				disposition = "attachment"
			}
			w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": filepath.Base(name)}))
		}
		http.ServeContent(w, r, name, info.ModTime(), f)
	}
	mux.HandleFunc("GET /api/assets/{id}/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if len(q) > 200 || strings.TrimSpace(q) == "" {
			fail(w, 400, "search query required within bounds")
			return
		}
		var allowed bool
		if e := db.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM variants v JOIN documents d ON d.id=v.document_id WHERE v.id=$1 AND d.published AND (NOT $2 OR v.public_approved))`, r.PathValue("id"), db.PublicOnly).Scan(&allowed); e != nil || !allowed {
			fail(w, 404, "asset unavailable")
			return
		}
		rows, e := db.Pool.Query(r.Context(), `SELECT t.page,left(t.text,350) FROM variants v JOIN documents d ON d.id=v.document_id JOIN page_text t ON t.asset_sha=v.asset_sha WHERE v.id=$1 AND d.published AND (NOT $3 OR v.public_approved) AND t.search @@ plainto_tsquery('simple',$2) ORDER BY t.page LIMIT 50`, r.PathValue("id"), q, db.PublicOnly)
		if e != nil {
			fail(w, 503, "search unavailable")
			return
		}
		defer rows.Close()
		hits := []catalog.Hit{}
		for rows.Next() {
			h := catalog.Hit{Variant: r.PathValue("id")}
			if e = rows.Scan(&h.Page, &h.Snippet); e != nil {
				fail(w, 503, "search unavailable")
				return
			}
			hits = append(hits, h)
		}
		if rows.Err() != nil {
			fail(w, 503, "search unavailable")
			return
		}
		respond(w, hits)
	})
	mux.HandleFunc("GET /api/assets/{id}", assets)
	mux.HandleFunc("GET /api/assets/{id}/cover", assets)
	return mux
}
func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if e := json.NewEncoder(w).Encode(v); e != nil {
		return
	}
}
func fail(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
