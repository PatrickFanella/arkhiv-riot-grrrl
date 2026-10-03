// arkhiv is the operator CLI; mutations are serialized and never exposed publicly.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/catalog"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/config"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/fixtures"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/index"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/ingest"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/recovery"
	ia "git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/source/internetarchive"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/storage"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/updates"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if e := run(ctx, os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: arkhiv approve-public|revoke-public|plan|refresh|acquire-changes|changes|review-change|acquire|migrate|list|inspect|edit|group|publish|unpublish|extract|audit|backup|restore|seed-fixture [flags]")
	}
	command := args[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	conf := flags.String("config", "../collections/riot-grrrl.yaml", "collection YAML")
	archive := flags.String("archive", os.Getenv("ARKHIV_ARCHIVE"), "archive root")
	migrations := flags.String("migrations", "../migrations", "migration directory")
	variant := flags.String("variant", "", "variant ID for grouping or public revocation")
	id := flags.String("id", "", "document ID")
	offset := flags.Int("offset", 0, "review queue offset, 500 events per page")
	forceCheck := flags.Bool("force-source-check", false, "bypass refresh cadence for operator qualification; retain enable and backoff gates")
	scheduled := flags.Bool("scheduled", false, "honor per-collection enable and cadence")
	decision := flags.String("decision", "", "source change decision: approve, reject or hold")
	note := flags.String("review", "", "recorded review note")
	rights := flags.String("rights", "", "rights/access statement")
	patch := flags.String("metadata", "{}", "editorial override JSON")
	destination := flags.String("destination", "", "new backup or restore path")
	backup := flags.String("backup", "", "backup source")
	manifest := flags.String("manifest", "", "reviewed immutable public access manifest")
	output := flags.String("output", "", "inventory/receipt output JSON")
	if e := flags.Parse(args[1:]); e != nil {
		return e
	}
	if flags.NArg() > 0 {
		return errors.New("unexpected arguments")
	}
	if command == "plan" {
		c, e := config.Load(*conf)
		if e != nil {
			return e
		}
		if c.Discovery.Query != "" {
			p, err := ingest.PlanCollection(ctx, c, ia.New())
			if err != nil {
				return err
			}
			if *output != "" {
				if err = config.SaveJSON(*output, p); err != nil {
					return err
				}
			}
			return json.NewEncoder(os.Stdout).Encode(p)
		}
		m, raw, e := ia.New().Metadata(ctx, c.Item)
		if e != nil {
			return e
		}
		p, e := ingest.Inventory(c, m)
		if e != nil {
			return e
		}
		if *output != "" {
			if e = config.SaveJSON(*output, p); e != nil {
				return e
			}
			if e = os.WriteFile(*output+".source.json", raw, 0600); e != nil {
				return e
			}
		}
		return json.NewEncoder(os.Stdout).Encode(p)
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL required")
	}
	if command == "restore" {
		return recovery.Restore(ctx, url, *backup, *destination)
	}
	db, e := catalog.Open(ctx, url)
	if e != nil {
		return e
	}
	defer db.Pool.Close()
	if command == "migrate" {
		return db.Migrate(ctx, *migrations)
	}
	s, e := storage.Open(*archive)
	if e != nil {
		return e
	}
	defer s.Close()
	lock, e := s.Root.OpenFile(".writer.lock", os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		return errors.New("another archive operation is active")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	switch command {
	case "approve-public":
		f, err := os.Open(*manifest)
		if err != nil {
			return err
		}
		defer f.Close()
		raw, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
		if err != nil {
			return err
		}
		count, err := db.ApprovePublic(ctx, s, raw, *note)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"approved_variants": count, "reuse_license_asserted": false})
	case "revoke-public":
		return db.RevokePublic(ctx, *variant, *note)

	case "refresh", "changes":
		c, err := config.Load(*conf)
		if err != nil {
			return err
		}
		if command == "changes" {
			queue, err := updates.QueuePage(ctx, db, c.ID, *offset)
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(queue)
		}
		honorCadence := *scheduled && (!*forceCheck || !c.Updates.Enabled)
		due, status, err := updates.Due(ctx, c, db, honorCadence)
		if err != nil {
			return err
		}
		if !due {
			if status == "source-backoff" {
				return errors.New("source backoff active; successful source receipt unchanged")
			}
			return json.NewEncoder(os.Stdout).Encode(map[string]any{"status": status, "checked": 0, "acquired": 0})
		}
		report, err := updates.Refresh(ctx, c, ia.New(), s, db)
		var acquired updates.Acquisition
		if err == nil && c.Updates.Policy == "acquire" {
			acquired, err = updates.Acquire(ctx, c, ia.New(), s, db)
		}

		if encodeErr := json.NewEncoder(os.Stdout).Encode(map[string]any{"inventory": report, "acquisition": acquired}); encodeErr != nil {
			return encodeErr
		}
		return err
	case "acquire-changes":
		c, err := config.Load(*conf)
		if err != nil {
			return err
		}
		report, err := updates.Acquire(ctx, c, ia.New(), s, db)
		if encodeErr := json.NewEncoder(os.Stdout).Encode(report); encodeErr != nil {
			return encodeErr
		}
		return err
	case "review-change":
		return updates.Review(ctx, db, *id, *decision, *note)
	case "acquire":
		c, e := config.Load(*conf)
		if e != nil {
			return e
		}
		if err := db.Collection(ctx, c); err != nil {
			return err
		}
		due, status, err := updates.Due(ctx, c, db, false)
		if err != nil {
			return err
		}
		if !due {
			return fmt.Errorf("source acquisition deferred: %s", status)
		}
		receipt, e := ingest.Acquire(ctx, c, ia.New(), s, db)
		var backoff *ia.RetryError
		if errors.As(e, &backoff) {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			e = errors.Join(e, db.SourceBackoff(cleanup, c.ID, "initial-acquisition", backoff.At))
			cancel()
		}

		if *output != "" {
			if we := config.SaveJSON(*output, receipt); we != nil {
				return we
			}
		}
		if je := json.NewEncoder(os.Stdout).Encode(receipt); je != nil {
			return je
		}
		return e
	case "list":
		c, e := config.Load(*conf)
		if e != nil {
			return e
		}
		docs, total, e := db.List(ctx, c.ID, "", "", "", 50, 0, false)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"documents": docs, "total": total})
	case "inspect":
		doc, e := db.Get(ctx, *id, false)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(doc)
	case "edit", "publish", "unpublish":
		var values map[string]any
		if e = json.Unmarshal([]byte(*patch), &values); e != nil {
			return e
		}
		for k, v := range values {
			if k == "subjects" {
				items, ok := v.([]any)
				if !ok {
					return errors.New("subjects must be an array")
				}
				for _, item := range items {
					if _, ok := item.(string); !ok {
						return errors.New("subjects must contain text")
					}
				}
			} else if k == "title" || k == "creator" || k == "date" || k == "language" {
				if _, ok := v.(string); !ok {
					return errors.New("metadata values must be text")
				}
			} else {
				return fmt.Errorf("unsupported editorial field %s", k)
			}
		}
		var publish *bool
		if command != "edit" {
			yes := command == "publish"
			publish = &yes
			if yes {
				doc, e := db.Get(ctx, *id, false)
				if e != nil {
					return e
				}
				if len(doc.Variants) == 0 {
					return errors.New("verified readable variant required")
				}
				for _, v := range doc.Variants {
					if e = s.Verify(v.SHA, v.Size); e != nil {
						return e
					}
					f, e := s.Read(v.SHA)
					if e != nil {
						return e
					}
					b := make([]byte, 5)
					_, e = io.ReadFull(f, b)
					f.Close()
					if e != nil || string(b) != "%PDF-" {
						return errors.New("asset is not a PDF")
					}
					if v.Pages < 1 || v.TextStatus == "failed" {
						return errors.New("successful PDF extraction required before publication")
					}
				}
			}
		}
		return db.Edit(ctx, *id, values, *rights, *note, publish)
	case "group":
		return db.Group(ctx, *variant, *id, *note)
	case "extract":
		return index.All(ctx, db, s)
	case "audit":
		report, err := recovery.Inspect(ctx, db, s)
		if encodeErr := json.NewEncoder(os.Stdout).Encode(report); encodeErr != nil {
			return encodeErr
		}
		return err
	case "backup":
		if *destination == "" {
			return errors.New("new backup destination required")
		}
		return recovery.Backup(ctx, db, s, url, *destination)
	case "seed-fixture":
		return fixtures.Seed(ctx, db, s)
	default:
		return fmt.Errorf("unknown command %s", command)
	}
}
