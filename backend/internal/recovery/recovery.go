// Package recovery copies immutable preservation state and portable catalog backups.
package recovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"

	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/catalog"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/storage"
	"github.com/jackc/pgx/v5"
	"strconv"
)

func Audit(ctx context.Context, db *catalog.DB, s *storage.Store) error {
	rows, e := db.Pool.Query(ctx, `SELECT sha256,size FROM assets ORDER BY sha256`)
	if e != nil {
		return e
	}
	defer rows.Close()
	var failures []string
	for rows.Next() {
		var h string
		var n int64
		if e = rows.Scan(&h, &n); e != nil {
			return e
		}
		if e = s.Verify(h, n); e != nil {
			failures = append(failures, h)
		}
	}
	if rows.Err() != nil {
		return rows.Err()
	}
	if len(failures) > 0 {
		return fmt.Errorf("integrity failures: %v", failures)
	}
	return nil
}

type AuditResult struct {
	Assets    int `json:"assets"`
	Variants  int `json:"variants"`
	Published int `json:"published"`
}

// Inspect checks preserved originals and the readable derivative references used
// by published documents. Unpublished unreadable sources remain preservable.
func Inspect(ctx context.Context, db *catalog.DB, s *storage.Store) (AuditResult, error) {
	var result AuditResult
	if err := Audit(ctx, db, s); err != nil {
		return result, err
	}
	if err := db.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM assets), (SELECT count(*) FROM variants), (SELECT count(*) FROM documents WHERE published)`).Scan(&result.Assets, &result.Variants, &result.Published); err != nil {
		return result, err
	}
	rows, err := db.Pool.Query(ctx, `SELECT DISTINCT v.asset_sha FROM variants v JOIN documents d ON d.id=v.document_id LEFT JOIN derivatives x ON x.asset_sha=v.asset_sha WHERE d.published AND (x.pages IS NULL OR x.pages<1 OR x.status='failed')`)
	if err != nil {
		return result, err
	}
	missing := rows.Next()
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if missing {
		return result, fmt.Errorf("published document has unavailable derivative")
	}
	rows, err = db.Pool.Query(ctx, `SELECT DISTINCT v.asset_sha FROM variants v JOIN documents d ON d.id=v.document_id WHERE d.published`)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var hash string
		if err = rows.Scan(&hash); err != nil {
			return result, err
		}
		var manifest struct {
			Input string `json:"input_sha256"`
			Pages int    `json:"pages"`
		}
		if err = s.JSON("derivatives/"+hash+"/manifest.json", &manifest); err != nil || manifest.Input != hash || manifest.Pages < 1 {
			return result, fmt.Errorf("published derivative manifest unavailable")
		}
	}
	return result, rows.Err()
}
func copyTree(source, destination string, basis ...string) error {
	root, e := os.OpenRoot(source)
	if e != nil {
		return e
	}
	defer root.Close()
	var previous *os.Root
	if len(basis) > 1 {
		return fmt.Errorf("one backup basis required")
	}
	if len(basis) == 1 && basis[0] != "" {
		previous, e = os.OpenRoot(basis[0])
		if e != nil {
			return e
		}
		defer previous.Close()
	}
	return fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup rejects symlink %s", path)
		}
		target := filepath.Join(destination, path)
		if entry.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if path == ".writer.lock" {
			return nil
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("backup rejects special file")
		}
		in, e := root.Open(path)
		if e != nil {
			return e
		}
		defer in.Close()
		if previous != nil {
			prior, err := previous.Lstat(path)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if err == nil && !prior.Mode().IsRegular() {
				return fmt.Errorf("backup basis rejects nonregular file")
			}
			if err == nil && prior.Mode().Perm()&0222 != 0 {
				return fmt.Errorf("backup basis file must be read-only")
			}
			if err == nil && prior.Size() == info.Size() {
				inputHash, _, err := storage.Digest(in)
				if err != nil {
					return err
				}
				old, err := previous.Open(path)
				if err != nil {
					return err
				}
				oldHash, _, err := storage.Digest(old)
				old.Close()
				if err != nil {
					return err
				}
				if inputHash == oldHash {
					// Link a verified earlier snapshot, never the mutable source tree.
					if err := os.Link(filepath.Join(basis[0], path), target); err != nil {
						return fmt.Errorf("backup basis must share the destination filesystem: %w", err)
					}
					return nil
				}
				if _, err := in.Seek(0, io.SeekStart); err != nil {
					return err
				}
			}
		}
		out, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		_, e = io.Copy(out, in)
		if e == nil {
			e = out.Sync()
		}
		ce := out.Close()
		if e != nil {
			return e
		}
		return ce
	})
}
func tool(ctx context.Context, name, path, urlValue string, maximumBytes ...int64) error {
	cfg, e := pgx.ParseConfig(urlValue)
	if e != nil {
		return fmt.Errorf("invalid database configuration")
	}
	var args []string
	if name == "pg_dump" {
		args = []string{"--format=custom", "--file", path}
	} else {
		args = []string{"--exit-on-error", "--single-transaction", "--dbname", cfg.Database, path}
	}
	cmd := exec.CommandContext(ctx, name, args...)
	var dumpOutput *os.File
	if name == "pg_dump" && len(maximumBytes) == 1 && maximumBytes[0] > 0 {
		output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer output.Close()
		dumpOutput = output
		cmd = exec.CommandContext(ctx, name, "--format=custom")
		cmd.Stdout = &boundedOutput{writer: output, remaining: maximumBytes[0]}
	}
	sslMode := "prefer"
	parsed, parseErr := url.Parse(urlValue)
	if parseErr != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return fmt.Errorf("backup tools require a PostgreSQL URL")
	}
	if value := parsed.Query().Get("sslmode"); value != "" {
		sslMode = value
	}
	cmd.Env = append(os.Environ(), "PGDATABASE="+cfg.Database, "PGHOST="+cfg.Host, "PGPORT="+strconv.Itoa(int(cfg.Port)), "PGUSER="+cfg.User, "PGPASSWORD="+cfg.Password, "PGSSLMODE="+sslMode)
	for queryKey, envKey := range map[string]string{"sslrootcert": "PGSSLROOTCERT", "sslcert": "PGSSLCERT", "sslkey": "PGSSLKEY"} {
		if value := parsed.Query().Get(queryKey); value != "" {
			cmd.Env = append(cmd.Env, envKey+"="+value)
		}
	}
	// Diagnostics can contain connection details; return a bounded nonsecret error.
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("%s failed", name)
	}
	if dumpOutput != nil {
		return dumpOutput.Sync()
	}
	return nil
}

type boundedOutput struct {
	writer    io.Writer
	remaining int64
}

func (w *boundedOutput) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, fmt.Errorf("backup byte budget reached")
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	return n, err
}

type BackupOptions struct {
	Basis        string
	MaximumBytes int64
}

func Backup(ctx context.Context, db *catalog.DB, s *storage.Store, url, destination string, options ...BackupOptions) error {
	src, e := filepath.Abs(s.Path)
	if e != nil {
		return e
	}
	dst, e := filepath.Abs(destination)
	if e != nil {
		return e
	}
	if dst == src || filepath.IsLocal(mustRelative(src, dst)) {
		return fmt.Errorf("backup destination must be outside archive")
	}
	var previous []string
	if len(options) > 1 {
		return fmt.Errorf("one backup basis required")
	}
	var option BackupOptions
	if len(options) == 1 {
		option = options[0]
	}
	if option.MaximumBytes < 0 {
		return fmt.Errorf("invalid backup byte budget")
	}
	if option.Basis != "" {
		old, err := filepath.Abs(option.Basis)
		if err != nil || old == src || filepath.IsLocal(mustRelative(src, old)) || old == dst {
			return fmt.Errorf("backup basis must be a separate verified snapshot")
		}
		if err := verifyBasis(old); err != nil {
			return err
		}
		previous = []string{filepath.Join(old, "archive")}
	}
	var sourceBytes int64
	if err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("backup rejects nonregular source")
		}
		if path != filepath.Join(src, ".writer.lock") {
			sourceBytes += info.Size()
		}
		return nil
	}); err != nil {
		return err
	}
	if option.MaximumBytes > 0 && sourceBytes >= option.MaximumBytes {
		return fmt.Errorf("archive exceeds backup byte budget")
	}
	if e = Audit(ctx, db, s); e != nil {
		return e
	}
	if e = os.Mkdir(dst, 0700); e != nil {
		return e
	}
	var dumpLimit int64
	if option.MaximumBytes > 0 {
		dumpLimit = option.MaximumBytes - sourceBytes
	}
	if e = tool(ctx, "pg_dump", filepath.Join(dst, "catalog.dump"), url, dumpLimit); e != nil {
		return e
	}
	if e = copyTree(src, filepath.Join(dst, "archive"), previous...); e != nil {
		return e
	}
	hash, n, e := digestFile(filepath.Join(dst, "catalog.dump"))
	if e != nil {
		return e
	}
	files, e := treeDigests(filepath.Join(dst, "archive"))
	if e != nil {
		return e
	}
	return sWrite(filepath.Join(dst, "complete.json"), backupManifest{1, true, hash, n, files})
}

func verifyBasis(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe backup basis root")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return err
	}
	defer root.Close()
	info, err = root.Lstat("complete.json")
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16<<20 {
		return fmt.Errorf("unsafe backup basis manifest")
	}
	raw, err := root.ReadFile("complete.json")
	var manifest backupManifest
	if err != nil || json.Unmarshal(raw, &manifest) != nil || manifest.Format != 1 || !manifest.Complete {
		return fmt.Errorf("incomplete backup basis")
	}
	info, err = root.Lstat("catalog.dump")
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("unsafe backup basis catalog")
	}
	info, err = root.Lstat("archive")
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe backup basis archive")
	}
	hash, size, err := digestFile(filepath.Join(path, "catalog.dump"))
	if err != nil || hash != manifest.CatalogSHA || size != manifest.CatalogSize {
		return fmt.Errorf("backup basis catalog mismatch")
	}
	files, err := treeDigests(filepath.Join(path, "archive"))
	if err != nil || !reflect.DeepEqual(files, manifest.Files) {
		return fmt.Errorf("backup basis archive mismatch")
	}
	return nil
}
func mustRelative(a, b string) string {
	p, e := filepath.Rel(a, b)
	if e != nil {
		return ".."
	}
	return p
}
func sWrite(path string, v any) error { return writeJSON(path, v) }
func Restore(ctx context.Context, url, backup, destination string) error {
	b, e := os.ReadFile(filepath.Join(backup, "complete.json"))
	var manifest backupManifest
	if e != nil || json.Unmarshal(b, &manifest) != nil || manifest.Format != 1 || !manifest.Complete {
		return fmt.Errorf("incomplete backup")
	}
	hash, size, e := digestFile(filepath.Join(backup, "catalog.dump"))
	if e != nil || hash != manifest.CatalogSHA || size != manifest.CatalogSize {
		return fmt.Errorf("catalog backup integrity mismatch")
	}
	actual, e := treeDigests(filepath.Join(backup, "archive"))
	if e != nil || !reflect.DeepEqual(actual, manifest.Files) {
		return fmt.Errorf("archive backup integrity mismatch")
	}
	db, e := catalog.Open(ctx, url)
	if e != nil {
		return e
	}
	defer db.Pool.Close()
	var count int
	if e = db.Pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='public'`).Scan(&count); e != nil {
		return e
	}
	if count > 0 {
		return fmt.Errorf("restore requires an empty database")
	}
	if _, e = os.Stat(destination); e == nil {
		return fmt.Errorf("restore requires a new archive destination")
	} else if !os.IsNotExist(e) {
		return e
	}
	if e = copyTree(filepath.Join(backup, "archive"), destination); e != nil {
		return e
	}
	if e = tool(ctx, "pg_restore", filepath.Join(backup, "catalog.dump"), url); e != nil {
		return e
	}
	s, e := storage.Open(destination)
	if e != nil {
		return e
	}
	defer s.Close()
	return Audit(ctx, db, s)
}

type backupManifest struct {
	Format      int               `json:"format"`
	Complete    bool              `json:"complete"`
	CatalogSHA  string            `json:"catalog_sha256"`
	CatalogSize int64             `json:"catalog_size"`
	Files       map[string]string `json:"files"`
}

func digestFile(path string) (string, int64, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", 0, e
	}
	defer f.Close()
	return storage.Digest(f)
}

func treeDigests(path string) (map[string]string, error) {
	root, e := os.OpenRoot(path)
	if e != nil {
		return nil, e
	}
	defer root.Close()
	result := map[string]string{}
	e = fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup symlink rejected")
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("backup special file rejected")
		}
		f, e := root.Open(path)
		if e != nil {
			return e
		}
		h, _, e := storage.Digest(f)
		f.Close()
		if e != nil {
			return e
		}
		result[path] = h
		return nil
	})
	return result, e
}
