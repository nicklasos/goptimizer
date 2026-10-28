package runs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"app/internal/db"
	"app/internal/errs"
	"app/internal/plans"
)

var (
	ErrRunNotFound   = errs.NewNotFoundError(errs.ErrKeyRunsNotFound, "Run not found")
	ErrRunInProgress = errs.NewDomainError(errs.ErrKeyPlansRunInProgress, "The plan already has a run queued or running", http.StatusConflict)
)

// RunDetail is a run with its log and the archives it stored.
type RunDetail struct {
	db.Run
	Archives []db.ListArchivesOfRunRow
}

type PaginatedRunsResult struct {
	Data  []db.ListRunsOfPlanPaginatedRow
	Total int64
}

type PaginatedArchivesResult struct {
	Data  []db.ListLiveArchivesOfPlanRow
	Total int64
}

// RunService is the API's side of runs: it enqueues them and reads them back. The Worker runs them.
type RunService struct {
	queries *db.Queries
	tx      *db.TxRunner
	// wake tells this process's worker that a run was queued. It is nil when the worker runs in
	// cmd/cron, which finds the run by polling.
	wake func()
}

func NewRunService(queries *db.Queries, tx *db.TxRunner, wake func()) *RunService {
	return &RunService{queries: queries, tx: tx, wake: wake}
}

// RunNow queues a manual run of the plan. A plan with a run queued or running already is a 409.
func (s *RunService) RunNow(ctx context.Context, planID int64) (*db.Run, error) {
	var run db.Run
	err := s.tx.WithTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetPlanByID(ctx, planID); err != nil {
			return planNotFoundOr(err)
		}
		active, err := q.CountActiveRunsOfPlan(ctx, planID)
		if err != nil {
			return errs.WrapDatabaseError(err)
		}
		if active > 0 {
			return ErrRunInProgress
		}
		if run, err = q.CreateManualRun(ctx, planID); err != nil {
			return errs.WrapDatabaseError(err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.notify()
	return &run, nil
}

// EnqueueDue queues the slot each enabled plan is due for at now, and returns how many it
// queued. A slot is queued once: running this again, here or in another process, adds nothing.
func (s *RunService) EnqueueDue(ctx context.Context, now time.Time) (int, error) {
	enabled, err := s.queries.ListEnabledPlans(ctx)
	if err != nil {
		return 0, errs.WrapDatabaseError(err)
	}
	queued := 0
	var problems []error
	for _, plan := range enabled {
		var schedules []plans.Schedule
		if err := json.Unmarshal([]byte(plan.Schedules), &schedules); err != nil {
			problems = append(problems, fmt.Errorf("plan %d: schedules: %w", plan.ID, err))
			continue
		}
		last, err := s.lastSlot(ctx, plan.ID)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		slot, ok := Due(schedules, plan.Enabled, plan.UpdatedAt.Time, last, now)
		if !ok {
			continue
		}
		added, err := s.queries.EnqueueScheduledRun(ctx, db.EnqueueScheduledRunParams{
			PlanID:       plan.ID,
			ScheduledFor: sql.NullTime{Time: slot, Valid: true},
		})
		if err != nil {
			problems = append(problems, fmt.Errorf("plan %d: %w", plan.ID, err))
			continue
		}
		queued += int(added)
	}
	if queued > 0 {
		s.notify()
	}
	return queued, errors.Join(problems...)
}

func (s *RunService) lastSlot(ctx context.Context, planID int64) (*time.Time, error) {
	last, err := s.queries.LastScheduledSlot(ctx, planID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !last.Valid) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("plan %d: last slot: %w", planID, err)
	}
	return &last.Time, nil
}

func (s *RunService) notify() {
	if s.wake != nil {
		s.wake()
	}
}

func (s *RunService) ListRuns(ctx context.Context, planID int64, page, pageSize int32) (*PaginatedRunsResult, error) {
	if _, err := s.queries.GetPlanByID(ctx, planID); err != nil {
		return nil, planNotFoundOr(err)
	}
	rows, err := s.queries.ListRunsOfPlanPaginated(ctx, db.ListRunsOfPlanPaginatedParams{
		PlanID: planID,
		Limit:  int64(pageSize),
		Offset: int64(page-1) * int64(pageSize),
	})
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	total, err := s.queries.CountRunsOfPlan(ctx, planID)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	return &PaginatedRunsResult{Data: rows, Total: total}, nil
}

func (s *RunService) GetRun(ctx context.Context, id int64) (*RunDetail, error) {
	run, err := s.queries.GetRunByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRunNotFound
	}
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	archives, err := s.queries.ListArchivesOfRun(ctx, id)
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	return &RunDetail{Run: run, Archives: archives}, nil
}

// ListArchives lists the archives of a plan that retention has not deleted, newest first, of
// one storage when storageID is not 0.
func (s *RunService) ListArchives(ctx context.Context, planID, storageID int64, page, pageSize int32) (*PaginatedArchivesResult, error) {
	if _, err := s.queries.GetPlanByID(ctx, planID); err != nil {
		return nil, planNotFoundOr(err)
	}
	rows, err := s.queries.ListLiveArchivesOfPlan(ctx, db.ListLiveArchivesOfPlanParams{
		PlanID:    planID,
		StorageID: storageID,
		Limit:     int64(pageSize),
		Offset:    int64(page-1) * int64(pageSize),
	})
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	total, err := s.queries.CountLiveArchivesOfPlan(ctx, db.CountLiveArchivesOfPlanParams{PlanID: planID, StorageID: storageID})
	if err != nil {
		return nil, errs.WrapDatabaseError(err)
	}
	return &PaginatedArchivesResult{Data: rows, Total: total}, nil
}

func planNotFoundOr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return plans.ErrPlanNotFound
	}
	return errs.WrapDatabaseError(err)
}
