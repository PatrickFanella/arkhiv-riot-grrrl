package internetarchive

import (
	"context"
	"encoding/json"
	"fmt"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/config"
	"io"
	"net/url"
	"strconv"
	"strings"
)

type Discovery struct {
	Items []string          `json:"items"`
	Pages []json.RawMessage `json:"pages"`
}

// Discover rejects truncated, unstable or over-budget pagination rather than
// treating a partial result as a complete removal baseline.
func (c *Client) Discover(ctx context.Context, query string, maxItems, pageSize int) (Discovery, error) {
	out := Discovery{Items: []string{}, Pages: []json.RawMessage{}}
	if strings.TrimSpace(query) == "" || len(query) > 2000 || maxItems < 1 || maxItems > 200 || pageSize < 1 || pageSize > 50 {
		return out, fmt.Errorf("invalid discovery limits")
	}
	expected := -1
	previous := ""
	for page := 1; page <= maxItems/pageSize+2; page++ {
		params := url.Values{"q": {query}, "output": {"json"}, "fl[]": {"identifier"}, "sort[]": {"identifier asc"}, "rows": {strconv.Itoa(pageSize)}, "page": {strconv.Itoa(page)}}
		response, err := c.Request(ctx, "/advancedsearch.php?"+params.Encode(), nil)
		if err != nil {
			return out, err
		}
		if response.StatusCode != 200 {
			response.Body.Close()
			return out, fmt.Errorf("discovery status %d", response.StatusCode)
		}
		raw, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
		response.Body.Close()
		if err != nil {
			return out, err
		}
		if len(raw) > 1<<20 {
			return out, fmt.Errorf("discovery response exceeds bound")
		}
		var decoded struct {
			Response *struct {
				Found int `json:"numFound"`
				Start int `json:"start"`
				Docs  []struct {
					ID string `json:"identifier"`
				} `json:"docs"`
			} `json:"response"`
		}
		if err = json.Unmarshal(raw, &decoded); err != nil {
			return out, err
		}
		r := decoded.Response
		if r == nil || r.Found < 0 || r.Found > maxItems || r.Start != (page-1)*pageSize {
			return out, fmt.Errorf("incomplete or excessive discovery")
		}
		if expected < 0 {
			expected = r.Found
		}
		if r.Found != expected {
			return out, fmt.Errorf("discovery count changed during pagination")
		}
		if len(r.Docs) > pageSize || len(out.Items)+len(r.Docs) > expected || (len(r.Docs) == 0 && len(out.Items) < expected) {
			return out, fmt.Errorf("incomplete discovery page")
		}
		for _, doc := range r.Docs {
			if !config.ValidID(doc.ID) || doc.ID <= previous {
				return out, fmt.Errorf("duplicate or unstable discovery identifiers")
			}
			out.Items = append(out.Items, doc.ID)
			previous = doc.ID
		}
		out.Pages = append(out.Pages, append([]byte(nil), raw...))
		if len(out.Items) == expected {
			return out, nil
		}
	}
	return out, fmt.Errorf("discovery page bound exhausted")
}

func (c *Client) LimitRequests(limit int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.limited = true
	c.remaining = limit
}
