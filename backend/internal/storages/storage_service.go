package storages

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"app/internal/db"
	"app/internal/errs"
	"app/internal/secretbox"
	"app/internal/storages/driver"
)

// remoteTimeout bounds a connection test or a listing, so a storage that does not answer
// fails the request instead of holding it open.
const remoteTimeout = 30 * time.Second

var (
	ErrStorageNotFound = errs.NewNotFoundError(errs.ErrKeyStoragesNotFound, "Storage not found")
	ErrInvalidPrefix   = errs.NewBadRequestError(errs.ErrKeyStoragesInvalidPrefix, "The folder is not a valid path inside the storage")
	errNameTaken       = errs.Fields{"name": {errs.ErrKeyStoragesNameTaken}}
	errSecretMissing   = errors.New("the stored credentials are missing")
)

// PaginatedStoragesResult is one page of storages
type PaginatedStoragesResult struct {
	Data  []db.Storage
	Total int64
}

// TestResult is the outcome of a connection test: nil Failure means every step passed.
type TestResult struct {
	Failure *driver.ProbeError
}

// draft is a storage request parsed into what is stored. secret is the credentials as JSON,
// not yet sealed, and nil when the request left them blank.
type draft struct {
	name   string
	kind   Kind
	config []byte
	secret []byte
}

type StorageService struct {
	queries *db.Queries
	secrets *secretbox.Box
}

func NewStorageService(queries *db.Queries, secrets *secretbox.Box) *StorageService {
	return &StorageService{queries: queries, secrets: secrets}
}

func (s *StorageService) ListStorages(ctx context.Context, page, pageSize int32) (*PaginatedStoragesResult, error) {
	storages, err := s.queries.ListStoragesPaginated(ctx, db.ListStoragesPaginatedParams{
		Limit:  int64(pageSize),
		Offset: int64(page-1) * int64(pageSize),
	})
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	total, err := s.queries.CountStorages(ctx)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	if storages == nil {
		storages = []db.Storage{}
	}
	return &PaginatedStoragesResult{Data: storages, Total: total}, nil
}

func (s *StorageService) GetStorage(ctx context.Context, id int64) (*db.Storage, error) {
	storage, err := s.queries.GetStorageByID(ctx, id)
	if err != nil {
		return nil, notFoundOr(err)
	}
	return &storage, nil
}

func (s *StorageService) CreateStorage(ctx context.Context, req *StorageRequest) (*db.Storage, error) {
	d, err := parseRequest(req)
	if err != nil {
		return nil, err
	}
	secret, err := s.sealedSecret(d, nil)
	if err != nil {
		return nil, err
	}

	storage, err := s.queries.CreateStorage(ctx, db.CreateStorageParams{
		Name:   d.name,
		Kind:   string(d.kind),
		Config: string(d.config),
		Secret: secret,
	})
	if err != nil {
		return nil, uniqueNameOr(err)
	}
	return &storage, nil
}

func (s *StorageService) UpdateStorage(ctx context.Context, id int64, req *StorageRequest) (*db.Storage, error) {
	existing, err := s.GetStorage(ctx, id)
	if err != nil {
		return nil, err
	}
	d, err := parseRequest(req)
	if err != nil {
		return nil, err
	}
	secret, err := s.sealedSecret(d, existing)
	if err != nil {
		return nil, err
	}

	storage, err := s.queries.UpdateStorage(ctx, db.UpdateStorageParams{
		ID:     id,
		Name:   d.name,
		Kind:   string(d.kind),
		Config: string(d.config),
		Secret: secret,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrStorageNotFound
		}
		return nil, uniqueNameOr(err)
	}
	return &storage, nil
}

// DeleteStorage removes a storage that no plan uploads to. One that plans use is a 409 whose
// details list those plans.
func (s *StorageService) DeleteStorage(ctx context.Context, id int64) error {
	if _, err := s.GetStorage(ctx, id); err != nil {
		return err
	}
	err := s.queries.DeleteStorage(ctx, id)
	if !errs.IsForeignKeyViolation(err) {
		return errs.WrapDatabaseError(err)
	}

	plans, err := s.queries.ListPlansUsingStorage(ctx, id)
	if err != nil {
		return errs.WrapDatabaseError(err)
	}
	using := make([]map[string]any, len(plans))
	for i, plan := range plans {
		using[i] = map[string]any{"id": plan.ID, "name": plan.Name}
	}
	return errs.NewDomainError(errs.ErrKeyStoragesInUse, "Backup plans upload to this storage", http.StatusConflict).
		WithDetails(map[string]interface{}{"plans": using})
}

// TestDraft tests a storage form before it is saved. A form that leaves the credentials blank
// is tested with those of storageID, the storage being edited, when it has some for this kind.
func (s *StorageService) TestDraft(ctx context.Context, req *StorageRequest, storageID int64) (*TestResult, error) {
	d, err := parseRequest(req)
	if err != nil {
		return nil, err
	}

	secret := d.secret
	if secret == nil && len(kinds[d.kind].secretFields) > 0 {
		var existing *db.Storage
		if storageID > 0 {
			if existing, err = s.GetStorage(ctx, storageID); err != nil {
				return nil, err
			}
		}
		if existing == nil || Kind(existing.Kind) != d.kind || existing.Secret == nil {
			return nil, secretRequired(d.kind)
		}
		if secret, err = s.secrets.Open(existing.Secret); err != nil {
			return connectFailed(err), nil
		}
	}
	return s.probe(ctx, d.kind, d.config, secret), nil
}

