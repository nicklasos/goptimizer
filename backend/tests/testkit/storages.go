package testkit

import (
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

const FakeBucket = "backups"

// FakeS3 is an in-memory S3 server on a loopback port with one empty bucket, FakeBucket.
// It accepts any access key.
type FakeS3 struct {
	Endpoint string
	Bucket   string
}

func NewFakeS3(t testing.TB) FakeS3 {
	t.Helper()

	backend := s3mem.New()
	if err := backend.CreateBucket(FakeBucket); err != nil {
		t.Fatalf("testkit: create fake S3 bucket: %v", err)
	}
	server := httptest.NewServer(gofakes3.New(backend).Server())
	t.Cleanup(server.Close)
	return FakeS3{Endpoint: server.URL, Bucket: FakeBucket}
}

// NewFakeGCS is an in-process GCS server with one empty bucket, FakeBucket, and a client for it.
// Nothing listens on a port.
func NewFakeGCS(t testing.TB) *storage.Client {
	t.Helper()

	server, err := fakestorage.NewServerWithOptions(fakestorage.Options{NoListener: true})
	if err != nil {
		t.Fatalf("testkit: start fake GCS: %v", err)
	}
	t.Cleanup(server.Stop)
	server.CreateBucketWithOpts(fakestorage.CreateBucketOpts{Name: FakeBucket})
	return server.Client()
}
