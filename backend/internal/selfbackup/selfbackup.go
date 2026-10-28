// Package selfbackup makes sure Gogobackup backs up its own database: a local storage and a
// SQLite plan for DATABASE_PATH, created through the same services as the API.
package selfbackup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"app/internal/db"
	"app/internal/plans"
	"app/internal/storages"

	"github.com/gin-gonic/gin/binding"
)

const (
	StorageName = "Gogobackup itself (local)"
	PlanName    = "Gogobackup itself"
)

// Options is what a plan created by Ensure looks like. A plan that already exists keeps its own.
type Options struct {
	Database    string
	Dir         string
	Time        string
	MaxAgeDays  int
	KeepWeekly  int
	KeepMonthly int
}

// Result is the plan backing up the database, the storage Ensure used when it had to create
// that plan (nil when the plan was there already), and what this call created.
type Result struct {
	Plan           *plans.Plan
	Storage        *db.Storage
	PlanCreated    bool
	StorageCreated bool
}

// Ensure finds a plan by what it backs up, not by name, so renaming the plan in the panel does
// not make the next deploy create another. An existing plan is left exactly as it is. Without
// one, it reuses a local storage at Dir or creates it, then creates the plan; a run that
// stopped between the two finds the storage next time.
func Ensure(ctx context.Context, q *db.Queries, storageService *storages.StorageService, planService *plans.PlanService, opts Options) (*Result, error) {
	if _, ok := plans.ParseTimeOfDay(opts.Time); !ok {
		return nil, fmt.Errorf("time %q is not HH:MM", opts.Time)
	}
	database, err := filepath.Abs(opts.Database)
	if err != nil {
		return nil, err
	}
	dir, err := filepath.Abs(opts.Dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}

	existing, err := q.ListSQLitePlansByPath(ctx, database)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		plan, err := planService.GetPlan(ctx, existing[0].ID)
		if err != nil {
			return nil, err
		}
		return &Result{Plan: plan}, nil
	}

	result := &Result{}
	found, err := q.ListLocalStoragesByPath(ctx, dir)
	if err != nil {
		return nil, err
	}
	if len(found) > 0 {
		result.Storage = &found[0]
	} else {
		req := &storages.StorageRequest{Name: StorageName, Kind: storages.KindLocal, Local: &storages.LocalInput{Path: dir}}
		if err := binding.Validator.ValidateStruct(req); err != nil {
			return nil, err
		}
		if result.Storage, err = storageService.CreateStorage(ctx, req); err != nil {
			return nil, err
		}
		result.StorageCreated = true
	}

	req := &plans.PlanRequest{
		Name:          PlanName,
		SourceRequest: plans.SourceRequest{SourceKind: plans.SourceSQLite, SQLite: &plans.SQLiteInput{Path: database}},
		Schedules:     []plans.ScheduleInput{{Time: opts.Time}},
		MaxAgeDays:    opts.MaxAgeDays,
		KeepWeekly:    opts.KeepWeekly,
		KeepMonthly:   opts.KeepMonthly,
		StorageIDs:    []int64{result.Storage.ID},
		Enabled:       true,
	}
	if err := binding.Validator.ValidateStruct(req); err != nil {
		return nil, err
	}
	if result.Plan, err = planService.CreatePlan(ctx, req); err != nil {
		return nil, err
	}
	result.PlanCreated = true
	return result, nil
}
