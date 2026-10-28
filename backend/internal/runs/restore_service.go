package runs

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"app/internal/db"
	"app/internal/errs"
	"app/internal/plans"
	"app/internal/secretbox"
)

var (
	ErrRestoreNotFound    = errs.NewNotFoundError(errs.ErrKeyRestoresNotFound, "Restore not found")
	ErrRestoreInProgress  = errs.NewDomainError(errs.ErrKeyRestoresInProgress, "The plan already has a restore queued or running", http.StatusConflict)
	ErrArchiveKindChanged = errs.NewDomainError(errs.ErrKeyRestoresKindChanged, "The archive was made while the plan backed up another kind of source", http.StatusConflict)
)

// Restore is a restore with what the lists show next to it: its archive and storage (empty
// once the archive row is gone) and who started it.
type Restore = db.GetRestoreByIDRow

type PaginatedRestoresResult struct {
	Data  []Restore
	Total int64
}

// RestoreService is the API's side of restores: it checks and queues them and reads them back.
// The Worker runs them.
type RestoreService struct {
	queries *db.Queries
	tx      *db.TxRunner
	secrets *secretbox.Box
	env     plans.TargetEnv
	wake    func()
}

// NewRestoreService takes the work directory and the service's own database path in env.
func NewRestoreService(queries *db.Queries, tx *db.TxRunner, secrets *secretbox.Box, env plans.TargetEnv, wake func()) *RestoreService {
	return &RestoreService{queries: queries, tx: tx, secrets: secrets, env: env, wake: wake}
}

// DefaultTarget is the target a restore of the archive starts from: the plan's own source, or
// for files a new directory in the work directory.
func (s *RestoreService) DefaultTarget(ctx context.Context, archiveID int64, now time.Time) (plans.RestoreTarget, error) {
	_, plan, err := s.archiveAndPlan(ctx, archiveID)
	if err != nil {
		return plans.RestoreTarget{}, err
	}
	env := s.env
	env.Now = now
	return plans.DefaultRestoreTarget(&plan, env)
}

// Start queues a restore of the archive into the request's target. The worker starts it once
// no run or other restore of the plan is running. A plan with a restore queued or running
// already is a 409.
func (s *RestoreService) Start(ctx context.Context, archiveID, userID int64, req *plans.RestoreRequest, now time.Time) (*Restore, error) {
	archive, plan, err := s.archiveAndPlan(ctx, archiveID)
	if err != nil {
		return nil, err
	}
	env := s.env
	env.Now = now
	target, err := plans.ParseRestoreTarget(&plan, req, env)
	if err != nil {
		return nil, err
	}
	var secret []byte
	if target.Secret != nil {
		secret = s.secrets.Seal(target.Secret)
	}

	var restore db.Restore
	err = s.tx.WithTx(ctx, func(q *db.Queries) error {
		active, err := q.CountActiveRestoresOfPlan(ctx, plan.ID)
		if err != nil {
			return errs.WrapDatabaseError(err)
		}
		if active > 0 {
			return ErrRestoreInProgress
		}
		restore, err = q.CreateRestore(ctx, db.CreateRestoreParams{
			ArchiveID:    sql.NullInt64{Int64: archive.ID, Valid: true},
			PlanID:       plan.ID,
			SourceKind:   plan.SourceKind,
			TargetConfig: string(target.Config),
			TargetSecret: secret,
			CreatedBy:    sql.NullInt64{Int64: userID, Valid: true},
		})
		return errs.WrapDatabaseError(err)
	})
	if err != nil {
		return nil, err
	}
	if s.wake != nil {
		s.wake()
	}
	return s.GetRestore(ctx, restore.ID)
}

// archiveAndPlan is a live archive and its plan. An archive made while the plan backed up
// another kind of source cannot go back into what the plan backs up now.
func (s *RestoreService) archiveAndPlan(ctx context.Context, archiveID int64) (db.Archive, db.Plan, error) {
	archive, err := liveArchive(ctx, s.queries, archiveID)
	if err != nil {
		return db.Archive{}, db.Plan{}, err
	}
	plan, err := s.queries.GetPlanByID(ctx, archive.PlanID)
	if err != nil {
		return db.Archive{}, db.Plan{}, planNotFoundOr(err)
	}
	if !strings.HasSuffix(archive.ObjectKey, "."+plans.ArchiveExtension(plans.SourceKind(plan.SourceKind))) {
		return db.Archive{}, db.Plan{}, ErrArchiveKindChanged
	}
	return archive, plan, nil
}

func (s *RestoreService) GetRestore(ctx context.Context, id int64) (*Restore, error) {
	row, err := s.queries.GetRestoreByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRestoreNotFound
	}
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	return &row, nil
}

// ListRestores lists the restores of a plan, newest first, without their logs.
func (s *RestoreService) ListRestores(ctx context.Context, planID int64, page, pageSize int32) (*PaginatedRestoresResult, error) {
	if _, err := s.queries.GetPlanByID(ctx, planID); err != nil {
		return nil, planNotFoundOr(err)
	}
	rows, err := s.queries.ListRestoresOfPlanPaginated(ctx, db.ListRestoresOfPlanPaginatedParams{
		PlanID: planID,
		Limit:  int64(pageSize),
		Offset: int64(page-1) * int64(pageSize),
	})
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	total, err := s.queries.CountRestoresOfPlan(ctx, planID)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	result := &PaginatedRestoresResult{Data: make([]Restore, len(rows)), Total: total}
	for i, row := range rows {
		result.Data[i] = Restore{
			Restore: db.Restore{
				ID: row.ID, ArchiveID: row.ArchiveID, PlanID: row.PlanID, SourceKind: row.SourceKind,
				TargetConfig: row.TargetConfig, TargetSecret: row.TargetSecret, Status: row.Status, Error: row.Error,
				CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt,
			},
			ArchiveKey: row.ArchiveKey, ArchiveSize: row.ArchiveSize, ArchiveCreatedAt: row.ArchiveCreatedAt,
			StorageName: row.StorageName, CreatedByName: row.CreatedByName, CreatedByEmail: row.CreatedByEmail,
		}
	}
	return result, nil
}
