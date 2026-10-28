package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"app/internal/errs"
	"app/internal/factory"
	"app/internal/storages"
	"app/tests/testkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	s3KeyID     = "AKIAEXAMPLEKEYID"
	s3SecretKey = "s3-secret-value-never-in-a-response"
	gcsKeyJSON  = `{"type":"service_account","project_id":"demo","private_key":"gcs-private-key-never-in-a-response","client_email":"backup@demo.iam.gserviceaccount.com"}`
)

func localRequest(name, path string) storages.StorageRequest {
	return storages.StorageRequest{Name: name, Kind: storages.KindLocal, Local: &storages.LocalInput{Path: path}}
}

func s3Request(name string, fake testkit.FakeS3, keyID, secret string) storages.StorageRequest {
	return storages.StorageRequest{Name: name, Kind: storages.KindS3, S3: &storages.S3Input{
		Endpoint:        fake.Endpoint,
		Region:          "us-east-1",
		Bucket:          fake.Bucket,
		Prefix:          "/nightly/",
		UsePathStyle:    true,
		AccessKeyID:     keyID,
		SecretAccessKey: secret,
	}}
}

func assertNoSecrets(t *testing.T, resp *testkit.Response) {
	t.Helper()
	for _, secret := range []string{s3SecretKey, "gcs-private-key-never-in-a-response", "credentials_json", "secret_access_key", "access_key_id"} {
		assert.NotContains(t, resp.String(), secret)
	}
}

func storedSecret(t *testing.T, app *testkit.App, id int64) []byte {
	t.Helper()
	storage, err := app.Queries.GetStorageByID(app.Ctx, id)
	require.NoError(t, err)
	return storage.Secret
}

func TestStoragesAPI_LocalCRUD(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	dir := t.TempDir()

	created := testkit.Decode[storages.StorageDataResponse](
		app.As(admin).POST("/api/v1/storages", localRequest("  Nightly  ", dir+"/")).
			AssertOK().
			AssertJSONPath("data.name", "Nightly").
			AssertJSONPath("data.kind", "local").
			AssertJSONPath("data.local.path", dir).
			AssertJSONPath("data.secret_set", false).
			AssertJSONMissing("data.s3"),
	)
	id := created.Data.ID
	app.AssertDatabaseHas("storages", testkit.Row{"id": id, "kind": "local", "secret": nil})

	app.As(admin).GET("/api/v1/storages/%d", id).AssertOK().AssertJSONPath("data.local.path", dir)

	app.Create.Storage(factory.WithStorageName("Another"))
	app.As(admin).GET("/api/v1/storages").
		AssertOK().
		AssertJSONCount("data", 2).
		AssertJSONPath("data.0.name", "Another").
		AssertJSONPath("pagination.total", 2)

	app.As(admin).PUT("/api/v1/storages/%d", localRequest("Weekly", dir), id).
		AssertOK().
		AssertJSONPath("data.name", "Weekly")

	app.As(admin).DELETE("/api/v1/storages/%d", id).AssertOK()
	app.As(admin).GET("/api/v1/storages/%d", id).AssertError(http.StatusNotFound, errs.ErrKeyStoragesNotFound)
	app.As(admin).PUT("/api/v1/storages/%d", localRequest("Weekly", dir), id).AssertNotFound()
	app.As(admin).DELETE("/api/v1/storages/%d", id).AssertNotFound()
}

