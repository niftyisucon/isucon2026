package isureal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
)

// Files are immutable and content addressed. SQL references are published only
// after the file and its directory entry have reached durable storage.
// Unreferenced files are deliberately retained: rollback/reset cannot delete a
// file that a concurrent committed transaction or initial image still needs.
type ImageStore struct{ dir string }

func NewImageStore(dir string) (*ImageStore, error) {
	if !filepath.IsAbs(dir) {
		return nil, errors.New("image directory must be absolute")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	parent, err := os.Open(filepath.Dir(dir))
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	if err = parent.Sync(); err != nil {
		return nil, err
	}
	return &ImageStore{dir: dir}, nil
}

func validStorageKey(key string) bool {
	if len(key) != 64 {
		return false
	}
	for _, c := range key {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func (s *ImageStore) Put(data []byte) (string, error) {
	sum := sha256.Sum256(data)
	key := hex.EncodeToString(sum[:])
	// Even on an existing key use a fresh fsync+rename. A concurrent writer may
	// have created its entry but not yet synced the parent directory.
	f, err := os.CreateTemp(s.dir, ".image-")
	if err != nil {
		return "", err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(name, filepath.Join(s.dir, key)); err != nil {
		return "", err
	}
	dir, err := os.Open(s.dir)
	if err != nil {
		return "", err
	}
	defer dir.Close()
	if err = dir.Sync(); err != nil {
		return "", err
	}
	return key, nil
}

func (s *ImageStore) Get(key string) ([]byte, error) {
	if !validStorageKey(key) {
		return nil, errors.New("invalid image storage key")
	}
	data, err := os.ReadFile(filepath.Join(s.dir, key))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != key {
		return nil, errors.New("image checksum mismatch")
	}
	return data, nil
}

type preparedImage struct {
	data []byte
	key  *string
}

func (h *HandlerContext) prepareImage(data []byte) (preparedImage, error) {
	if h.Files == nil {
		return preparedImage{data: data}, nil
	}
	key, err := h.Files.Put(data)
	if err != nil {
		return preparedImage{}, fmt.Errorf("persist image: %w", err)
	}
	return preparedImage{data: []byte{}, key: &key}, nil
}
func (h *HandlerContext) insertImage(ctx context.Context, tx *sqlx.Tx, id, owner string, image preparedImage, createdAt *time.Time) error {
	stored, key := image.data, image.key

	if h.Files == nil { // Legacy mode does not require a schema migration.
		_, err := tx.ExecContext(ctx, `INSERT INTO image_objects(id,owner_user_id,data,media_type,created_at) VALUES($1,$2,$3,'image/jpeg',COALESCE($4::timestamptz,clock_timestamp()))`, id, owner, stored, createdAt)
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO image_objects(id,owner_user_id,data,media_type,created_at,storage_key) VALUES($1,$2,$3,'image/jpeg',COALESCE($4::timestamptz,clock_timestamp()),$5)`, id, owner, stored, createdAt, key)
	return err
}

// Legacy/initial blobs are authoritative when no file reference is present.
// A missing referenced file must never produce a successful empty image.
func (h *HandlerContext) imageBytes(data []byte, key *string) ([]byte, error) {
	if key == nil {
		return data, nil
	}
	if h.Files == nil {
		return nil, errors.New("image store unavailable; keep ISUREAL_IMAGE_DIR configured")
	}
	file, err := h.Files.Get(*key)
	if err == nil {
		return file, nil
	}
	if len(data) > 0 {
		return data, nil
	}
	return nil, err
}
