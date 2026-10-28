package driver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// s3PartSize bounds the memory an upload of unknown size takes: minio-go buffers one part at a
// time. S3 allows 10,000 parts, so the largest object is about 160 GB.
const s3PartSize = 16 << 20

const awsEndpoint = "s3.amazonaws.com"

// S3Options describe an S3-compatible bucket: AWS when Endpoint is empty, or MinIO, R2, B2 and
// the like at their own endpoint.
type S3Options struct {
	Endpoint        string
	Region          string
	Bucket          string
	Prefix          string
	UsePathStyle    bool
	AccessKeyID     string
	SecretAccessKey string
}

type S3 struct {
	client *minio.Client
	bucket string
	prefix string
}

func NewS3(opts S3Options) (*S3, error) {
	host, secure, err := ParseS3Endpoint(opts.Endpoint)
	if err != nil {
		return nil, err
	}
	lookup := minio.BucketLookupAuto
	if opts.UsePathStyle {
		lookup = minio.BucketLookupPath
	}
	client, err := minio.New(host, &minio.Options{
		Creds:        credentials.NewStaticV4(opts.AccessKeyID, opts.SecretAccessKey, ""),
		Secure:       secure,
		Region:       opts.Region,
		BucketLookup: lookup,
		MaxRetries:   3,
	})
	if err != nil {
		return nil, err
	}
	return &S3{client: client, bucket: opts.Bucket, prefix: opts.Prefix}, nil
}

// ParseS3Endpoint accepts "https://host", "http://host:9000" or a bare "host[:port]" (HTTPS),
// and "" for AWS. It returns the host and whether to use TLS.
func ParseS3Endpoint(endpoint string) (host string, secure bool, err error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return awsEndpoint, true, nil
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" {
		return "", false, fmt.Errorf("invalid S3 endpoint %q: expected a host such as https://s3.example.com", endpoint)
	}
	switch u.Scheme {
	case "https":
		return u.Host, true, nil
	case "http":
		return u.Host, false, nil
	default:
		return "", false, fmt.Errorf("invalid S3 endpoint %q: the scheme must be http or https", endpoint)
	}
}

func (s *S3) Put(ctx context.Context, key string, r io.Reader) error {
	key, err := CleanKey(key)
	if err != nil {
		return err
	}
	// Unsigned payloads are what minio-go sends over HTTPS anyway. Over plain HTTP it would sign each
	// chunk instead (aws-chunked), which some S3-compatible servers, and the gofakes3 test server
	// for multipart parts, do not decode. The per-part CRC32C checksum still guards the content.
	_, err = s.client.PutObject(ctx, s.bucket, joinPrefix(s.prefix, key), r, -1, minio.PutObjectOptions{
		PartSize:             s3PartSize,
		DisableContentSha256: true,
	})
	return err
}

func (s *S3) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	key, err := CleanKey(key)
	if err != nil {
		return nil, err
	}
	object, err := s.client.GetObject(ctx, s.bucket, joinPrefix(s.prefix, key), minio.GetObjectOptions{})
	if err != nil {
		return nil, s3Error(err)
	}
	// GetObject is lazy; Stat makes a missing object fail here rather than on the first read.
	if _, err := object.Stat(); err != nil {
		object.Close()
		return nil, s3Error(err)
	}
	return object, nil
}

func (s *S3) List(ctx context.Context, prefix string) ([]Entry, error) {
	prefix, err := CleanPrefix(prefix)
	if err != nil {
		return nil, err
	}
	full := joinPrefix(s.prefix, prefix)

	entries := []Entry{}
	for object := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: full}) {
		if object.Err != nil {
			return nil, s3Error(object.Err)
		}
		name := strings.TrimPrefix(object.Key, full)
		if strings.HasSuffix(name, "/") {
			entries = append(entries, Entry{Name: strings.TrimSuffix(name, "/"), IsDir: true})
			continue
		}
		if name == "" {
			continue
		}
		entries = append(entries, Entry{Name: path.Base(name), Size: object.Size, ModifiedAt: object.LastModified.UTC()})
	}
	return sortEntries(entries), nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	key, err := CleanKey(key)
	if err != nil {
		return err
	}
	err = s3Error(s.client.RemoveObject(ctx, s.bucket, joinPrefix(s.prefix, key), minio.RemoveObjectOptions{}))
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

// Close has nothing to release: minio-go keeps no connection of its own beyond the HTTP pool.
func (s *S3) Close() error { return nil }

func s3Error(err error) error {
	if err == nil {
		return nil
	}
	if minio.ToErrorResponse(err).Code == "NoSuchKey" {
		return ErrNotFound
	}
	return err
}
