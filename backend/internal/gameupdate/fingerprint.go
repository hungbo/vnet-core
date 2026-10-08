package gameupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"
)

// Fingerprint summarises a folder by every file's path, size and modification time. It is cheap
// (no file contents are read), so the master can poll it to notice when a launcher has patched a
// game, and only then pay for re-hashing.
func Fingerprint(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		// WalkDir visits in lexical order, so the hash is stable across runs.
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00%d\n", filepath.ToSlash(rel), fi.Size(), fi.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Stabilizer decides when a folder has stopped changing. A launcher patches files over minutes;
// publishing in the middle would ship a half-updated game, so a new fingerprint must hold for
// Quiet before it counts.
type Stabilizer struct {
	Quiet time.Duration

	published string
	pending   string
	since     time.Time
}

// NewStabilizer starts from the fingerprint that is already published ("" when none is).
func NewStabilizer(quiet time.Duration, published string) *Stabilizer {
	return &Stabilizer{Quiet: quiet, published: published}
}

// Observe feeds one poll result and reports whether fp is a new, settled version to publish.
func (s *Stabilizer) Observe(fp string, now time.Time) bool {
	if fp == s.published {
		s.pending = ""
		return false
	}
	if fp != s.pending {
		s.pending, s.since = fp, now
		return false
	}
	return now.Sub(s.since) >= s.Quiet
}

// MarkPublished records fp as the version now published.
func (s *Stabilizer) MarkPublished(fp string) {
	s.published, s.pending = fp, ""
}
