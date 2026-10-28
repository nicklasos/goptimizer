// Command screenshots-seed fills a throwaway database with the storages, plans and 30 days of
// runs that the README screenshots show. scripts/screenshots/shoot.mjs runs it twice:
//
//	screenshots-seed -phase history -nas DIR -analytics FILE   before the API starts
//	screenshots-seed -phase live -nas DIR                     after the scheduler's first tick
//
// The live phase adds what the scheduler would otherwise act on within a minute: an overdue plan,
// which it would queue, and a running run, which a starting API fails as left over by a crash.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"time"

	"app/config"
	"app/internal/db"
	"app/internal/factory"
	"app/internal/middleware"
	"app/internal/plans"
	"app/internal/runs"
	"app/internal/secretbox"
	"app/internal/storages"
)

const (
	nas = "NAS (local)"
	b2  = "Backblaze B2"
	gcs = "GCS europe-west1"
	mb  = int64(1 << 20)
)

// demoPlan is one card on the dashboard and the runs behind it.
type demoPlan struct {
	name      string
	live      bool
	source    func(analytics string) factory.PlanOption
	schedules []plans.Schedule
	retention runs.Retention
	storages  []string
	paused    bool

	size   int64         // archive size 30 days ago
	growth int64         // added per day
	took   time.Duration // a typical run
	// failures are failed runs by slot, counted back from the newest slot (0).
	failures map[int]string
	// skipNewest leaves the newest slots without a run, as when the scheduler was not running.
	skipNewest int
	// stoppedDays leaves the last days without runs, as for a plan that was paused then.
	stoppedDays int
}

func daily(times ...string) []plans.Schedule {
	var schedules []plans.Schedule
	for _, t := range times {
		at, _ := plans.ParseTimeOfDay(t)
		schedules = append(schedules, plans.Schedule{Time: at, Days: []plans.Weekday{}})
	}
	return schedules
}

func weekdays(t string) []plans.Schedule {
	at, _ := plans.ParseTimeOfDay(t)
	return []plans.Schedule{{Time: at, Days: []plans.Weekday{plans.Monday, plans.Tuesday, plans.Wednesday, plans.Thursday, plans.Friday}}}
}

func postgres(host, user, database string) func(string) factory.PlanOption {
	return func(string) factory.PlanOption {
		return factory.PostgresSource(plans.PostgresConfig{Host: host, Port: 5432, User: user, Database: database, SSLMode: "disable"}, "demo-password")
	}
}

func mysql(host, user, database string) func(string) factory.PlanOption {
	return func(string) factory.PlanOption {
		return factory.MySQLSource(plans.MySQLConfig{Host: host, Port: 3306, User: user, Database: database}, "demo-password")
	}
}

var demoPlans = []demoPlan{
	{
		name:      "shop-prod Postgres",
		source:    postgres("10.0.4.12", "shop", "shop_prod"),
		schedules: daily("03:00", "15:00"),
		retention: runs.Retention{MaxAgeDays: 14, KeepWeekly: 8, KeepMonthly: 6},
		storages:  []string{nas, b2},
		size:      1210 * mb, growth: 4 * mb, took: 2*time.Minute + 50*time.Second,
		failures: map[int]string{
			4: `the dump failed: pg_dump: error: connection to server at "10.0.4.12", port 5432 failed: timeout expired`,
		},
	},
	{
		name:      "blog MySQL",
		source:    mysql("10.0.4.20", "blog", "blog"),
		schedules: daily("02:30"),
		retention: runs.Retention{MaxAgeDays: 30, KeepWeekly: 8},
		storages:  []string{nas, gcs},
		size:      84 * mb, growth: mb / 5, took: 24 * time.Second,
		failures: map[int]string{
			0:  "the dump failed: mysqldump: Got error: 1045: Access denied for user 'blog'@'10.0.4.5' (using password: YES) when trying to connect",
			11: "the dump failed: mysqldump: Error 2013: Lost connection to MySQL server during query when dumping table `wp_posts` at row: 48211",
		},
	},
	{
		name:      "analytics SQLite",
		source:    func(path string) factory.PlanOption { return factory.SQLiteSource(path) },
		schedules: daily("04:00"),
		retention: runs.Retention{MaxAgeDays: 30, KeepWeekly: 8, KeepMonthly: 12},
		storages:  []string{nas},
		size:      3 * mb, growth: mb / 20, took: 3 * time.Second,
	},
	{
		name: "uploads files",
		source: func(string) factory.PlanOption {
			return factory.FilesSourceExcluding([]string{"/var/www/shop/shared/uploads"}, []string{"*.tmp", "cache/"})
		},
		schedules: daily("01:00"),
		retention: runs.Retention{MaxAgeDays: 7, KeepWeekly: 4, KeepMonthly: 3},
		storages:  []string{nas, b2},
		size:      4600 * mb, growth: 25 * mb, took: 9 * time.Minute,
		failures: map[int]string{
			19: "the dump failed: open /var/www/shop/shared/uploads/2026/09: permission denied",
		},
	},
	{
		name:      "legacy-forum MySQL",
		source:    mysql("10.0.4.20", "forum", "forum"),
		schedules: daily("01:30"),
		retention: runs.Retention{MaxAgeDays: 60},
		storages:  []string{nas},
		paused:    true,
		size:      310 * mb, growth: 0, took: 41 * time.Second,
		stoppedDays: 9,
	},
	{
		name:      "crm-staging Postgres",
		live:      true,
		source:    postgres("10.0.4.31", "crm", "crm_staging"),
		schedules: weekdays("06:00"),
		retention: runs.Retention{MaxAgeDays: 14},
		storages:  []string{nas},
		size:      236 * mb, growth: mb, took: 38 * time.Second,
		skipNewest: 2,
	},
}

