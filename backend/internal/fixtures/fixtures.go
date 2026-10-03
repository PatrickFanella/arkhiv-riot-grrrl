// Package fixtures creates clearly identified synthetic documents for isolated verification.
package fixtures

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/catalog"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/config"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/index"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/storage"
)

func PDF(pages ...string) []byte {
	objects := []string{"", "<< /Type /Catalog /Pages 2 0 R >>", ""}
	kids := []string{}
	for i, s := range pages {
		p := 3 + i*2
		kids = append(kids, fmt.Sprintf("%d 0 R", p))
		text := strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)").Replace(s)
		stream := fmt.Sprintf("BT /F1 18 Tf 50 750 Td (%s) Tj ET", text)
		objects = append(objects, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >> /Contents %d 0 R >>", p+1), fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	}
	objects[2] = fmt.Sprintf("<< /Type /Pages /Count %d /Kids [%s] >>", len(pages), strings.Join(kids, " "))
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i := 1; i < len(objects); i++ {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i, objects[i])
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects))
	for _, o := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects), xref)
	return b.Bytes()
}
func Seed(ctx context.Context, db *catalog.DB, s *storage.Store) error {
	if os.Getenv("ARKHIV_FIXTURE") != "1" {
		return fmt.Errorf("fixture seed requires ARKHIV_FIXTURE=1")
	}
	for i, id := range []string{"fixture-zines", "fixture-second"} {
		c := config.Collection{ID: id, Item: "synthetic", Name: []string{"Synthetic verification collection", "Second synthetic archive"}[i], Introduction: []string{"Synthetic documents for local verification; these are not archive acquisitions.", "A different editorial introduction for the second synthetic collection."}[i], Accent: []string{"#8c2921", "#245c42"}[i], Limits: config.Limits{Documents: 50, TotalBytes: 2 << 30, FileBytes: 250 << 20, Concurrency: 2}}
		if e := db.Collection(ctx, c); e != nil {
			return e
		}
		snap, e := db.Snapshot(ctx, "synthetic", []byte(`{"synthetic":true}`))
		if e != nil {
			return e
		}
		for j, title := range []string{"Mutual aid fixture", "Private fixture", "No-text fixture"} {
			pdf := PDF(title, "Solidarity knownword appears on page two.")
			if j == 2 {
				pdf = PDF("")
			}
			hash, n, e := storage.Digest(bytes.NewReader(pdf))
			if e != nil {
				return e
			}
			part := fmt.Sprintf("fixture-%s-%d.part", id, j)
			if e = os.WriteFile(s.Path+"/"+part, pdf, 0600); e != nil {
				return e
			}
			if e = s.Promote(part, hash, n); e != nil {
				return e
			}
			selection := config.Selection{Key: title, File: title + ".pdf", Title: title, Creator: "Fixture author", Date: "unknown", Language: "en", Subjects: []string{"mutual aid"}, Rights: "synthetic test fixture"}
			if e = db.Import(ctx, c, selection, hash, n, snap); e != nil {
				return e
			}
			if e = index.Extract(ctx, db, s, hash); e != nil {
				return e
			}
			if j == 0 {
				yes := true
				if e = db.Edit(ctx, catalog.ID(c.ID, c.Item, title), nil, "", "Synthetic publication review", &yes); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
