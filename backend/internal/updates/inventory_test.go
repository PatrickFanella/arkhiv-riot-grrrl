package updates

import (
	ia "git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/source/internetarchive"
	"testing"
)

func TestCompleteAndDiff(t *testing.T) {
	before := Inventory{Metadata: map[string]any{"title": "Original"}, Files: map[string]ia.File{
		"changed.pdf":  {Name: "changed.pdf", Size: "10", MD5: "old"},
		"gone.pdf":     {Name: "gone.pdf", Size: "12", MD5: "gone"},
		"metadata.pdf": {Name: "metadata.pdf", Size: "15", MD5: "same", Private: "false"},
		"unknown.pdf":  {Name: "unknown.pdf", Size: "20"},
		"same.pdf":     {Name: "same.pdf", Size: "30", MD5: "same"},
	}}
	after := Inventory{Metadata: map[string]any{"title": "Updated"}, Files: map[string]ia.File{
		"changed.pdf":  {Name: "changed.pdf", Size: "10", MD5: "new"},
		"new.pdf":      {Name: "new.pdf", Size: "12", MD5: "new"},
		"metadata.pdf": {Name: "metadata.pdf", Size: "15", MD5: "same", Private: "true"},
		"unknown.pdf":  {Name: "unknown.pdf", Size: "20"},
		"same.pdf":     {Name: "same.pdf", Size: "30", MD5: "same"},
	}}
	got := map[string]string{}
	for _, c := range Diff(before, after) {
		got[c.File] = c.Kind
	}
	want := map[string]string{"changed.pdf": "content", "gone.pdf": "removed", "new.pdf": "added", "metadata.pdf": "metadata", "unknown.pdf": "uncertain", "": "metadata"}
	if !equal(got, want) {
		t.Fatal(got)
	}
	for _, m := range []ia.Metadata{
		{}, {Metadata: map[string]any{}, Files: []ia.File{{Name: "a"}, {Name: "a"}}},
		{Metadata: map[string]any{}, Files: []ia.File{{Name: "../unsafe"}}},
	} {
		if _, err := Complete(m); err == nil {
			t.Fatal("accepted incomplete or unsafe inventory")
		}
	}
}
