package ingest

import (
	"context"
	"fmt"
	"sort"
	"syscall"
	"time"

	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/catalog"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/config"
	ia "git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/source/internetarchive"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/storage"
)

type CollectionPlan struct {
	Collection string `json:"collection"`
	Plans      []Plan `json:"items"`
	Bytes      int64  `json:"selected_bytes"`
	Documents  int    `json:"documents"`
}

func split(c config.Collection) ([]config.Collection, error) {
	if len(c.Selection) == 0 {
		return nil, fmt.Errorf("reviewed selection required")
	}
	groups := map[string][]config.Selection{}
	for _, selected := range c.Selection {
		item := selected.Item
		if item == "" {
			item = c.Item
		}
		if !config.ValidID(item) {
			return nil, fmt.Errorf("selection source item required")
		}
		groups[item] = append(groups[item], selected)
	}
	items := []string{}
	for item := range groups {
		items = append(items, item)
	}
	sort.Strings(items)
	out := []config.Collection{}
	for _, item := range items {
		single := c
		single.Item = item
		single.Selection = groups[item]
		single.Discovery = config.Discovery{}
		out = append(out, single)
	}
	return out, nil
}

func PlanCollection(ctx context.Context, c config.Collection, client *ia.Client) (CollectionPlan, error) {
	result := CollectionPlan{Collection: c.ID, Plans: []Plan{}}
	groups, err := split(c)
	if err != nil {
		return result, err
	}
	for _, single := range groups {
		metadata, _, err := client.Metadata(ctx, single.Item)
		if err != nil {
			return result, err
		}
		plan, err := Inventory(single, metadata)
		if err != nil {
			return result, err
		}
		result.Plans = append(result.Plans, plan)
		result.Bytes += plan.Bytes
		result.Documents += plan.Documents
	}
	if result.Bytes > c.Limits.TotalBytes || result.Documents > c.Limits.Documents {
		return result, fmt.Errorf("collection selection exceeds aggregate limits")
	}
	return result, nil
}

func Acquire(ctx context.Context, c config.Collection, client *ia.Client, store *storage.Store, db *catalog.DB) (Receipt, error) {
	if c.Discovery.Query == "" {
		return acquireItem(ctx, c, client, store, db)
	}
	receipt := Receipt{Collection: c.ID, Started: time.Now().UTC(), Files: []Result{}, Snapshots: []string{}}
	// Complete aggregate preflight before any payload transfer.
	plan, err := PlanCollection(ctx, c, client)
	if err != nil {
		return receipt, err
	}
	var disk syscall.Statfs_t
	if err = syscall.Statfs(store.Path, &disk); err != nil {
		return receipt, err
	}
	if uint64(disk.Bavail)*uint64(disk.Bsize) < uint64(2<<30)+uint64(plan.Bytes) {
		return receipt, fmt.Errorf("insufficient free space for collection batch")
	}

	groups, err := split(c)
	if err != nil {
		return receipt, err
	}
	remaining := c.Limits.TotalBytes
	for _, single := range groups {
		single.Limits.TotalBytes = remaining
		result, err := acquireItem(ctx, single, client, store, db)
		receipt.Files = append(receipt.Files, result.Files...)
		for _, file := range result.Files {
			remaining -= file.Size
		}
		if result.Snapshot != "" {
			receipt.Snapshots = append(receipt.Snapshots, result.Snapshot)
		}
		if err != nil {
			db.Collection(ctx, c)
			return receipt, err
		}
	}
	return receipt, db.Collection(ctx, c)
}
