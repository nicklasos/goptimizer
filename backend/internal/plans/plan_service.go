package plans

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"app/internal/db"
	"app/internal/errs"
	"app/internal/secretbox"
	"app/internal/tools"
)

var (
	ErrPlanNotFound    = errs.NewNotFoundError(errs.ErrKeyPlansNotFound, "Plan not found")
	errNameTaken       = errs.Fields{"name": {errs.ErrKeyPlansNameTaken}}
	errStorageNotFound = errs.Fields{"storage_ids": {errs.ErrKeyPlansStorageNotFound}}
	errSecretMissing   = errors.New("the stored password is missing")
)

// Plan is a plan with the storages it uploads to, its latest run (nil before the first), whether
// a restore of it is queued or running, and when its last successful run finished and the size
// of what it stored.
type Plan struct {
	db.Plan
	Storages        []db.ListStoragesOfPlansRow
	LastRun         *db.Run
	Restoring       bool
	LastSuccessAt   sql.NullTime
	LastSuccessSize sql.NullInt64
}

// PaginatedPlansResult is one page of plans
type PaginatedPlansResult struct {
	Data  []Plan
	Total int64
}

// draft is a plan request parsed into what is stored. secret is the source credentials as
// JSON, not yet sealed, and nil when the request left them blank.
type draft struct {
	name        string
	kind        SourceKind
	config      []byte
	secret      []byte
	schedules   []byte
	maxAgeDays  int64
	keepWeekly  int64
	keepMonthly int64
	storageIDs  []int64
	enabled     bool
}

type PlanService struct {
	queries *db.Queries
	tx      *db.TxRunner
	secrets *secretbox.Box
	tools   tools.Finder
}

func NewPlanService(queries *db.Queries, tx *db.TxRunner, secrets *secretbox.Box, finder tools.Finder) *PlanService {
	return &PlanService{queries: queries, tx: tx, secrets: secrets, tools: finder}
}

func (s *PlanService) ListPlans(ctx context.Context, page, pageSize int32) (*PaginatedPlansResult, error) {
	rows, err := s.queries.ListPlansPaginated(ctx, db.ListPlansPaginatedParams{
		Limit:  int64(pageSize),
		Offset: int64(page-1) * int64(pageSize),
	})
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	total, err := s.queries.CountPlans(ctx)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	plans, err := s.withRelations(ctx, rows)
	if err != nil {
		return nil, err
	}
	return &PaginatedPlansResult{Data: plans, Total: total}, nil
}

func (s *PlanService) GetPlan(ctx context.Context, id int64) (*Plan, error) {
	row, err := s.queries.GetPlanByID(ctx, id)
	if err != nil {
		return nil, notFoundOr(err)
	}
	plans, err := s.withRelations(ctx, []db.Plan{row})
	if err != nil {
		return nil, err
	}
	return &plans[0], nil
}

func (s *PlanService) CreatePlan(ctx context.Context, req *PlanRequest) (*Plan, error) {
	d, err := parseRequest(req)
	if err != nil {
		return nil, err
	}
	secret, err := s.sealedSecret(d, nil)
	if err != nil {
		return nil, err
	}

	var id int64
	err = s.tx.WithTx(ctx, func(q *db.Queries) error {
		if err := checkStorages(ctx, q, d.storageIDs); err != nil {
			return err
		}
		plan, err := q.CreatePlan(ctx, db.CreatePlanParams{
			Name:         d.name,
			SourceKind:   string(d.kind),
			SourceConfig: string(d.config),
			SourceSecret: secret,
			Schedules:    string(d.schedules),
			MaxAgeDays:   d.maxAgeDays,
			KeepWeekly:   d.keepWeekly,
			KeepMonthly:  d.keepMonthly,
			Enabled:      d.enabled,
		})
		if err != nil {
			return uniqueNameOr(err)
		}
		id = plan.ID
		return addStorages(ctx, q, id, d.storageIDs)
	})
	if err != nil {
		return nil, err
	}
	return s.GetPlan(ctx, id)
}

