// Package ingest acquires explicitly selected documents with durable checkpoints.
package ingest

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/catalog"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/config"
	ia "git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/source/internetarchive"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/storage"
)

type Candidate struct {
	ia.File
	Selected     bool   `json:"selected"`
	Reason       string `json:"reason"`
	CompanionFor string `json:"companion_for,omitempty"`
}
type Plan struct {
	Item                string               `json:"item"`
	Candidates          []Candidate          `json:"candidates"`
	Bytes               int64                `json:"selected_bytes"`
	Documents           int                  `json:"documents"`
	GroupingSuggestions []GroupingSuggestion `json:"grouping_suggestions"`
}

type GroupingSuggestion struct {
	Stem           string   `json:"stem"`
	Files          []string `json:"files"`
	RequiresReview bool     `json:"requires_review"`
}

func Inventory(c config.Collection, m ia.Metadata) (Plan, error) {
	p := Plan{Item: c.Item, Candidates: []Candidate{}}
	files := map[string]ia.File{}
	selected := map[string]bool{}
	keys := map[string]bool{}
	for _, s := range c.Selection {
		selected[s.File] = true
		keys[s.Key] = true
	}
	p.Documents = len(keys)
	companionFor := map[string]string{}
	for _, f := range m.Files {
		files[f.Name] = f
	}
	for _, sel := range c.Selection {
		name := strings.TrimSuffix(sel.File, ".pdf") + "_djvu.txt"
		if _, ok := files[name]; ok {
			selected[name] = true
			companionFor[name] = sel.File
		}
	}
	groups := map[string][]string{}
	variantSuffix := regexp.MustCompile(`(?i)([-_](print|read|screen|imposed|text|printing|reading))+$`)
	for _, f := range m.Files {
		if strings.HasSuffix(strings.ToLower(f.Name), ".pdf") {
			stem := strings.ToLower(variantSuffix.ReplaceAllString(strings.TrimSuffix(f.Name, ".pdf"), ""))
			groups[stem] = append(groups[stem], f.Name)
		}
	}
	p.GroupingSuggestions = []GroupingSuggestion{}
	stems := []string{}
	for stem, names := range groups {
		if len(names) > 1 {
			stems = append(stems, stem)
		}
	}
	sort.Strings(stems)
	for _, stem := range stems {
		sort.Strings(groups[stem])
		p.GroupingSuggestions = append(p.GroupingSuggestions, GroupingSuggestion{stem, groups[stem], true})
	}
	for _, f := range m.Files {
		files[f.Name] = f
		reason := "not selected"
		if !strings.HasSuffix(strings.ToLower(f.Name), ".pdf") {
			reason = "OCR companion or non-PDF derivative"
		}
		if f.Bytes() <= 0 {
			reason = "unknown size"
		}
		if f.Private == "true" {
			reason = "restricted file"
		}
		if selected[f.Name] {
			reason = "selected"
			p.Bytes += f.Bytes()
		}
		if companionFor[f.Name] != "" {
			reason = "selected OCR companion"
		}
		p.Candidates = append(p.Candidates, Candidate{File: f, Selected: selected[f.Name], Reason: reason, CompanionFor: companionFor[f.Name]})
	}
	for _, s := range c.Selection {
		f, ok := files[s.File]
		if !ok || f.Bytes() <= 0 || f.Bytes() > c.Limits.FileBytes || f.Private == "true" || !strings.HasSuffix(strings.ToLower(f.Name), ".pdf") {
			return p, fmt.Errorf("selected file missing, restricted, not PDF or exceeds limit: %s", s.File)
		}
	}
	// Preserve matching IA OCR sidecars as separate verified assets.
	companions := map[string]bool{}
	for _, sel := range c.Selection {
		name := strings.TrimSuffix(sel.File, ".pdf") + "_djvu.txt"
		if f, ok := files[name]; ok {
			if f.Bytes() <= 0 || f.Bytes() > c.Limits.FileBytes || f.Private == "true" {
				return p, fmt.Errorf("invalid OCR companion: %s", name)
			}
			if !companions[name] {
				companions[name] = true
			}
		}
	}
	if p.Bytes > c.Limits.TotalBytes {
		return p, fmt.Errorf("selected bytes exceed batch limit")
	}
	return p, nil
}