func TestStoragesAPI_SecretsNeverLeaveTheAPI(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.SuperAdmin())
	fake := testkit.NewFakeS3(t)

	resp := app.As(admin).POST("/api/v1/storages", s3Request("Offsite", fake, s3KeyID, s3SecretKey)).
		AssertOK().
		AssertJSONPath("data.secret_set", true).
		AssertJSONPath("data.s3.bucket", fake.Bucket).
		AssertJSONPath("data.s3.prefix", "nightly")
	assertNoSecrets(t, resp)
	id := testkit.Decode[storages.StorageDataResponse](resp).Data.ID

	sealed := storedSecret(t, app, id)
	assert.NotContains(t, string(sealed), s3SecretKey, "the column holds ciphertext")
	plain, err := app.Secrets.Open(sealed)
	require.NoError(t, err)
	assert.Contains(t, string(plain), s3SecretKey)

	assertNoSecrets(t, app.As(admin).GET("/api/v1/storages/%d", id).AssertOK())
	assertNoSecrets(t, app.As(admin).GET("/api/v1/storages").AssertOK())

	gcs := app.As(admin).POST("/api/v1/storages", storages.StorageRequest{
		Name: "Cloud", Kind: storages.KindGCS,
		GCS: &storages.GCSInput{Bucket: "archive", Prefix: "db", CredentialsJSON: gcsKeyJSON},
	}).AssertOK().AssertJSONPath("data.secret_set", true).AssertJSONPath("data.gcs.bucket", "archive")
	assertNoSecrets(t, gcs)

	t.Run("blank credentials on an update keep the stored ones", func(t *testing.T) {
		resp := app.As(admin).PUT("/api/v1/storages/%d", s3Request("Offsite renamed", fake, "", ""), id).
			AssertOK().
			AssertJSONPath("data.name", "Offsite renamed").
			AssertJSONPath("data.secret_set", true)
		assertNoSecrets(t, resp)
		assert.Equal(t, sealed, storedSecret(t, app, id))

		app.As(admin).POST("/api/v1/storages/%d/test", nil, id).AssertOK().AssertJSONPath("data.ok", true)
	})

	t.Run("new credentials replace the stored ones", func(t *testing.T) {
		resp := app.As(admin).PUT("/api/v1/storages/%d", s3Request("Offsite", fake, "NEWKEY", "replaced-secret"), id).AssertOK()
		assertNoSecrets(t, resp)
		assert.NotContains(t, resp.String(), "replaced-secret")

		plain, err := app.Secrets.Open(storedSecret(t, app, id))
		require.NoError(t, err)
		var secret storages.S3Secret
		require.NoError(t, json.Unmarshal(plain, &secret))
		assert.Equal(t, storages.S3Secret{AccessKeyID: "NEWKEY", SecretAccessKey: "replaced-secret"}, secret)
	})

	t.Run("half the credentials are refused", func(t *testing.T) {
		app.As(admin).PUT("/api/v1/storages/%d", s3Request("Offsite", fake, "ONLYTHEID", ""), id).
			AssertFieldError("s3.secret_access_key", "validation.s3.secret_access_key.required")
	})

	t.Run("changing the kind needs that kind's credentials", func(t *testing.T) {
		app.As(admin).PUT("/api/v1/storages/%d", storages.StorageRequest{
			Name: "Offsite", Kind: storages.KindGCS, GCS: &storages.GCSInput{Bucket: "archive"},
		}, id).AssertFieldError("gcs.credentials_json", "validation.gcs.credentials_json.required")

		app.As(admin).PUT("/api/v1/storages/%d", localRequest("Offsite", t.TempDir()), id).
			AssertOK().
			AssertJSONPath("data.secret_set", false)
		app.AssertDatabaseHas("storages", testkit.Row{"id": id, "secret": nil})
	})
}

func TestStoragesAPI_Validation(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	fake := testkit.NewFakeS3(t)
	app.Create.Storage(factory.WithStorageName("Taken"))

	cases := []struct {
		name  string
		body  any
		field string
		key   string
	}{
		{"name is required", localRequest("   ", "/tmp"), "name", "validation.name.required"},
		{"kind must be known", `{"name":"x","kind":"ftp"}`, "kind", "validation.kind.oneof"},
		{"the kind's block is required", storages.StorageRequest{Name: "x", Kind: storages.KindS3}, "s3", "validation.s3.required"},
		{"another kind's block is refused", storages.StorageRequest{Name: "x", Kind: storages.KindLocal, Local: &storages.LocalInput{Path: "/tmp"}, GCS: &storages.GCSInput{Bucket: "b"}}, "gcs", "validation.gcs.invalid"},
		{"the path must be absolute", localRequest("x", "relative/dir"), "local.path", "validation.local.path.invalid"},
		{"the path is required", storages.StorageRequest{Name: "x", Kind: storages.KindLocal, Local: &storages.LocalInput{}}, "local.path", "validation.local.path.required"},
		{"the bucket is required", storages.StorageRequest{Name: "x", Kind: storages.KindGCS, GCS: &storages.GCSInput{CredentialsJSON: gcsKeyJSON}}, "gcs.bucket", "validation.gcs.bucket.required"},
		{"credentials are required on create", s3Request("x", fake, "", ""), "s3.access_key_id", "validation.s3.access_key_id.required"},
		{"the endpoint must be a URL", storages.StorageRequest{Name: "x", Kind: storages.KindS3, S3: &storages.S3Input{Endpoint: "ftp://host", Bucket: "b", AccessKeyID: "a", SecretAccessKey: "b"}}, "s3.endpoint", "validation.s3.endpoint.invalid"},
		{"the prefix cannot climb", storages.StorageRequest{Name: "x", Kind: storages.KindS3, S3: &storages.S3Input{Bucket: "b", Prefix: "a/../..", AccessKeyID: "a", SecretAccessKey: "b"}}, "s3.prefix", "validation.s3.prefix.invalid"},
		{"the key file must be a service account", storages.StorageRequest{Name: "x", Kind: storages.KindGCS, GCS: &storages.GCSInput{Bucket: "b", CredentialsJSON: `{"type":"external_account"}`}}, "gcs.credentials_json", "validation.gcs.credentials_json.invalid"},
		{"the name is unique", localRequest("taken", "/tmp"), "name", errs.ErrKeyStoragesNameTaken},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app.As(admin).POST("/api/v1/storages", tc.body).AssertFieldError(tc.field, tc.key)
		})
	}
	app.AssertDatabaseCount("storages", 1)

	t.Run("renaming onto a taken name", func(t *testing.T) {
		other := app.Create.Storage()
		app.As(admin).PUT("/api/v1/storages/%d", localRequest("TAKEN", "/tmp"), other.ID).
			AssertFieldError("name", errs.ErrKeyStoragesNameTaken)
	})
}