// TestStorage tests a saved storage with its stored settings.
func (s *StorageService) TestStorage(ctx context.Context, id int64) (*TestResult, error) {
	storage, err := s.GetStorage(ctx, id)
	if err != nil {
		return nil, err
	}
	secret, err := s.unseal(storage)
	if err != nil {
		return connectFailed(err), nil
	}
	return s.probe(ctx, Kind(storage.Kind), []byte(storage.Config), secret), nil
}

// ListObjects lists one folder of a storage. A storage that cannot be reached is a 502 whose
// details carry the reason, which the browser shows.
func (s *StorageService) ListObjects(ctx context.Context, id int64, prefix string) ([]driver.Entry, error) {
	storage, err := s.GetStorage(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := driver.CleanPrefix(prefix); err != nil {
		return nil, ErrInvalidPrefix
	}

	ctx, cancel := context.WithTimeout(ctx, remoteTimeout)
	defer cancel()

	d, err := s.Driver(ctx, storage)
	if err != nil {
		return nil, Unreachable(err)
	}
	defer d.Close()
	entries, err := d.List(ctx, prefix)
	if err != nil {
		return nil, Unreachable(err)
	}
	return entries, nil
}

// Driver opens a saved storage for the runner, downloads and restores. The caller closes it.
func (s *StorageService) Driver(ctx context.Context, storage *db.Storage) (driver.Driver, error) {
	secret, err := s.unseal(storage)
	if err != nil {
		return nil, err
	}
	return kinds[Kind(storage.Kind)].open(ctx, []byte(storage.Config), secret)
}

func (s *StorageService) probe(ctx context.Context, kind Kind, config, secret []byte) *TestResult {
	ctx, cancel := context.WithTimeout(ctx, remoteTimeout)
	defer cancel()

	d, err := kinds[kind].open(ctx, config, secret)
	if err != nil {
		return connectFailed(err)
	}
	defer d.Close()
	var failure *driver.ProbeError
	errors.As(driver.Probe(ctx, d), &failure)
	return &TestResult{Failure: failure}
}

// sealedSecret is the secret column for a draft: the new credentials when it has them, the
// stored ones of existing when it left them blank and is still the same kind, and nothing for
// a kind without credentials.
func (s *StorageService) sealedSecret(d draft, existing *db.Storage) ([]byte, error) {
	if len(kinds[d.kind].secretFields) == 0 {
		return nil, nil
	}
	if d.secret != nil {
		return s.secrets.Seal(d.secret), nil
	}
	if existing != nil && Kind(existing.Kind) == d.kind && existing.Secret != nil {
		return existing.Secret, nil
	}
	return nil, secretRequired(d.kind)
}

func (s *StorageService) unseal(storage *db.Storage) ([]byte, error) {
	if len(kinds[Kind(storage.Kind)].secretFields) == 0 {
		return nil, nil
	}
	if storage.Secret == nil {
		return nil, errSecretMissing
	}
	return s.secrets.Open(storage.Secret)
}

// parseRequest turns a bound request into a draft. Binding has checked the fields of the block
// that was sent; this checks that it is the block the kind names, and what binding cannot.
func parseRequest(req *StorageRequest) (draft, error) {
	fields := errs.Fields{}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		fields.Add("name", "required")
	}
	var chosen block
	for k, spec := range kinds {
		b, present := spec.block(req)
		switch {
		case k == req.Kind && present:
			chosen = b
		case k == req.Kind:
			fields.Add(string(k), "required")
		case present:
			fields.Add(string(k), "invalid")
		}
	}
	if len(fields) > 0 {
		return draft{}, fields
	}

	config, secret, err := chosen.split()
	if err != nil {
		return draft{}, err
	}
	d := draft{name: name, kind: req.Kind}
	if d.config, err = json.Marshal(config); err != nil {
		return draft{}, err
	}
	if secret != nil {
		if d.secret, err = json.Marshal(secret); err != nil {
			return draft{}, err
		}
	}
	return d, nil
}

func secretRequired(kind Kind) errs.Fields {
	fields := errs.Fields{}
	for _, field := range kinds[kind].secretFields {
		fields.Add(field, "required")
	}
	return fields
}

func connectFailed(err error) *TestResult {
	return &TestResult{Failure: &driver.ProbeError{Step: driver.StepConnect, Err: err}}
}

// Unreachable is the 502 for a storage that failed: details.reason says why.
func Unreachable(err error) error {
	return errs.WrapDomainError(errs.ErrKeyStoragesUnreachable, "The storage could not be reached", http.StatusBadGateway, err).
		WithDetails(map[string]interface{}{"reason": err.Error()})
}

func notFoundOr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrStorageNotFound
	}
	return errs.WrapDatabaseError(err)
}

func uniqueNameOr(err error) error {
	if errs.IsUniqueViolation(err, "storages.name") {
		return errNameTaken
	}
	return errs.WrapDatabaseError(err)
}