type Checkpoint struct {
	Version string `json:"version"`
	ETag    string `json:"etag"`
	SHA     string `json:"sha256,omitempty"`
	Size    int64  `json:"size"`
}

func Download(ctx context.Context, client *ia.Client, store *storage.Store, item string, f ia.File, limit int64) (string, int64, bool, error) {
	key := catalog.ID(item, f.Name)
	base := "jobs/" + key
	part := base + ".part"
	version := catalog.ID(f.Name, f.Size, f.MD5, f.SHA1)
	var cp Checkpoint
	_ = store.JSON(base+".json", &cp)
	if cp.Version == version && cp.SHA != "" && (f.MD5 != "" || f.SHA1 != "") {
		if e := store.Verify(cp.SHA, cp.Size); e == nil {
			return cp.SHA, cp.Size, true, nil
		}
	}
	if e := store.Root.MkdirAll("jobs", 0700); e != nil {
		return "", 0, false, e
	}
	offset := int64(0)
	if cp.Version == version && cp.ETag != "" && !strings.HasPrefix(cp.ETag, "W/") {
		if info, e := store.Root.Stat(part); e == nil {
			if info.Size() > 0 && info.Size() < f.Bytes() && info.Size() <= limit {
				offset = info.Size()
			}
		}
	}
	headers := map[string]string{}
	if offset > 0 {
		headers["Range"] = fmt.Sprintf("bytes=%d-", offset)
		headers["If-Range"] = cp.ETag
	}
	resp, e := client.Request(ctx, ia.FilePath(item, f.Name), headers)
	if e != nil {
		return "", 0, false, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		return "", 0, false, fmt.Errorf("download status %d", resp.StatusCode)
	}
	if resp.StatusCode == 200 {
		offset = 0
	} else {
		cr := resp.Header.Get("Content-Range")
		var start, end, total int64
		fields, parseErr := fmt.Sscanf(cr, "bytes %d-%d/%d", &start, &end, &total)
		if offset == 0 || parseErr != nil || fields != 3 || start != offset || end < start || end >= total || total != f.Bytes() || (resp.ContentLength >= 0 && resp.ContentLength != end-start+1) || resp.Header.Get("ETag") != cp.ETag {
			return "", 0, false, fmt.Errorf("unsafe partial response")
		}
	}
	if offset > limit || resp.ContentLength > limit-offset {
		return "", 0, false, fmt.Errorf("download exceeds file bound")
	}
	flags := os.O_CREATE | os.O_WRONLY
	if offset > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	out, e := store.Root.OpenFile(part, flags, 0600)
	if e != nil {
		return "", 0, false, e
	}
	cp = Checkpoint{Version: version, ETag: resp.Header.Get("ETag")}
	if e = store.WriteJSON(base+".json", cp); e != nil {
		out.Close()
		return "", 0, false, e
	}
	n, e := io.Copy(out, io.LimitReader(resp.Body, limit-offset+1))
	syncErr := out.Sync()
	closeErr := out.Close()
	if e != nil {
		return "", 0, false, e
	}
	if syncErr != nil {
		return "", 0, false, syncErr
	}
	if closeErr != nil {
		return "", 0, false, closeErr
	}
	if n+offset > limit || n+offset != f.Bytes() {
		return "", 0, false, fmt.Errorf("incomplete or oversized file")
	}
	input, e := store.Root.Open(part)
	if e != nil {
		return "", 0, false, e
	}
	defer input.Close()
	h := sha256.New()
	m := md5.New()
	s := sha1.New()
	n, e = io.Copy(io.MultiWriter(h, m, s), input)
	if e != nil {
		return "", 0, false, e
	}
	if f.MD5 != "" && !strings.EqualFold(f.MD5, hex.EncodeToString(m.Sum(nil))) || f.SHA1 != "" && !strings.EqualFold(f.SHA1, hex.EncodeToString(s.Sum(nil))) {
		store.Root.Remove(part)
		return "", 0, false, fmt.Errorf("source checksum mismatch")
	}
	hash := hex.EncodeToString(h.Sum(nil))
	if e = store.Promote(part, hash, n); e != nil {
		return "", 0, false, e
	}
	cp.SHA = hash
	cp.Size = n
	if e = store.WriteJSON(base+".json", cp); e != nil {
		return "", 0, false, e
	}
	return hash, n, false, nil
}

