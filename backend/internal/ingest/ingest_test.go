package ingest

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/catalog"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/config"
	ia "git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/source/internetarchive"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/storage"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDownloadResumeReuseAndSourceChange(t *testing.T) {
	data := "original payload"
	sum := md5.Sum([]byte(data))
	f := ia.File{Name: "zine.pdf", Size: fmt.Sprint(len(data)), MD5: hex.EncodeToString(sum[:])}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("ETag", `"stable"`)
		if calls == 1 {
			w.Header().Set("Content-Length", f.Size)
			fmt.Fprint(w, data[:5])
			return
		}
		if r.Header.Get("Range") != "bytes=5-" || r.Header.Get("If-Range") != `"stable"` {
			t.Error("missing safe resume headers")
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes 5-%d/%d", len(data)-1, len(data)))
		w.WriteHeader(206)
		fmt.Fprint(w, data[5:])
	}))
	defer server.Close()
	s, _ := storage.Open(t.TempDir())
	defer s.Close()
	c := &ia.Client{HTTP: server.Client(), Base: server.URL}
	if _, _, _, e := Download(context.Background(), c, s, "item", f, 100); e == nil {
		t.Fatal("accepted interrupted file")
	}
	hash, n, reused, e := Download(context.Background(), c, s, "item", f, 100)
	if e != nil || reused || n != int64(len(data)) {
		t.Fatalf("resume: %v", e)
	}
	h, _, reused, e := Download(context.Background(), c, s, "item", f, 100)
	if e != nil || !reused || h != hash || calls != 2 {
		t.Fatal("failed verified reuse")
	}
}
func TestDownloadRejectsBadChecksumAndOversize(t *testing.T) {
	for _, tc := range []struct {
		name, body, md5 string
		limit           int64
	}{{"checksum", "abcd", strings.Repeat("0", 32), 100}, {"oversize", "abcdef", "", 4}} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.body) }))
			defer server.Close()
			s, _ := storage.Open(t.TempDir())
			defer s.Close()
			c := &ia.Client{HTTP: server.Client(), Base: server.URL}
			if _, _, _, e := Download(context.Background(), c, s, "item", ia.File{Name: "../outside.pdf", Size: "4", MD5: tc.md5}, tc.limit); e == nil {
				t.Fatal("accepted invalid download")
			}
			entries, _ := os.ReadDir(s.Path + "/originals")
			if len(entries) > 0 {
				t.Fatal("promoted bad bytes")
			}
		})
	}
}

func TestResumeRestartsOrRejectsUnsafeRanges(t *testing.T) {
	for _, mode := range []string{"ignored", "oversized-part", "changed-etag", "wrong-range"} {
		t.Run(mode, func(t *testing.T) {
			payload := "abcdefghij"
			m := md5.Sum([]byte(payload))
			f := ia.File{Name: "zine.pdf", Size: "10", MD5: hex.EncodeToString(m[:])}
			s, _ := storage.Open(t.TempDir())
			defer s.Close()
			base := "jobs/" + catalog.ID("item", f.Name)
			s.Root.MkdirAll("jobs", 0700)
			os.WriteFile(s.Path+"/"+base+".part", []byte("abc"), 0600)
			if mode == "oversized-part" {
				os.WriteFile(s.Path+"/"+base+".part", []byte("oversized partial data"), 0600)
			}
			s.WriteJSON(base+".json", Checkpoint{Version: catalog.ID(f.Name, f.Size, f.MD5, f.SHA1), ETag: `"old"`})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == "oversized-part" {
					if r.Header.Get("Range") != "" {
						t.Error("resumed invalid partial")
					}
					fmt.Fprint(w, payload)
					return
				}
				if r.Header.Get("Range") != "bytes=3-" {
					t.Error("did not attempt bounded resume")
				}
				if mode == "ignored" {
					fmt.Fprint(w, payload)
					return
				}
				w.Header().Set("ETag", `"old"`)
				w.Header().Set("Content-Range", "bytes 3-9/10")
				if mode == "changed-etag" {
					w.Header().Set("ETag", `"new"`)
				}
				if mode == "wrong-range" {
					w.Header().Set("Content-Range", "bytes 4-9/10")
				}
				w.WriteHeader(206)
				fmt.Fprint(w, payload[3:])
			}))
			defer server.Close()
			c := &ia.Client{HTTP: server.Client(), Base: server.URL}
			hash, n, _, e := Download(context.Background(), c, s, "item", f, 20)
			if mode == "ignored" || mode == "oversized-part" {
				if e != nil || n != 10 || s.Verify(hash, n) != nil {
					t.Fatal("failed safe restart", e)
				}
			} else if e == nil {
				t.Fatal("accepted unsafe partial response")
			}
		})
	}
}

func TestInventoryShowsSelectedCompanionsAndReviewSuggestions(t *testing.T) {
	c := config.Collection{Item: "item", Selection: []config.Selection{{File: "book-read.pdf", Key: "book", Title: "Book"}}, Limits: config.Limits{Documents: 1, TotalBytes: 20, FileBytes: 10, Concurrency: 1}}
	m := ia.Metadata{Files: []ia.File{{Name: "book-read.pdf", Size: "10"}, {Name: "book-print.pdf", Size: "10"}, {Name: "book-read_djvu.txt", Size: "5"}}}
	p, e := Inventory(c, m)
	if e != nil || p.Bytes != 15 || p.Documents != 1 {
		t.Fatal("inventory total", e, p)
	}
	if !p.Candidates[2].Selected || p.Candidates[2].CompanionFor != "book-read.pdf" {
		t.Fatal("companion selection is hidden")
	}
	if len(p.GroupingSuggestions) != 1 || !p.GroupingSuggestions[0].RequiresReview {
		t.Fatal("grouping suggestion did not require review")
	}
	c.Limits.TotalBytes = 12
	if _, e = Inventory(c, m); e == nil {
		t.Fatal("companion exceeded batch cap")
	}
}
