package updates

import (
	"context"
	"encoding/json"
	"fmt"

	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/catalog"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/config"
	ia "git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/source/internetarchive"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/storage"
)

type sourceRecord struct {
	Item      string
	Snapshot  string
	Raw       []byte
	Inventory Inventory
}

func gather(ctx context.Context, c config.Collection, client *ia.Client, store *storage.Store) ([]sourceRecord, string, []byte, error) {
	scope := "item:" + c.Item
	items := []string{c.Item}
	proof := []byte(`{}`)
	if c.Discovery.Query != "" {
		scope = "query:" + c.Discovery.Query
		client.LimitRequests(c.Discovery.MaxRequests)
		discovered, err := client.Discover(ctx, c.Discovery.Query, c.Discovery.MaxItems, c.Discovery.PageSize)
		if err != nil {
			return nil, scope, nil, err
		}
		items = discovered.Items
		proof, err = json.Marshal(discovered)
		if err != nil {
			return nil, scope, nil, err
		}
	}
	records := []sourceRecord{}
	total := len(proof)
	for _, item := range items {
		m, raw, err := client.Metadata(ctx, item)
		if err != nil {
			return nil, scope, nil, err
		}
		inventory, err := Complete(m)
		if err != nil {
			return nil, scope, nil, err
		}
		total += len(raw)
		if total > 32<<20 {
			return nil, scope, nil, fmt.Errorf("aggregate source metadata exceeds bound")
		}
		snapshot := catalog.ID(item, string(raw))
		if err = store.WriteBytes("snapshots/"+snapshot+".json", raw); err != nil {
			return nil, scope, nil, err
		}
		records = append(records, sourceRecord{item, snapshot, raw, inventory})
	}
	return records, scope, proof, nil
}