type reporter struct{}

func (reporter) Helper() {}
func (reporter) Fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "screenshots-seed: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	phase := flag.String("phase", "", "history (before the API starts) or live (after it)")
	nasDir := flag.String("nas", "", "directory of the local storage")
	analytics := flag.String("analytics", "", "SQLite file the analytics plan backs up")
	flag.Parse()
	if (*phase != "history" && *phase != "live") || *nasDir == "" {
		reporter{}.Fatalf("usage: -phase history|live -nas DIR [-analytics FILE]")
	}

	cfg, err := config.Load()
	if err != nil {
		reporter{}.Fatalf("%v", err)
	}
	conn, err := db.Open(cfg.DatabasePath)
	if err != nil {
		reporter{}.Fatalf("%v", err)
	}
	defer conn.Close()
	secrets, err := secretbox.New(cfg.EncryptionKey)
	if err != nil {
		reporter{}.Fatalf("%v", err)
	}
	s := &seeder{
		create:  factory.New(reporter{}, conn, secrets),
		queries: db.New(conn),
		nas:     *nasDir,
		now:     time.Now().UTC(),
		rand:    rand.New(rand.NewPCG(7, 11)),
	}

	if *phase == "history" {
		s.create.User(factory.WithEmail("admin@example.com"), factory.WithName("Admin"), factory.WithRoles(middleware.RoleSuperAdmin))
	}
	s.storages()
	for _, p := range demoPlans {
		if p.live == (*phase == "live") {
			s.plan(p, *analytics)
		}
	}
	if *phase == "live" {
		s.running("uploads files", 3*time.Minute)
	}
}

type seeder struct {
	create  *factory.Factory
	queries *db.Queries
	nas     string
	now     time.Time
	rand    *rand.Rand
	byName  map[string]*db.Storage
}

// storages creates the three storages, or finds them when the history phase already did.
func (s *seeder) storages() {
	s.byName = map[string]*db.Storage{}
	existing, err := s.queries.ListStoragesPaginated(context.Background(), db.ListStoragesPaginatedParams{Limit: 100})
	if err != nil {
		reporter{}.Fatalf("list storages: %v", err)
	}
	for i := range existing {
		s.byName[existing[i].Name] = &existing[i]
	}
	if len(s.byName) > 0 {
		return
	}
	if err := os.MkdirAll(s.nas, 0o750); err != nil {
		reporter{}.Fatalf("%v", err)
	}
	s.byName[nas] = s.create.Storage(factory.WithStorageName(nas), factory.LocalStorage(s.nas))
	s.byName[b2] = s.create.Storage(factory.WithStorageName(b2), factory.S3Storage(storages.S3Config{
		Endpoint: "https://s3.eu-central-003.backblazeb2.com", Region: "eu-central-003", Bucket: "acme-backups", Prefix: "servers/web-1/",
	}, "003a8f1c2e9d4b70000000004", "K003demoSecretNotARealKey"))
	s.byName[gcs] = s.create.Storage(factory.WithStorageName(gcs), factory.GCSStorage(storages.GCSConfig{
		Bucket: "acme-backups-ew1", Prefix: "web-1/",
	}, `{"type":"service_account","project_id":"acme-backups","client_email":"gogobackup@acme-backups.iam.gserviceaccount.com"}`))
}

