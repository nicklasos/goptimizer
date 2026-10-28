package factory

import (
	"encoding/json"
	"os"
	"path/filepath"

	"app/internal/db"
	"app/internal/storages"
)

type storageAttrs struct {
	name   string
	kind   storages.Kind
	config any
	secret any
}

type StorageOption func(*storageAttrs)

func WithStorageName(name string) StorageOption { return func(a *storageAttrs) { a.name = name } }

// LocalStorage is a directory on the server, the default kind.
func LocalStorage(path string) StorageOption {
	return func(a *storageAttrs) {
		a.kind, a.config, a.secret = storages.KindLocal, storages.LocalConfig{Path: path}, nil
	}
}

func S3Storage(config storages.S3Config, accessKeyID, secretAccessKey string) StorageOption {
	return func(a *storageAttrs) {
		a.kind, a.config = storages.KindS3, config
		a.secret = storages.S3Secret{AccessKeyID: accessKeyID, SecretAccessKey: secretAccessKey}
	}
}

func GCSStorage(config storages.GCSConfig, credentialsJSON string) StorageOption {
	return func(a *storageAttrs) {
		a.kind, a.config = storages.KindGCS, config
		a.secret = storages.GCSSecret{CredentialsJSON: credentialsJSON}
	}
}

// Storage creates a storage: a local directory under the system temp directory unless an option
// picks another kind. Credentials are sealed with the factory's key.
func (f *Factory) Storage(opts ...StorageOption) *db.Storage {
	f.t.Helper()

	attrs := storageAttrs{name: "Storage " + unique()}
	LocalStorage(filepath.Join(os.TempDir(), "gogobackup-factory"))(&attrs)
	for _, opt := range opts {
		opt(&attrs)
	}

	config, err := json.Marshal(attrs.config)
	if err != nil {
		f.t.Fatalf("factory.Storage: %v", err)
	}
	var secret []byte
	if attrs.secret != nil {
		plain, err := json.Marshal(attrs.secret)
		if err != nil {
			f.t.Fatalf("factory.Storage: %v", err)
		}
		secret = f.secrets.Seal(plain)
	}

	storage, err := f.queries.FactoryCreateStorage(ctx, db.FactoryCreateStorageParams{
		Name:   attrs.name,
		Kind:   string(attrs.kind),
		Config: string(config),
		Secret: secret,
	})
	if err != nil {
		f.t.Fatalf("factory.Storage: %v", err)
	}
	return &storage
}
