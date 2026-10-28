package unit

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"app/internal/storages/driver"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func drivers(t *testing.T, bucketPrefix string) map[string]driver.Driver {
	t.Helper()

	local, err := driver.NewLocal(t.TempDir())
	require.NoError(t, err)

	fakeS3 := testkit.NewFakeS3(t)
	s3, err := driver.NewS3(driver.S3Options{
		Endpoint:        fakeS3.Endpoint,
		Region:          "us-east-1",
		Bucket:          fakeS3.Bucket,
		Prefix:          bucketPrefix,
		UsePathStyle:    true,
		AccessKeyID:     "test",
		SecretAccessKey: "test-secret",
	})
	require.NoError(t, err)

	gcs := driver.NewGCS(testkit.NewFakeGCS(t), testkit.FakeBucket, bucketPrefix)

	return map[string]driver.Driver{"local": local, "s3": s3, "gcs": gcs}
}

func put(t *testing.T, d driver.Driver, key, content string) {
	t.Helper()
	require.NoError(t, d.Put(context.Background(), key, strings.NewReader(content)))
}

func read(t *testing.T, d driver.Driver, key string) string {
	t.Helper()
	r, err := d.Open(context.Background(), key)
	require.NoError(t, err)
	defer r.Close()
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(b)
}

type listed struct {
	Name  string
	IsDir bool
	Size  int64
}

func list(t *testing.T, d driver.Driver, prefix string) []listed {
	t.Helper()
	entries, err := d.List(context.Background(), prefix)
	require.NoError(t, err)
	out := []listed{}
	for _, e := range entries {
		out = append(out, listed{e.Name, e.IsDir, e.Size})
		if !e.IsDir {
			assert.False(t, e.ModifiedAt.IsZero(), "%s has a modification time", e.Name)
		}
	}
	return out
}

func TestStorageDrivers_RoundTrip(t *testing.T) {
	for _, bucketPrefix := range []string{"", "nightly/db"} {
		for name, d := range drivers(t, bucketPrefix) {
			t.Run(name+" prefix="+bucketPrefix, func(t *testing.T) {
				ctx := context.Background()

				put(t, d, "a.txt", "hello")
				put(t, d, "dir/b.txt", "bee")
				put(t, d, "dir/sub/c.txt", "sea")
				put(t, d, "a.txt", "hello again")

				assert.Equal(t, "hello again", read(t, d, "a.txt"))
				assert.Equal(t, "sea", read(t, d, "dir/sub/c.txt"))

				assert.Equal(t, []listed{{"dir", true, 0}, {"a.txt", false, 11}}, list(t, d, ""))
				assert.Equal(t, []listed{{"sub", true, 0}, {"b.txt", false, 3}}, list(t, d, "dir"))
				assert.Equal(t, list(t, d, "dir"), list(t, d, "dir/"))
				assert.Empty(t, list(t, d, "nothing-here/"))

				_, err := d.Open(ctx, "missing.txt")
				assert.ErrorIs(t, err, driver.ErrNotFound)

				require.NoError(t, d.Delete(ctx, "a.txt"))
				_, err = d.Open(ctx, "a.txt")
				assert.ErrorIs(t, err, driver.ErrNotFound)
				assert.NoError(t, d.Delete(ctx, "a.txt"), "deleting a missing object succeeds")

				assert.NoError(t, driver.Probe(ctx, d))
				assert.Equal(t, []listed{{"dir", true, 0}}, list(t, d, ""), "the probe leaves nothing behind")
			})
		}
	}
}