type Result struct {
	Item   string `json:"item,omitempty"`
	File   string `json:"file"`
	SHA    string `json:"sha256,omitempty"`
	Size   int64  `json:"size"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}
type Receipt struct {
	Collection string    `json:"collection"`
	Snapshot   string    `json:"snapshot"`
	Snapshots  []string  `json:"snapshots,omitempty"`
	Started    time.Time `json:"started"`
	Files      []Result  `json:"files"`
}

func acquireItem(ctx context.Context, c config.Collection, client *ia.Client, store *storage.Store, db *catalog.DB) (Receipt, error) {
	receipt := Receipt{Collection: c.ID, Started: time.Now().UTC(), Files: []Result{}}
	if len(c.Selection) == 0 {
		return receipt, fmt.Errorf("reviewed selection required")
	}
	if e := db.Collection(ctx, c); e != nil {
		return receipt, e
	}
	m, raw, e := client.Metadata(ctx, c.Item)
	if e != nil {
		return receipt, e
	}
	if _, e = Inventory(c, m); e != nil {
		return receipt, e
	}
	snap, e := db.Snapshot(ctx, c.Item, raw)
	if e != nil {
		return receipt, e
	}
	receipt.Snapshot = snap
	if e = store.WriteBytes("snapshots/"+snap+".json", raw); e != nil {
		return receipt, e
	}
	by := map[string]ia.File{}
	for _, f := range m.Files {
		by[f.Name] = f
	}
	failed := false
	for _, sel := range c.Selection {
		if e = db.Attempt(ctx, c.ID, c.Item, sel.File, "started", snap); e != nil {
			return receipt, e
		}
		hash, n, reused, err := Download(ctx, client, store, c.Item, by[sel.File], c.Limits.FileBytes)
		result := Result{Item: c.Item, File: sel.File, SHA: hash, Size: n, Status: "verified"}
		if reused {
			result.Status = "reused"
		}
		if err == nil {
			err = db.Import(ctx, c, sel, hash, n, snap)
		}
		if err == nil {
			companionName := strings.TrimSuffix(sel.File, ".pdf") + "_djvu.txt"
			if companion, ok := by[companionName]; ok {
				ch, cn, cr, ce := Download(ctx, client, store, c.Item, companion, c.Limits.FileBytes)
				status := "verified"
				if cr {
					status = "reused"
				}
				if ce == nil {
					_, ce = db.Pool.Exec(ctx, `INSERT INTO assets(sha256,size) VALUES($1,$2) ON CONFLICT DO NOTHING`, ch, cn)
				}
				if ce == nil {
					_, ce = db.Pool.Exec(ctx, `INSERT INTO asset_companions(asset_sha,companion_sha,filename,source_snapshot) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, hash, ch, companionName, snap)
				}
				message := ""
				if ce != nil {
					status = "failed"
					message = ce.Error()
					err = ce
				}
				receipt.Files = append(receipt.Files, Result{Item: c.Item, File: companionName, SHA: ch, Size: cn, Status: status, Error: message})
			}
		}
		if err != nil {
			failed = true
			result.Status = "failed"
			result.Error = err.Error()
		}
		if e = db.Attempt(ctx, c.ID, c.Item, sel.File, result.Status, result.Error); e != nil {
			return receipt, e
		}
		receipt.Files = append(receipt.Files, result)
		if e = store.WriteJSON("receipts/"+catalog.ID(c.ID, receipt.Started.String())+".json", receipt); e != nil {
			return receipt, e
		}
		var backoff *ia.RetryError
		if errors.As(err, &backoff) {
			return receipt, err
		}

	}
	if failed {
		return receipt, fmt.Errorf("acquisition has failures; see receipt")
	}
	return receipt, nil
}