// slots are the plan's scheduled times over the last 30 days, oldest first.
func (s *seeder) slots(p demoPlan) []time.Time {
	var slots []time.Time
	from := s.now.AddDate(0, 0, -30)
	for at := s.now; ; {
		slot, ok := plans.PrevRun(p.schedules, at)
		if !ok || slot.Before(from) {
			break
		}
		slots = append([]time.Time{slot}, slots...)
		at = slot.Add(-time.Second)
	}
	slots = slots[:len(slots)-p.skipNewest]
	cut := s.now.AddDate(0, 0, -p.stoppedDays)
	for len(slots) > 0 && p.stoppedDays > 0 && slots[len(slots)-1].After(cut) {
		slots = slots[:len(slots)-1]
	}
	return slots
}

// jitter is d give or take a tenth.
func (s *seeder) jitter(d time.Duration) time.Duration {
	return d + time.Duration((s.rand.Float64()-0.5)*0.2*float64(d))
}

type stored struct {
	run     *db.Run
	started time.Time
	size    int64
}

func (s *seeder) plan(p demoPlan, analytics string) {
	var targets []*db.Storage
	for _, name := range p.storages {
		targets = append(targets, s.byName[name])
	}
	opts := []factory.PlanOption{
		factory.WithPlanName(p.name),
		p.source(analytics),
		factory.WithSchedules(p.schedules...),
		factory.WithRetention(p.retention.MaxAgeDays, p.retention.KeepWeekly, p.retention.KeepMonthly),
		factory.WithStorages(targets...),
		factory.CreatedAt(s.now.AddDate(0, 0, -31)),
	}
	if p.paused {
		opts = append(opts, factory.Disabled())
	}
	plan := s.create.Plan(opts...)

	slots := s.slots(p)
	var kept []stored
	for i, slot := range slots {
		started := slot.Add(time.Duration(1+s.rand.IntN(4)) * time.Second)
		if failure, failed := p.failures[len(slots)-1-i]; failed {
			s.create.Run(plan, factory.ScheduledFor(slot), factory.StartedAt(started), factory.WithRunStatus(runs.StatusFailed),
				factory.FinishedAfter(s.jitter(p.took/4)), factory.RunError(failure))
			continue
		}
		days := started.Sub(slots[0]).Hours() / 24
		size := p.size + int64(days*float64(p.growth)) + s.rand.Int64N(p.size/200+1)
		run := s.create.Run(plan, factory.ScheduledFor(slot), factory.StartedAt(started), factory.WithRunStatus(runs.StatusSucceeded),
			factory.FinishedAfter(s.jitter(p.took)), factory.RunSize(size))
		kept = append(kept, stored{run: run, started: started, size: size})
	}

	retained := make([]runs.Archive, len(kept))
	for i, k := range kept {
		retained[i] = runs.Archive{ID: int64(i), CreatedAt: k.started}
	}
	pruned := map[int64]bool{}
	for _, i := range runs.Prune(retained, p.retention, s.now) {
		pruned[i] = true
	}
	for i, k := range kept {
		key := runs.ObjectKey(p.name, k.run.ID, k.started, plans.ArchiveExtension(plans.SourceKind(plan.SourceKind)))
		for _, storage := range targets {
			opts := []factory.ArchiveOption{factory.ArchiveKey(key), factory.ArchiveContent([]byte(storage.Name + key)), factory.ArchiveSize(k.size), factory.ArchiveCreatedAt(k.started)}
			if pruned[int64(i)] {
				opts = append(opts, factory.ArchiveDeleted())
			}
			s.create.Archive(k.run, storage, opts...)
			if storage.Name == nas && !pruned[int64(i)] {
				s.placeholder(key, k.size, k.started)
			}
		}
	}
}

// placeholder is a sparse file of the archive's size, so the storage browser lists the archives
// the history recorded without them taking up the disk.
func (s *seeder) placeholder(key string, size int64, written time.Time) {
	path := filepath.Join(s.nas, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		reporter{}.Fatalf("%v", err)
	}
	f, err := os.Create(path)
	if err == nil {
		err = errors.Join(f.Truncate(size), f.Close())
	}
	if err == nil {
		err = os.Chtimes(path, written, written)
	}
	if err != nil {
		reporter{}.Fatalf("placeholder %s: %v", key, err)
	}
}

// running starts a manual run of the plan that has been going for a while.
func (s *seeder) running(name string, since time.Duration) {
	all, err := s.queries.ListPlans(context.Background())
	if err != nil {
		reporter{}.Fatalf("list plans: %v", err)
	}
	for i := range all {
		if all[i].Name == name {
			s.create.Run(&all[i], factory.WithRunStatus(runs.StatusRunning), factory.StartedAt(s.now.Add(-since)))
			return
		}
	}
	reporter{}.Fatalf("no plan %q", name)
}