func TestStorageDrivers_RejectKeysOutsideTheRoot(t *testing.T) {
	ctx := context.Background()
	for name, d := range drivers(t, "inside") {
		t.Run(name, func(t *testing.T) {
			for _, key := range []string{"../escape.txt", "a/../../escape.txt", "/etc/passwd", "", "a\\..\\b"} {
				assert.ErrorIs(t, d.Put(ctx, key, strings.NewReader("x")), driver.ErrInvalidKey, key)
				_, err := d.Open(ctx, key)
				assert.ErrorIs(t, err, driver.ErrInvalidKey, key)
				assert.ErrorIs(t, d.Delete(ctx, key), driver.ErrInvalidKey, key)
			}
			_, err := d.List(ctx, "../")
			assert.ErrorIs(t, err, driver.ErrInvalidKey)
		})
	}
}

func TestLocalDriver_StaysInsideItsDirectory(t *testing.T) {
	ctx := context.Background()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("do not read"), 0o600))

	root := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "link")))
	d, err := driver.NewLocal(root)
	require.NoError(t, err)

	_, err = d.Open(ctx, "link/secret.txt")
	assert.Error(t, err, "a symlink does not lead out of the root")
	assert.Error(t, d.Put(ctx, "link/planted.txt", strings.NewReader("x")))
	assert.NoFileExists(t, filepath.Join(outside, "planted.txt"))
	assert.Error(t, d.Delete(ctx, "link/secret.txt"))
	assert.FileExists(t, filepath.Join(outside, "secret.txt"))

	put(t, d, "kept.txt", "inside")
	assert.FileExists(t, filepath.Join(root, "kept.txt"))

	_, err = driver.NewLocal("relative/path")
	assert.Error(t, err)
}

func TestProbe_ReportsTheFailingStep(t *testing.T) {
	missing, err := driver.NewLocal(filepath.Join(t.TempDir(), "does-not-exist"))
	require.NoError(t, err)

	err = driver.Probe(context.Background(), missing)

	var probeErr *driver.ProbeError
	require.True(t, errors.As(err, &probeErr), "got %v", err)
	assert.Equal(t, driver.StepWrite, probeErr.Step)

	s3, err := driver.NewS3(driver.S3Options{Endpoint: testkit.NewFakeS3(t).Endpoint, Bucket: "no-such-bucket", UsePathStyle: true, AccessKeyID: "a", SecretAccessKey: "b"})
	require.NoError(t, err)
	err = driver.Probe(context.Background(), s3)
	require.True(t, errors.As(err, &probeErr), "got %v", err)
	assert.Equal(t, driver.StepWrite, probeErr.Step)
}

func TestS3Driver_StreamsAnObjectLargerThanOnePart(t *testing.T) {
	fake := testkit.NewFakeS3(t)
	d, err := driver.NewS3(driver.S3Options{Endpoint: fake.Endpoint, Region: "us-east-1", Bucket: fake.Bucket, UsePathStyle: true, AccessKeyID: "a", SecretAccessKey: "b"})
	require.NoError(t, err)

	content := make([]byte, 17<<20)
	rand.Read(content)
	require.NoError(t, d.Put(context.Background(), "big.bin", io.MultiReader(bytes.NewReader(content))))

	r, err := d.Open(context.Background(), "big.bin")
	require.NoError(t, err)
	defer r.Close()
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(content, got), "read back %d bytes of %d", len(got), len(content))
}

func TestParseS3Endpoint(t *testing.T) {
	cases := []struct {
		in     string
		host   string
		secure bool
		ok     bool
	}{
		{"", "s3.amazonaws.com", true, true},
		{"https://s3.eu-central-1.wasabisys.com", "s3.eu-central-1.wasabisys.com", true, true},
		{"http://minio:9000", "minio:9000", false, true},
		{"minio.internal:9000", "minio.internal:9000", true, true},
		{"https://host/path", "", false, false},
		{"ftp://host", "", false, false},
	}
	for _, tc := range cases {
		host, secure, err := driver.ParseS3Endpoint(tc.in)
		if !tc.ok {
			assert.Error(t, err, tc.in)
			continue
		}
		require.NoError(t, err, tc.in)
		assert.Equal(t, tc.host, host, tc.in)
		assert.Equal(t, tc.secure, secure, tc.in)
	}
}
