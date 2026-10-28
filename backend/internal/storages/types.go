package storages

import (
	"app/internal"
)

// StorageRequest creates, updates or tests a storage. Exactly the block named by kind is sent.
// Credentials left blank on an update keep the stored ones.
type StorageRequest struct {
	Name  string      `json:"name" binding:"required,max=100"`
	Kind  Kind        `json:"kind" binding:"required,oneof=local s3 gcs" enums:"local,s3,gcs"`
	Local *LocalInput `json:"local" validate:"optional"`
	S3    *S3Input    `json:"s3" validate:"optional"`
	GCS   *GCSInput   `json:"gcs" validate:"optional"`
}

// LocalInput is a directory on the server, as an absolute path
type LocalInput struct {
	Path string `json:"path" binding:"required"`
}

// S3Input is an S3-compatible bucket. An empty endpoint is AWS; a MinIO, R2 or B2 endpoint
// is a URL such as https://s3.example.com.
type S3Input struct {
	Endpoint        string `json:"endpoint" validate:"optional"`
	Region          string `json:"region" validate:"optional"`
	Bucket          string `json:"bucket" binding:"required"`
	Prefix          string `json:"prefix" validate:"optional"`
	UsePathStyle    bool   `json:"use_path_style" validate:"optional"`
	AccessKeyID     string `json:"access_key_id" validate:"optional"`
	SecretAccessKey string `json:"secret_access_key" validate:"optional"`
}

// GCSInput is a Google Cloud Storage bucket and a service-account key (the JSON file)
type GCSInput struct {
	Bucket          string `json:"bucket" binding:"required"`
	Prefix          string `json:"prefix" validate:"optional"`
	CredentialsJSON string `json:"credentials_json" validate:"optional"`
}

// StorageResponse is a storage without its credentials: secret_set says whether they are stored.
// Exactly the block named by kind is present.
type StorageResponse struct {
	ID        int64        `json:"id"`
	Name      string       `json:"name"`
	Kind      Kind         `json:"kind" enums:"local,s3,gcs"`
	Local     *LocalConfig `json:"local,omitempty" validate:"optional"`
	S3        *S3Config    `json:"s3,omitempty" validate:"optional"`
	GCS       *GCSConfig   `json:"gcs,omitempty" validate:"optional"`
	SecretSet bool         `json:"secret_set"`
	CreatedAt string       `json:"created_at"`
	UpdatedAt string       `json:"updated_at"`
}

// StorageDataResponse wraps a storage
type StorageDataResponse struct {
	Data *StorageResponse `json:"data"`
}

// PaginatedStoragesResponse wraps a page of storages
type PaginatedStoragesResponse struct {
	Data       []StorageResponse       `json:"data"`
	Pagination internal.PaginationMeta `json:"pagination"`
}

// TestResultResponse is the outcome of a connection test. A failed test is a result, not an
// HTTP error: step names what failed and error says why.
type TestResultResponse struct {
	OK    bool   `json:"ok"`
	Step  string `json:"step,omitempty" validate:"optional" enums:"connect,write,read,delete"`
	Error string `json:"error,omitempty" validate:"optional"`
}

// TestResultDataResponse wraps a connection test result
type TestResultDataResponse struct {
	Data TestResultResponse `json:"data"`
}

// ObjectResponse is one folder or file in a storage listing
type ObjectResponse struct {
	Name       string `json:"name"`
	IsDir      bool   `json:"is_dir"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at,omitempty" validate:"optional"`
}

// ObjectsResponse lists one folder of a storage: its folders first, then its files
type ObjectsResponse struct {
	Data []ObjectResponse `json:"data"`
}
