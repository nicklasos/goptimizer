package driver

import (
	"context"
	"errors"
	"io"
	"path"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GCS owns its client: Close closes it.
type GCS struct {
	client *storage.Client
	bucket *storage.BucketHandle
	prefix string
}

// NewGCSClient signs in with a service-account key. Other credential types are refused: an
// "external_account" file makes the client fetch tokens from URLs the file itself names.
func NewGCSClient(ctx context.Context, credentialsJSON []byte) (*storage.Client, error) {
	return storage.NewClient(ctx, option.WithAuthCredentialsJSON(option.ServiceAccount, credentialsJSON))
}

func NewGCS(client *storage.Client, bucket, prefix string) *GCS {
	return &GCS{client: client, bucket: client.Bucket(bucket), prefix: prefix}
}

func (g *GCS) Close() error { return g.client.Close() }

func (g *GCS) Put(ctx context.Context, key string, r io.Reader) error {
	key, err := CleanKey(key)
	if err != nil {
		return err
	}
	// Cancelling the context is how a GCS upload is abandoned: the object is then never created.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	w := g.bucket.Object(joinPrefix(g.prefix, key)).NewWriter(ctx)
	if _, err := io.Copy(w, r); err != nil {
		cancel()
		_ = w.Close()
		return err
	}
	return w.Close()
}

func (g *GCS) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	key, err := CleanKey(key)
	if err != nil {
		return nil, err
	}
	r, err := g.bucket.Object(joinPrefix(g.prefix, key)).NewReader(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil, ErrNotFound
	}
	return r, err
}

func (g *GCS) List(ctx context.Context, prefix string) ([]Entry, error) {
	prefix, err := CleanPrefix(prefix)
	if err != nil {
		return nil, err
	}
	full := joinPrefix(g.prefix, prefix)

	entries := []Entry{}
	it := g.bucket.Objects(ctx, &storage.Query{Prefix: full, Delimiter: "/"})
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		if attrs.Prefix != "" {
			entries = append(entries, Entry{Name: strings.TrimSuffix(strings.TrimPrefix(attrs.Prefix, full), "/"), IsDir: true})
			continue
		}
		name := strings.TrimPrefix(attrs.Name, full)
		if name == "" {
			continue
		}
		entries = append(entries, Entry{Name: path.Base(name), Size: attrs.Size, ModifiedAt: attrs.Updated.UTC()})
	}
	return sortEntries(entries), nil
}

func (g *GCS) Delete(ctx context.Context, key string) error {
	key, err := CleanKey(key)
	if err != nil {
		return err
	}
	err = g.bucket.Object(joinPrefix(g.prefix, key)).Delete(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil
	}
	return err
}