func TestStoragesAPI_TestConnection(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	fake := testkit.NewFakeS3(t)
	dir := t.TempDir()

	t.Run("a writable directory passes and is left clean", func(t *testing.T) {
		app.As(admin).POST("/api/v1/storages/test", localRequest("x", dir)).
			AssertOK().
			AssertJSONPath("data.ok", true).
			AssertJSONMissing("data.step")
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("a missing directory fails at the write step", func(t *testing.T) {
		resp := app.As(admin).POST("/api/v1/storages/test", localRequest("x", filepath.Join(dir, "missing"))).
			AssertOK().
			AssertJSONPath("data.ok", false).
			AssertJSONPath("data.step", "write")
		assert.Contains(t, resp.String(), "no such file or directory")
	})

	t.Run("an S3 bucket passes", func(t *testing.T) {
		app.As(admin).POST("/api/v1/storages/test", s3Request("x", fake, s3KeyID, s3SecretKey)).
			AssertOK().
			AssertJSONPath("data.ok", true)
	})

	t.Run("a missing S3 bucket fails at the write step", func(t *testing.T) {
		req := s3Request("x", fake, s3KeyID, s3SecretKey)
		req.S3.Bucket = "no-such-bucket"
		app.As(admin).POST("/api/v1/storages/test", req).
			AssertOK().
			AssertJSONPath("data.ok", false).
			AssertJSONPath("data.step", "write")
	})

	t.Run("an edit form with blank credentials uses the stored ones", func(t *testing.T) {
		saved := app.Create.Storage(factory.S3Storage(storages.S3Config{Endpoint: fake.Endpoint, Region: "us-east-1", Bucket: fake.Bucket, UsePathStyle: true}, s3KeyID, s3SecretKey))

		app.As(admin).POST("/api/v1/storages/test?storage_id=%d", s3Request("x", fake, "", ""), saved.ID).
			AssertOK().
			AssertJSONPath("data.ok", true)
		app.As(admin).POST("/api/v1/storages/test", s3Request("x", fake, "", "")).
			AssertFieldError("s3.access_key_id", "validation.s3.access_key_id.required")
		app.As(admin).POST("/api/v1/storages/test?storage_id=%d", s3Request("x", fake, "", ""), 999999).AssertNotFound()
		app.As(admin).POST("/api/v1/storages/test?storage_id=abc", s3Request("x", fake, "", "")).AssertInvalid("storage_id")
	})

	t.Run("a saved storage", func(t *testing.T) {
		ok := app.Create.Storage(factory.LocalStorage(dir))
		app.As(admin).POST("/api/v1/storages/%d/test", nil, ok.ID).AssertOK().AssertJSONPath("data.ok", true)

		missing := app.Create.Storage(factory.LocalStorage(filepath.Join(dir, "missing")))
		app.As(admin).POST("/api/v1/storages/%d/test", nil, missing.ID).AssertOK().AssertJSONPath("data.step", "write")

		app.As(admin).POST("/api/v1/storages/%d/test", nil, 999999).AssertNotFound()
	})

	t.Run("credentials sealed with another key fail at the connect step", func(t *testing.T) {
		storage := app.Create.Storage(factory.S3Storage(storages.S3Config{Endpoint: fake.Endpoint, Bucket: fake.Bucket}, "a", "b"))
		_, err := app.DB.Exec(`UPDATE storages SET secret = ? WHERE id = ?`, []byte("not sealed with this key"), storage.ID)
		require.NoError(t, err)

		app.As(admin).POST("/api/v1/storages/%d/test", nil, storage.ID).
			AssertOK().
			AssertJSONPath("data.ok", false).
			AssertJSONPath("data.step", "connect")
	})
}

func TestStoragesAPI_ListObjects(t *testing.T) {
	app := testkit.New(t)
	admin := app.Create.User(factory.Admin())
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "2026", "10"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0o640))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "2026", "10", "db.dump.gz"), []byte("archive"), 0o640))
	storage := app.Create.Storage(factory.LocalStorage(dir))

	app.As(admin).GET("/api/v1/storages/%d/objects", storage.ID).
		AssertOK().
		AssertJSONCount("data", 2).
		AssertJSONPath("data.0.name", "2026").
		AssertJSONPath("data.0.is_dir", true).
		AssertJSONPath("data.1.name", "readme.txt").
		AssertJSONPath("data.1.is_dir", false).
		AssertJSONPath("data.1.size", 5)

	app.As(admin).GET("/api/v1/storages/%d/objects?prefix=2026/10/", storage.ID).
		AssertOK().
		AssertJSONCount("data", 1).
		AssertJSONPath("data.0.name", "db.dump.gz").
		AssertJSONPath("data.0.size", 7)

	app.As(admin).GET("/api/v1/storages/%d/objects?prefix=nothing/", storage.ID).AssertOK().AssertJSONCount("data", 0)
	app.As(admin).GET("/api/v1/storages/%d/objects?prefix=../", storage.ID).
		AssertError(http.StatusBadRequest, errs.ErrKeyStoragesInvalidPrefix)
	app.As(admin).GET("/api/v1/storages/%d/objects", 999999).AssertNotFound()

	gone := app.Create.Storage(factory.LocalStorage(filepath.Join(dir, "missing")))
	resp := app.As(admin).GET("/api/v1/storages/%d/objects", gone.ID).
		AssertError(http.StatusBadGateway, errs.ErrKeyStoragesUnreachable)
	assert.Contains(t, resp.String(), "no such file or directory")

	t.Run("an S3 bucket under its prefix", func(t *testing.T) {
		fake := testkit.NewFakeS3(t)
		s3 := app.Create.Storage(factory.S3Storage(storages.S3Config{Endpoint: fake.Endpoint, Region: "us-east-1", Bucket: fake.Bucket, Prefix: "nightly", UsePathStyle: true}, "a", "b"))
		d, err := storagesService(app).Driver(app.Ctx, s3)
		require.NoError(t, err)
		t.Cleanup(func() { d.Close() })
		require.NoError(t, d.Put(app.Ctx, "db/2026-10-17.dump.gz", strings.NewReader("x")))

		app.As(admin).GET("/api/v1/storages/%d/objects", s3.ID).
			AssertOK().
			AssertJSONCount("data", 1).
			AssertJSONPath("data.0.name", "db").
			AssertJSONPath("data.0.is_dir", true)
		app.As(admin).GET("/api/v1/storages/%d/objects?prefix=db/", s3.ID).
			AssertOK().
			AssertJSONPath("data.0.name", "2026-10-17.dump.gz")
	})
}

