package catalog

import (
	"strings"
	"testing"
)

func TestBrowseFiltersCountCreatorCharacters(t *testing.T) {
	creator := strings.Repeat("ж", 200)
	if err := (BrowseFilters{Creator: creator}).Validate(); err != nil {
		t.Fatalf("200-character creator rejected: %v", err)
	}
	if err := (BrowseFilters{Creator: creator + "ж"}).Validate(); err == nil {
		t.Fatal("201-character creator accepted")
	}
}
