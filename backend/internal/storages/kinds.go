package storages

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"app/internal/errs"
	"app/internal/storages/driver"
)

type Kind string

const (
	KindLocal Kind = "local"
	KindS3    Kind = "s3"
	KindGCS   Kind = "gcs"
)

// kind is everything that differs between storage kinds. Each kind has its own block in a
// request and a response, named after it, a typed config stored as JSON in storages.config,
// and, for a kind with credentials, a typed secret sealed in storages.secret.
type kind struct {
	// block returns the kind's block of a request, and whether the request has one.
	block func(*StorageRequest) (block, bool)
	// secretFields are the request fields of the credentials; none for a kind without.
	secretFields []string
	// open builds a driver from the stored config and the unsealed secret.
	open func(ctx context.Context, config, secret []byte) (driver.Driver, error)
	// respond puts the stored config into the kind's block of a response.
	respond func(config []byte, r *StorageResponse) error
}

// block is one kind's block of a request.
type block interface {
	// split checks the block and returns the config to store and the credentials, or nil
	// credentials when they were left blank. Failures are keyed by field path ("s3.bucket").
	split() (config any, secret any, err error)
}

var kinds = map[Kind]kind{
	KindLocal: {
		block: func(r *StorageRequest) (block, bool) { return r.Local, r.Local != nil },
		open: func(_ context.Context, config, _ []byte) (driver.Driver, error) {
			var c LocalConfig
			if err := json.Unmarshal(config, &c); err != nil {
				return nil, err
			}
			return driver.NewLocal(c.Path)
		},
		respond: func(config []byte, r *StorageResponse) error { return json.Unmarshal(config, &r.Local) },
	},
	KindS3: {
		block:        func(r *StorageRequest) (block, bool) { return r.S3, r.S3 != nil },
		secretFields: []string{"s3.access_key_id", "s3.secret_access_key"},
		open: func(_ context.Context, config, secret []byte) (driver.Driver, error) {
			var c S3Config
			var s S3Secret
			if err := unmarshalBoth(config, &c, secret, &s); err != nil {
				return nil, err
			}
			return driver.NewS3(driver.S3Options{
				Endpoint:        c.Endpoint,
				Region:          c.Region,
				Bucket:          c.Bucket,
				Prefix:          c.Prefix,
				UsePathStyle:    c.UsePathStyle,
				AccessKeyID:     s.AccessKeyID,
				SecretAccessKey: s.SecretAccessKey,
			})
		},
		respond: func(config []byte, r *StorageResponse) error { return json.Unmarshal(config, &r.S3) },
	},
	KindGCS: {
		block:        func(r *StorageRequest) (block, bool) { return r.GCS, r.GCS != nil },
		secretFields: []string{"gcs.credentials_json"},
		open: func(ctx context.Context, config, secret []byte) (driver.Driver, error) {
			var c GCSConfig
			var s GCSSecret
			if err := unmarshalBoth(config, &c, secret, &s); err != nil {
				return nil, err
			}
			client, err := driver.NewGCSClient(ctx, []byte(s.CredentialsJSON))
			if err != nil {
				return nil, err
			}
			return driver.NewGCS(client, c.Bucket, c.Prefix), nil
		},
		respond: func(config []byte, r *StorageResponse) error { return json.Unmarshal(config, &r.GCS) },
	},
}

// LocalConfig is a directory on this server.
type LocalConfig struct {
	Path string `json:"path"`
}

// S3Config is an S3-compatible bucket. An empty endpoint is AWS.
type S3Config struct {
	Endpoint     string `json:"endpoint"`
	Region       string `json:"region"`
	Bucket       string `json:"bucket"`
	Prefix       string `json:"prefix"`
	UsePathStyle bool   `json:"use_path_style"`
}

// GCSConfig is a Google Cloud Storage bucket.
type GCSConfig struct {
	Bucket string `json:"bucket"`
	Prefix string `json:"prefix"`
}

// S3Secret and GCSSecret are the credentials as sealed in storages.secret. No response carries
// them: a response only says whether they are set.
type S3Secret struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

type GCSSecret struct {
	CredentialsJSON string `json:"credentials_json"`
}

func (in *LocalInput) split() (any, any, error) {
	path := strings.TrimSpace(in.Path)
	if !filepath.IsAbs(path) {
		return nil, nil, errs.FieldError("local.path", "invalid")
	}
	return LocalConfig{Path: filepath.Clean(path)}, nil, nil
}

func (in *S3Input) split() (any, any, error) {
	fields := errs.Fields{}
	endpoint := strings.TrimSpace(in.Endpoint)
	if _, _, err := driver.ParseS3Endpoint(endpoint); err != nil {
		fields.Add("s3.endpoint", "invalid")
	}
	prefix, ok := cleanBucketPrefix(in.Prefix)
	if !ok {
		fields.Add("s3.prefix", "invalid")
	}

	var secret any
	keyID, key := strings.TrimSpace(in.AccessKeyID), strings.TrimSpace(in.SecretAccessKey)
	switch {
	case keyID != "" && key != "":
		secret = S3Secret{AccessKeyID: keyID, SecretAccessKey: key}
	case keyID != "":
		fields.Add("s3.secret_access_key", "required")
	case key != "":
		fields.Add("s3.access_key_id", "required")
	}

	if len(fields) > 0 {
		return nil, nil, fields
	}
	return S3Config{
		Endpoint:     endpoint,
		Region:       strings.TrimSpace(in.Region),
		Bucket:       strings.TrimSpace(in.Bucket),
		Prefix:       prefix,
		UsePathStyle: in.UsePathStyle,
	}, secret, nil
}

func (in *GCSInput) split() (any, any, error) {
	fields := errs.Fields{}
	prefix, ok := cleanBucketPrefix(in.Prefix)
	if !ok {
		fields.Add("gcs.prefix", "invalid")
	}

	var secret any
	if credentials := strings.TrimSpace(in.CredentialsJSON); credentials != "" {
		var object map[string]any
		if json.Unmarshal([]byte(credentials), &object) != nil || object["type"] != "service_account" {
			fields.Add("gcs.credentials_json", "invalid")
		}
		secret = GCSSecret{CredentialsJSON: credentials}
	}

	if len(fields) > 0 {
		return nil, nil, fields
	}
	return GCSConfig{Bucket: strings.TrimSpace(in.Bucket), Prefix: prefix}, secret, nil
}

// cleanBucketPrefix stores "a/b" for "/a/b/", and refuses a prefix that climbs with "..".
func cleanBucketPrefix(prefix string) (string, bool) {
	cleaned, err := driver.CleanPrefix(strings.Trim(strings.TrimSpace(prefix), "/"))
	if err != nil {
		return "", false
	}
	return strings.TrimSuffix(cleaned, "/"), true
}

func unmarshalBoth(config []byte, c any, secret []byte, s any) error {
	if err := json.Unmarshal(config, c); err != nil {
		return err
	}
	return json.Unmarshal(secret, s)
}
