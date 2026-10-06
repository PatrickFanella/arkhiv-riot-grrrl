package catalog

import (
	"fmt"
	"time"
)

type BrowseFilters struct{ Creator, DateFrom, DateTo string }

func (f BrowseFilters) Validate() error {
	if len(f.Creator) > 200 {
		return fmt.Errorf("creator exceeds 200 characters")
	}
	for _, value := range []string{f.DateFrom, f.DateTo} {
		if value != "" {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				return fmt.Errorf("use complete YYYY-MM-DD date bounds")
			}
		}
	}
	if f.DateFrom != "" && f.DateTo != "" && f.DateFrom > f.DateTo {
		return fmt.Errorf("the original-date range is reversed")
	}
	return nil
}