func (s *PlanService) UpdatePlan(ctx context.Context, id int64, req *PlanRequest) (*Plan, error) {
	existing, err := s.GetPlan(ctx, id)
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

	err = s.tx.WithTx(ctx, func(q *db.Queries) error {
		if err := checkStorages(ctx, q, d.storageIDs); err != nil {
			return err
		}
		_, err := q.UpdatePlan(ctx, db.UpdatePlanParams{
			ID:           id,
			Name:         d.name,
			SourceKind:   string(d.kind),
			SourceConfig: string(d.config),
			SourceSecret: secret,
			Schedules:    string(d.schedules),
			MaxAgeDays:   d.maxAgeDays,
			KeepWeekly:   d.keepWeekly,
			KeepMonthly:  d.keepMonthly,
			Enabled:      d.enabled,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrPlanNotFound
			}
			return uniqueNameOr(err)
		}
		if err := q.ClearPlanStorages(ctx, id); err != nil {
			return errs.WrapDatabaseError(err)
		}
		return addStorages(ctx, q, id, d.storageIDs)
	})
	if err != nil {
		return nil, err
	}
	return s.GetPlan(ctx, id)
}

// SetEnabled pauses or resumes a plan. Resuming counts as a save, so the scheduler starts with
// the next slot, not the ones missed while paused.
func (s *PlanService) SetEnabled(ctx context.Context, id int64, enabled bool) (*Plan, error) {
	if _, err := s.queries.SetPlanEnabled(ctx, db.SetPlanEnabledParams{ID: id, Enabled: enabled}); err != nil {
		return nil, notFoundOr(err)
	}
	return s.GetPlan(ctx, id)
}

// DeletePlan removes a plan and its links to storages. The storages and what is in them stay.
func (s *PlanService) DeletePlan(ctx context.Context, id int64) error {
	if _, err := s.queries.GetPlanByID(ctx, id); err != nil {
		return notFoundOr(err)
	}
	return errs.WrapDatabaseError(s.queries.DeletePlan(ctx, id))
}

// TestDraft tests the source of a plan form before it is saved. A form that leaves the password
// blank is tested with the one of planID, the plan being edited, when it is the same kind.
func (s *PlanService) TestDraft(ctx context.Context, req *SourceRequest, planID int64) (*TestResult, error) {
	config, secret, err := parseSource(req)
	if err != nil {
		return nil, err
	}

	if secret == nil && len(sources[req.SourceKind].secretFields) > 0 {
		var existing *db.Plan
		if planID > 0 {
			row, err := s.queries.GetPlanByID(ctx, planID)
			if err != nil {
				return nil, notFoundOr(err)
			}
			existing = &row
		}
		if existing == nil || SourceKind(existing.SourceKind) != req.SourceKind || existing.SourceSecret == nil {
			return nil, secretRequired(req.SourceKind)
		}
		if secret, err = s.secrets.Open(existing.SourceSecret); err != nil {
			return connectFailed(err), nil
		}
	}
	result := sources[req.SourceKind].test(ctx, config, secret, s.tools)
	return &result, nil
}

// TestPlan tests the source of a saved plan with its stored settings.
func (s *PlanService) TestPlan(ctx context.Context, id int64) (*TestResult, error) {
	plan, err := s.queries.GetPlanByID(ctx, id)
	if err != nil {
		return nil, notFoundOr(err)
	}
	secret, err := s.unseal(&plan)
	if err != nil {
		return connectFailed(err), nil
	}
	result := sources[SourceKind(plan.SourceKind)].test(ctx, []byte(plan.SourceConfig), secret, s.tools)
	return &result, nil
}

func (s *PlanService) withRelations(ctx context.Context, rows []db.Plan) ([]Plan, error) {
	plans := make([]Plan, len(rows))
	if len(rows) == 0 {
		return plans, nil
	}
	index := make(map[int64]int, len(rows))
	ids := make([]int64, len(rows))
	for i, row := range rows {
		plans[i] = Plan{Plan: row, Storages: []db.ListStoragesOfPlansRow{}}
		index[row.ID] = i
		ids[i] = row.ID
	}
	links, err := s.queries.ListStoragesOfPlans(ctx, ids)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	for _, link := range links {
		plan := &plans[index[link.PlanID]]
		plan.Storages = append(plan.Storages, link)
	}

	latest, err := s.queries.LatestRunsOfPlans(ctx, ids)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	for _, row := range latest {
		run := db.Run(row)
		plans[index[row.PlanID]].LastRun = &run
	}
	restoring, err := s.queries.ListRestoringPlans(ctx, ids)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	for _, id := range restoring {
		plans[index[id]].Restoring = true
	}
	successes, err := s.queries.LatestSuccessesOfPlans(ctx, ids)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	for _, row := range successes {
		plans[index[row.PlanID]].LastSuccessAt = row.FinishedAt
		plans[index[row.PlanID]].LastSuccessSize = row.SizeBytes
	}
	return plans, nil
}

