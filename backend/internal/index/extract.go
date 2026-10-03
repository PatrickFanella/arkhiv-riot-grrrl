// Package index produces rebuildable, resource-bounded PDF derivatives.
package index

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/catalog"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/storage"
)

type bounded struct {
	bytes.Buffer
	limit int
}

func (b *bounded) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, fmt.Errorf("tool output exceeded bound")
	}
	return b.Buffer.Write(p)
}
func command(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "prlimit", append([]string{"--as=536870912", "--cpu=30", "--fsize=33554432", "--", name}, args...)...)
	var output bounded
	output.limit = 16 << 20
	var stderr bounded
	stderr.limit = 64 << 10
	cmd.Stdout = &output
	cmd.Stderr = &stderr
	if e := cmd.Run(); e != nil {
		return nil, fmt.Errorf("%s failed or resource limit reached", name)
	}
	return output.Bytes(), nil
}
func Extract(ctx context.Context, db *catalog.DB, store *storage.Store, hash string) error {
	var size int64
	if e := db.Pool.QueryRow(ctx, `SELECT size FROM assets WHERE sha256=$1`, hash).Scan(&size); e != nil {
		return e
	}
	if e := store.Verify(hash, size); e != nil {
		return e
	}
	path, e := storage.Original(hash)
	if e != nil {
		return e
	}
	absolute := filepath.Join(store.Path, path)
	info, err := command(ctx, "pdfinfo", absolute)
	pages := 0
	status := "no-text"
	text := []byte{}
	if err == nil {
		m := regexp.MustCompile(`(?m)^Pages:\s+(\d+)`).FindSubmatch(info)
		if len(m) == 2 {
			pages, _ = strconv.Atoi(string(m[1]))
		}
		if pages < 1 || pages > 10000 {
			err = fmt.Errorf("invalid or excessive PDF page count")
		} else {
			text, err = command(ctx, "pdftotext", "-layout", "-enc", "UTF-8", absolute, "-")
		}
	}
	toolVersion, e := exec.CommandContext(ctx, "pdftotext", "-v").CombinedOutput()
	if e != nil {
		toolVersion = []byte("poppler unavailable")
	}
	tool := strings.TrimSpace(string(toolVersion))
	if len(tool) > 300 {
		tool = tool[:300]
	}
	tx, e := db.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `DELETE FROM page_text WHERE asset_sha=$1`, hash); e != nil {
		return e
	}
	detail := ""
	if err != nil {
		status = "failed"
		detail = err.Error()
	} else {
		sections := strings.Split(string(text), "\f")
		for n, s := range sections {
			if n >= pages {
				break
			}
			if strings.TrimSpace(s) != "" {
				status = "page-text"
				if _, e = tx.Exec(ctx, `INSERT INTO page_text(asset_sha,page,text) VALUES($1,$2,$3)`, hash, n+1, s); e != nil {
					return e
				}
			}
		}
	}
	if status == "no-text" {
		var companion string
		var size int64
		ce := tx.QueryRow(ctx, `SELECT c.companion_sha,a.size FROM asset_companions c JOIN assets a ON a.sha256=c.companion_sha WHERE c.asset_sha=$1 ORDER BY c.companion_sha LIMIT 1`, hash).Scan(&companion, &size)
		if ce == nil {
			if ce = store.Verify(companion, size); ce != nil {
				return ce
			}
			f, ce := store.Read(companion)
			if ce != nil {
				return ce
			}
			b, ce := io.ReadAll(io.LimitReader(f, 16<<20+1))
			f.Close()
			if ce != nil || len(b) > 16<<20 {
				return fmt.Errorf("OCR companion exceeds text bound")
			}
			if strings.TrimSpace(string(b)) != "" {
				status = "document-text"
				if _, e = tx.Exec(ctx, `INSERT INTO page_text(asset_sha,page,text) VALUES($1,0,$2)`, hash, string(b)); e != nil {
					return e
				}
			}
		}
	}
	if _, e = tx.Exec(ctx, `INSERT INTO derivatives(asset_sha,status,pages,tool,error) VALUES($1,$2,$3,$4,$5) ON CONFLICT(asset_sha) DO UPDATE SET status=excluded.status,pages=excluded.pages,tool=excluded.tool,error=excluded.error,updated_at=now()`, hash, status, pages, tool, detail); e != nil {
		return e
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	if err != nil {
		return err
	}
	dir := "derivatives/" + hash
	if e = store.Root.MkdirAll(dir, 0700); e != nil {
		return e
	}
	// Cover failure does not invalidate the original or searchable text.
	_, coverErr := command(ctx, "pdftoppm", "-f", "1", "-singlefile", "-scale-to", "700", "-png", absolute, filepath.Join(store.Path, dir, "cover"))
	return store.WriteJSON(dir+"/manifest.json", map[string]any{"input_sha256": hash, "pages": pages, "tool": tool, "status": status, "cover_generated": coverErr == nil})
}
func All(ctx context.Context, db *catalog.DB, store *storage.Store) error {
	rows, e := db.Pool.Query(ctx, `SELECT DISTINCT asset_sha FROM variants ORDER BY asset_sha`)
	if e != nil {
		return e
	}
	hashes := []string{}
	for rows.Next() {
		var s string
		if e = rows.Scan(&s); e != nil {
			rows.Close()
			return e
		}
		hashes = append(hashes, s)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	failed := 0
	for _, h := range hashes {
		if e = Extract(ctx, db, store, h); e != nil {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d assets failed extraction; original files retained", failed)
	}
	return nil
}
func Cover(store *storage.Store, hash string) (*os.File, error) {
	if _, e := storage.Original(hash); e != nil {
		return nil, e
	}
	return store.Root.Open("derivatives/" + hash + "/cover.png")
}