func TestStoragesAPI_AdminsOnly(t *testing.T) {
	app := testkit.New(t)
	user := app.Create.User()
	storage := app.Create.Storage()

	one := fmt.Sprintf("/api/v1/storages/%d", storage.ID)
	body := localRequest("x", "/tmp")
	routes := []struct {
		name string
		send func(*testkit.Client) *testkit.Response
	}{
		{"list", func(c *testkit.Client) *testkit.Response { return c.GET("/api/v1/storages") }},
		{"create", func(c *testkit.Client) *testkit.Response { return c.POST("/api/v1/storages", body) }},
		{"test a form", func(c *testkit.Client) *testkit.Response { return c.POST("/api/v1/storages/test", body) }},
		{"get", func(c *testkit.Client) *testkit.Response { return c.GET(one) }},
		{"update", func(c *testkit.Client) *testkit.Response { return c.PUT(one, body) }},
		{"delete", func(c *testkit.Client) *testkit.Response { return c.DELETE(one) }},
		{"test", func(c *testkit.Client) *testkit.Response { return c.POST(one+"/test", nil) }},
		{"browse", func(c *testkit.Client) *testkit.Response { return c.GET(one + "/objects") }},
	}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			route.send(app.Client).AssertUnauthorized()
			route.send(app.As(user)).AssertError(http.StatusForbidden, errs.ErrKeyForbidden)
		})
	}
	app.AssertDatabaseCount("storages", 1)
}

func storagesService(app *testkit.App) *storages.StorageService {
	return storages.NewStorageService(app.Queries, app.Secrets)
}
