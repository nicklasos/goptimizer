package factory

import (
	"encoding/json"
	"os"
	"time"

	"app/internal/db"
	"app/internal/plans"
)

type planAttrs struct {
	name        string
	kind        plans.SourceKind
	config      any
	secret      any
	schedules   []plans.Schedule
	maxAgeDays  int64
	keepWeekly  int64
	keepMonthly int64
	storageIDs  []int64
	enabled     bool
	createdAt   *time.Time
}

type PlanOption func(*planAttrs)

func WithPlanName(name string) PlanOption { return func(a *planAttrs) { a.name = name } }

// FilesSource backs up paths on the server, the default kind (the system temp directory).
func FilesSource(paths ...string) PlanOption {
	return func(a *planAttrs) {
		a.kind, a.config, a.secret = plans.SourceFiles, plans.FilesConfig{Paths: paths, Excludes: []string{}}, nil
	}
}

// FilesSourceExcluding backs up paths, leaving out what matches a pattern.
func FilesSourceExcluding(paths, excludes []string) PlanOption {
	return func(a *planAttrs) {
		a.kind, a.config, a.secret = plans.SourceFiles, plans.FilesConfig{Paths: paths, Excludes: excludes}, nil
	}
}

func SQLiteSource(path string) PlanOption {
	return func(a *planAttrs) {
		a.kind, a.config, a.secret = plans.SourceSQLite, plans.SQLiteConfig{Path: path}, nil
	}
}

func PostgresSource(config plans.PostgresConfig, password string) PlanOption {
	return func(a *planAttrs) {
		a.kind, a.config, a.secret = plans.SourcePostgres, config, plans.DatabaseSecret{Password: password}
	}
}

func MySQLSource(config plans.MySQLConfig, password string) PlanOption {
	return func(a *planAttrs) {
		a.kind, a.config, a.secret = plans.SourceMySQL, config, plans.DatabaseSecret{Password: password}
	}
}

// WithSchedules replaces the default 03:00 every day; none makes a plan that only runs by hand.
func WithSchedules(schedules ...plans.Schedule) PlanOption {
	return func(a *planAttrs) { a.schedules = schedules }
}

func WithRetention(maxAgeDays, keepWeekly, keepMonthly int64) PlanOption {
	return func(a *planAttrs) { a.maxAgeDays, a.keepWeekly, a.keepMonthly = maxAgeDays, keepWeekly, keepMonthly }
}

// WithStorages sets where the plan uploads. Without it the factory creates a local storage.
func WithStorages(storages ...*db.Storage) PlanOption {
	return func(a *planAttrs) {
		a.storageIDs = nil
		for _, storage := range storages {
			a.storageIDs = append(a.storageIDs, storage.ID)
		}
	}
}

// CreatedAt backdates the plan, its creation and its last save. The scheduler runs no slot from
// before the plan was last saved.
func CreatedAt(at time.Time) PlanOption {
	return func(a *planAttrs) { utc := at.UTC(); a.createdAt = &utc }
}

func Disabled() PlanOption { return func(a *planAttrs) { a.enabled = false } }

// Plan creates a backup plan: the temp directory as files, 03:00 every day, kept 30 days, to a
// new local storage, unless options say otherwise. A password is sealed with the factory's key.
func (f *Factory) Plan(opts ...PlanOption) *db.Plan {
	f.t.Helper()

	attrs := planAttrs{
		name:       "Plan " + unique(),
		schedules:  []plans.Schedule{{Time: 3 * 60, Days: []plans.Weekday{}}},
		maxAgeDays: 30,
		enabled:    true,
	}
	FilesSource(os.TempDir())(&attrs)
	for _, opt := range opts {
		opt(&attrs)
	}
	if attrs.storageIDs == nil {
		attrs.storageIDs = []int64{f.Storage().ID}
	}
	if attrs.schedules == nil {
		attrs.schedules = []plans.Schedule{}
	}

	config, err := json.Marshal(attrs.config)
	if err != nil {
		f.t.Fatalf("factory.Plan: %v", err)
	}
	schedules, err := json.Marshal(attrs.schedules)
	if err != nil {
		f.t.Fatalf("factory.Plan: %v", err)
	}
	var secret []byte
	if attrs.secret != nil {
		plain, err := json.Marshal(attrs.secret)
		if err != nil {
			f.t.Fatalf("factory.Plan: %v", err)
		}
		secret = f.secrets.Seal(plain)
	}

	plan, err := f.queries.FactoryCreatePlan(ctx, db.FactoryCreatePlanParams{
		Name:         attrs.name,
		SourceKind:   string(attrs.kind),
		SourceConfig: string(config),
		SourceSecret: secret,
		Schedules:    string(schedules),
		MaxAgeDays:   attrs.maxAgeDays,
		KeepWeekly:   attrs.keepWeekly,
		KeepMonthly:  attrs.keepMonthly,
		Enabled:      attrs.enabled,
		CreatedAt:    attrs.createdAt,
	})
	if err != nil {
		f.t.Fatalf("factory.Plan: %v", err)
	}
	for _, storageID := range attrs.storageIDs {
		_, err := f.queries.FactoryLinkPlanStorage(ctx, db.FactoryLinkPlanStorageParams{PlanID: plan.ID, StorageID: storageID})
		if err != nil {
			f.t.Fatalf("factory.Plan: %v", err)
		}
	}
	return &plan
}