// sealedSecret is the secret column for a draft: the new password when it has one, the stored
// one of existing when it left it blank and is still the same kind, and nothing for a kind
// without a password.
func (s *PlanService) sealedSecret(d draft, existing *Plan) ([]byte, error) {
	if len(sources[d.kind].secretFields) == 0 {
		return nil, nil
	}
	if d.secret != nil {
		return s.secrets.Seal(d.secret), nil
	}
	if existing != nil && SourceKind(existing.SourceKind) == d.kind && existing.SourceSecret != nil {
		return existing.SourceSecret, nil
	}
	return nil, secretRequired(d.kind)
}

func (s *PlanService) unseal(plan *db.Plan) ([]byte, error) {
	if len(sources[SourceKind(plan.SourceKind)].secretFields) == 0 {
		return nil, nil
	}
	if plan.SourceSecret == nil {
		return nil, errSecretMissing
	}
	return s.secrets.Open(plan.SourceSecret)
}

// parseRequest turns a bound request into a draft. Binding has checked types, ranges and the
// fields of the source block that was sent; this checks what binding cannot, and reports every
// field it can at once.
func parseRequest(req *PlanRequest) (draft, error) {
	fields := errs.Fields{}
	d := draft{
		name:        strings.TrimSpace(req.Name),
		kind:        req.SourceKind,
		maxAgeDays:  int64(req.MaxAgeDays),
		keepWeekly:  int64(req.KeepWeekly),
		keepMonthly: int64(req.KeepMonthly),
		enabled:     req.Enabled,
	}
	if d.name == "" {
		fields.Add("name", "required")
	}

	config, secret, err := parseSource(&req.SourceRequest)
	if err != nil {
		if !merge(fields, err) {
			return draft{}, err
		}
	}
	d.config, d.secret = config, secret

	schedules, scheduleErrs := parseSchedules(req.Schedules)
	merge(fields, scheduleErrs)

	if d.maxAgeDays == 0 {
		for field, keep := range map[string]int64{"keep_weekly": d.keepWeekly, "keep_monthly": d.keepMonthly} {
			if keep > 0 {
				fields[field] = append(fields[field], errs.ErrKeyPlansKeepNeedsMaxAge)
			}
		}
	}

	for _, id := range req.StorageIDs {
		if id <= 0 {
			fields.Add("storage_ids", "invalid")
			break
		}
		if !slices.Contains(d.storageIDs, id) {
			d.storageIDs = append(d.storageIDs, id)
		}
	}

	if len(fields) > 0 {
		return draft{}, fields
	}
	if d.schedules, err = json.Marshal(schedules); err != nil {
		return draft{}, err
	}
	return d, nil
}

// merge adds a validation failure's fields to fields, and reports false for any other error.
func merge(fields errs.Fields, err error) bool {
	var failure errs.Fields
	if !errors.As(err, &failure) {
		return err == nil
	}
	for field, keys := range failure {
		fields[field] = append(fields[field], keys...)
	}
	return true
}

func checkStorages(ctx context.Context, q *db.Queries, ids []int64) error {
	count, err := q.CountStoragesByIDs(ctx, ids)
	if err != nil {
		return errs.WrapDatabaseError(err)
	}
	if count != int64(len(ids)) {
		return errStorageNotFound
	}
	return nil
}

func addStorages(ctx context.Context, q *db.Queries, planID int64, ids []int64) error {
	for _, id := range ids {
		if err := q.AddPlanStorage(ctx, db.AddPlanStorageParams{PlanID: planID, StorageID: id}); err != nil {
			return errs.WrapDatabaseError(err)
		}
	}
	return nil
}

func connectFailed(err error) *TestResult {
	return &TestResult{Failure: &TestFailure{Step: StepConnect, Message: err.Error()}}
}

func notFoundOr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrPlanNotFound
	}
	return errs.WrapDatabaseError(err)
}

func uniqueNameOr(err error) error {
	if errs.IsUniqueViolation(err, "plans.name") {
		return errNameTaken
	}
	return errs.WrapDatabaseError(err)
}
