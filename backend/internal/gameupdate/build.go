// Package gameupdate distributes game folders between the VNET master machine and cafe servers
// over BitTorrent: the master builds a torrent from a game folder it keeps up to date, and each
// cafe server downloads it in place on top of its previous copy, fetching only changed pieces.
package gameupdate

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
)

const (
	minPieceLength = 256 << 10
	maxPieceLength = 4 << 20
	// Aim for about this many pieces: smaller pieces mean less re-download when one file changes,
	// more pieces mean a bigger .torrent. 20k × 20-byte hashes is 400 KB of metadata.
	targetPieces = 20000
)

// BuildTorrent hashes every file under dir into a multi-file torrent called name.
// The torrent is always multi-file, even for one file, so the files land directly under the
// destination folder on the cafe side.
func BuildTorrent(dir, name string) (*metainfo.MetaInfo, error) {
	if name == "" || strings.ContainsAny(name, `/\`) {
		return nil, fmt.Errorf("invalid torrent name %q", name)
	}
	files, total, err := listFiles(dir)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s: no files to publish", dir)
	}

	info := metainfo.Info{Name: name, PieceLength: pieceLengthFor(total), Files: files}
	err = info.GeneratePieces(func(fi metainfo.FileInfo) (io.ReadCloser, error) {
		return os.Open(filepath.Join(append([]string{dir}, fi.Path...)...))
	})
	if err != nil {
		return nil, fmt.Errorf("hashing %s: %w", dir, err)
	}
	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		return nil, err
	}
	return &metainfo.MetaInfo{InfoBytes: infoBytes, CreatedBy: "VNET"}, nil
}

// listFiles returns the regular files under dir in a stable order. Symlinks are rejected rather
// than followed: a link out of the game folder would publish (or overwrite) unrelated files.
func listFiles(dir string) (files []metainfo.FileInfo, total int64, err error) {
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s: symlinks are not supported", path)
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s: not a regular file", path)
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, metainfo.FileInfo{
			Length: fi.Size(),
			Path:   strings.Split(filepath.ToSlash(rel), "/"),
		})
		total += fi.Size()
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	sort.Slice(files, func(i, j int) bool {
		return strings.Join(files[i].Path, "/") < strings.Join(files[j].Path, "/")
	})
	return files, total, nil
}

func pieceLengthFor(total int64) int64 {
	l := int64(minPieceLength)
	for l < maxPieceLength && total/l > targetPieces {
		l *= 2
	}
	return l
}

// LoadTorrent parses .torrent bytes and checks they describe a multi-file torrent, the only shape
// BuildTorrent produces and Install accepts.
func LoadTorrent(b []byte) (*metainfo.MetaInfo, *metainfo.Info, error) {
	mi, err := metainfo.Load(strings.NewReader(string(b)))
	if err != nil {
		return nil, nil, err
	}
	info, err := mi.UnmarshalInfo()
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return nil, nil, errors.New("torrent is not a multi-file torrent")
	}
	return mi, &info, nil
}
