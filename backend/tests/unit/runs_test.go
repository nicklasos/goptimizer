package unit

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"app/internal/plans"
	"app/internal/runlog"
	"app/internal/runs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrevRun(t *testing.T) {
	raw, err := os.ReadFile("testdata/prev_run.json")
	require.NoError(t, err)
	var cases []struct {
		Name      string           `json:"name"`
		Schedules []plans.Schedule `json:"schedules"`
		At        time.Time        `json:"at"`
		Prev      *time.Time       `json:"prev"`
	}
	require.NoError(t, json.Unmarshal(raw, &cases))
	require.NotEmpty(t, cases)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			prev, ok := plans.PrevRun(tc.Schedules, tc.At)
			if tc.Prev == nil {
				assert.False(t, ok)
				return
			}
			require.True(t, ok)
			assert.Equal(t, tc.Prev.UTC(), prev)
		})
	}
}

func utc(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

func TestDue(t *testing.T) {
	daily := []plans.Schedule{{Time: 3 * 60, Days: []plans.Weekday{}}}
	created := utc("2026-10-01T00:00:00Z")
	slot := utc("2026-10-14T03:00:00Z")
	yesterday := utc("2026-10-13T03:00:00Z")

	cases := []struct {
		name      string
		schedules []plans.Schedule
		enabled   bool
		created   time.Time
		last      *time.Time
		now       time.Time
		want      *time.Time
	}{
		{"the slot that just passed", daily, true, created, &yesterday, utc("2026-10-14T03:00:30Z"), &slot},
		{"a slot at exactly now", daily, true, created, &yesterday, slot, &slot},
		{"never ran", daily, true, created, nil, utc("2026-10-14T03:00:30Z"), &slot},
		{"already enqueued", daily, true, created, &slot, utc("2026-10-14T03:01:00Z"), nil},
		{"a missed slot runs late, once", daily, true, created, &yesterday, utc("2026-10-14T05:00:00Z"), &slot},
		{"days of downtime run only the latest slot", daily, true, created, ptr(utc("2026-10-10T03:00:00Z")), utc("2026-10-14T05:00:00Z"), &slot},
		{"not before the next slot", daily, true, created, &slot, utc("2026-10-15T02:59:59Z"), nil},
		{"a slot before the plan existed", daily, true, utc("2026-10-14T04:00:00Z"), nil, utc("2026-10-14T05:00:00Z"), nil},
		{"a slot before the plan was re-enabled", daily, true, utc("2026-10-14T04:00:00Z"), &yesterday, utc("2026-10-14T05:00:00Z"), nil},
		{"disabled", daily, false, created, &yesterday, utc("2026-10-14T03:00:30Z"), nil},
		{"no schedules", []plans.Schedule{}, true, created, nil, utc("2026-10-14T03:00:30Z"), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := runs.Due(tc.schedules, tc.enabled, tc.created, tc.last, tc.now)
			if tc.want == nil {
				assert.False(t, ok, "got %v", got)
				return
			}
			require.True(t, ok)
			assert.Equal(t, *tc.want, got)
		})
	}
}

func ptr[T any](v T) *T { return &v }

// history is one archive at each of the times of day, every day from first to last.
func history(first, last time.Time, times ...time.Duration) []runs.Archive {
	var archives []runs.Archive
	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		for _, at := range times {
			archives = append(archives, runs.Archive{ID: int64(len(archives) + 1), CreatedAt: day.Add(at)})
		}
	}
	return archives
}

func archivesAt(times ...string) []runs.Archive {
	archives := make([]runs.Archive, len(times))
	for i, s := range times {
		archives[i] = runs.Archive{ID: int64(i + 1), CreatedAt: utc(s)}
	}
	return archives
}

// kept is the creation times of the archives Prune leaves, oldest first.
func kept(archives []runs.Archive, removed []int64) []string {
	var out []string
	for _, a := range archives {
		if !slices.Contains(removed, a.ID) {
			out = append(out, a.CreatedAt.Format("2006-01-02T15:04"))
		}
	}
	slices.Sort(out)
	return out
}

func TestPrune(t *testing.T) {
	// A Saturday. History starts 399 days earlier, on 2025-09-13.
	now := utc("2026-10-17T12:00:00Z")
	start := utc("2025-09-13T00:00:00Z")
	daily := history(start, utc("2026-10-17T00:00:00Z"), 3*time.Hour)
	twiceDaily := history(start, utc("2026-10-17T00:00:00Z"), 3*time.Hour, 15*time.Hour)
	twiceDaily = slices.DeleteFunc(twiceDaily, func(a runs.Archive) bool { return a.CreatedAt.After(now) })
	require.Len(t, daily, 400)

	monthStarts := func(from, to string) []string {
		var out []string
		for m := utc(from + "-01T03:00:00Z"); !m.After(utc(to + "-01T03:00:00Z")); m = m.AddDate(0, 1, 0) {
			out = append(out, m.Format("2006-01-02T15:04"))
		}
		return out
	}
	days := func(from, to string, at string) []string {
		var out []string
		for d := utc(from + "T" + at + ":00Z"); !d.After(utc(to + "T" + at + ":00Z")); d = d.AddDate(0, 0, 1) {
			out = append(out, d.Format("2006-01-02T15:04"))
		}
		return out
	}
	join := func(parts ...[]string) []string {
		var out []string
		for _, p := range parts {
			out = append(out, p...)
		}
		slices.Sort(out)
		return out
	}

	cases := []struct {
		name     string
		archives []runs.Archive
		policy   runs.Retention
		now      time.Time
		kept     []string
	}{
		{
			name:     "max age 0 keeps everything",
			archives: daily,
			policy:   runs.Retention{MaxAgeDays: 0, KeepWeekly: 4, KeepMonthly: 12},
			now:      now,
			kept:     kept(daily, nil),
		},
		{
			name:     "daily, 30 days, nothing more",
			archives: daily,
			policy:   runs.Retention{MaxAgeDays: 30},
			now:      now,
			kept:     days("2026-09-18", "2026-10-17", "03:00"),
		},
		{
			name:     "daily, 7 days and the Monday of each of the last 8 weeks",
			archives: daily,
			policy:   runs.Retention{MaxAgeDays: 7, KeepWeekly: 8},
			now:      now,
			kept: join(days("2026-10-11", "2026-10-17", "03:00"), []string{
				"2026-08-24T03:00", "2026-08-31T03:00", "2026-09-07T03:00", "2026-09-14T03:00",
				"2026-09-21T03:00", "2026-09-28T03:00", "2026-10-05T03:00",
			}),
		},
		{
			name:     "daily, 30 days and the first of each of the last 12 months, across a new year",
			archives: daily,
			policy:   runs.Retention{MaxAgeDays: 30, KeepMonthly: 12},
			now:      now,
			kept:     join(days("2026-09-18", "2026-10-17", "03:00"), monthStarts("2025-11", "2026-09")),
		},
		{
			name:     "twice daily, 14 days, 4 weeks and 6 months",
			archives: twiceDaily,
			policy:   runs.Retention{MaxAgeDays: 14, KeepWeekly: 4, KeepMonthly: 6},
			now:      now,
			kept: join(
				[]string{"2026-10-03T15:00"}, days("2026-10-04", "2026-10-16", "03:00"), days("2026-10-04", "2026-10-16", "15:00"),
				[]string{"2026-10-17T03:00"},
				[]string{"2026-09-21T03:00", "2026-09-28T03:00"},
				monthStarts("2026-05", "2026-10"),
			),
		},
		{
			name:     "an ISO week runs Monday to Sunday and across the new year",
			archives: archivesAt("2026-12-27T23:00:00Z", "2026-12-28T01:00:00Z", "2026-12-31T03:00:00Z", "2027-01-01T03:00:00Z", "2027-01-04T03:00:00Z"),
			policy:   runs.Retention{MaxAgeDays: 7, KeepWeekly: 52},
			now:      utc("2027-03-01T00:00:00Z"),
			kept:     []string{"2026-12-27T23:00", "2026-12-28T01:00", "2027-01-04T03:00"},
		},
		{
			name:     "a month starts on the 1st in UTC",
			archives: archivesAt("2026-12-27T23:00:00Z", "2026-12-28T01:00:00Z", "2026-12-31T23:59:00Z", "2027-01-01T00:00:00Z", "2027-01-04T03:00:00Z"),
			policy:   runs.Retention{MaxAgeDays: 7, KeepMonthly: 12},
			now:      utc("2027-03-01T00:00:00Z"),
			kept:     []string{"2026-12-27T23:00", "2027-01-01T00:00", "2027-01-04T03:00"},
		},
		{
			name:     "weeks and months before the window are not kept",
			archives: archivesAt("2026-01-05T03:00:00Z", "2026-06-01T03:00:00Z", "2026-10-16T03:00:00Z"),
			policy:   runs.Retention{MaxAgeDays: 7, KeepWeekly: 4, KeepMonthly: 3},
			now:      now,
			kept:     []string{"2026-10-16T03:00"},
		},
		{
			name:     "the newest archive stays when every archive is past max age",
			archives: archivesAt("2026-05-01T03:00:00Z", "2026-06-01T03:00:00Z"),
			policy:   runs.Retention{MaxAgeDays: 30},
			now:      now,
			kept:     []string{"2026-06-01T03:00"},
		},
		{
			name:     "a single old archive stays",
			archives: archivesAt("2026-01-01T03:00:00Z"),
			policy:   runs.Retention{MaxAgeDays: 1},
			now:      now,
			kept:     []string{"2026-01-01T03:00"},
		},
		{
			name:     "no archives",
			archives: nil,
			policy:   runs.Retention{MaxAgeDays: 1},
			now:      now,
			kept:     nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			removed := runs.Prune(tc.archives, tc.policy, tc.now)
			assert.Equal(t, tc.kept, kept(tc.archives, removed))
			assert.Empty(t, runs.Prune(filterKept(tc.archives, removed), tc.policy, tc.now), "a second prune removes nothing more")
		})
	}
}

func filterKept(archives []runs.Archive, removed []int64) []runs.Archive {
	return slices.DeleteFunc(slices.Clone(archives), func(a runs.Archive) bool { return slices.Contains(removed, a.ID) })
}

type sink struct {
	bytes.Buffer
}

func (s *sink) put(_ context.Context, r io.Reader) error {
	_, err := io.Copy(&s.Buffer, r)
	return err
}

func TestFanOut(t *testing.T) {
	data := make([]byte, 3<<20+17)
	_, err := rand.Read(data)
	require.NoError(t, err)
	sum := sha256.Sum256(data)

	var first, second sink
	refused := errors.New("bucket refused the upload")
	fan := runs.NewFanOut(context.Background(), []runs.Destination{
		{Name: "first", Put: first.put},
		{Name: "refuses", Put: func(context.Context, io.Reader) error { return refused }},
		{Name: "breaks halfway", Put: func(_ context.Context, r io.Reader) error {
			_, err := io.CopyN(io.Discard, r, 1<<20)
			if err != nil {
				return err
			}
			return errors.New("connection reset")
		}},
		{Name: "second", Put: second.put},
	})
	n, err := io.Copy(fan, bytes.NewReader(data))
	require.NoError(t, err, "one failing storage does not stop the others")
	uploads := fan.Close()

	assert.Equal(t, int64(len(data)), n)
	assert.Equal(t, int64(len(data)), fan.Size())
	assert.Equal(t, hex.EncodeToString(sum[:]), fan.SHA256())
	assert.Equal(t, data, first.Bytes())
	assert.Equal(t, data, second.Bytes())
	require.Len(t, uploads, 4)
	assert.Equal(t, runs.Upload{Name: "first"}, uploads[0])
	assert.ErrorIs(t, uploads[1].Err, refused)
	assert.EqualError(t, uploads[2].Err, "connection reset")
	assert.NoError(t, uploads[3].Err)
}

func TestFanOut_EveryDestinationFails(t *testing.T) {
	fan := runs.NewFanOut(context.Background(), []runs.Destination{
		{Name: "a", Put: func(context.Context, io.Reader) error { return errors.New("a is down") }},
		{Name: "b", Put: func(context.Context, io.Reader) error { return errors.New("b is down") }},
	})
	_, err := io.Copy(fan, bytes.NewReader(make([]byte, 1<<20)))
	assert.ErrorIs(t, err, runs.ErrNoDestination)
	for _, upload := range fan.Close() {
		assert.Error(t, upload.Err, upload.Name)
	}
}

func TestFanOut_AbortFailsEveryUpload(t *testing.T) {
	var stored sink
	fan := runs.NewFanOut(context.Background(), []runs.Destination{{Name: "a", Put: stored.put}})
	_, err := fan.Write([]byte("half a dump"))
	require.NoError(t, err)
	dumpErr := errors.New("pg_dump: connection lost")

	uploads := fan.Abort(dumpErr)

	require.Len(t, uploads, 1)
	assert.ErrorIs(t, uploads[0].Err, dumpErr, "the storage sees the dump fail and does not keep a partial archive")
}

func TestRunLog_KeepsTheLatestMegabyte(t *testing.T) {
	log := runlog.New()
	line := strings.Repeat("x", 99)
	for i := 0; i < 30000; i++ {
		log.Printf("%06d %s", i, line)
	}

	text := log.String()
	assert.LessOrEqual(t, len(text), runlog.MaxBytes+100)
	assert.Greater(t, len(text), runlog.MaxBytes-200)
	assert.True(t, strings.HasPrefix(text, "[earlier lines were dropped"), "says that lines were dropped")
	assert.Contains(t, log.Tail(1), "029999 ")
	assert.NotContains(t, text, " 000000 ")
	for _, l := range strings.Split(strings.TrimSpace(text), "\n")[1:] {
		require.Regexp(t, `^\d\d:\d\d:\d\d \d{6} x+$`, l, "every kept line is whole")
	}
}

func TestRunLog_Lines(t *testing.T) {
	log := runlog.New()
	log.Printf("starting %s", "plan")
	w := log.Writer("pg_dump: ")
	fmt.Fprint(w, "first li")
	fmt.Fprint(w, "ne\nsecond line\r\nunfinished")
	require.NoError(t, w.Close())
	log.Warnf("skipped %d files", 2)

	lines := strings.Split(strings.TrimSpace(log.String()), "\n")
	require.Len(t, lines, 5)
	for i, want := range []string{"starting plan", "pg_dump: first line", "pg_dump: second line", "pg_dump: unfinished", "warning: skipped 2 files"} {
		assert.Regexp(t, `^\d\d:\d\d:\d\d `, lines[i])
		assert.Equal(t, want, lines[i][9:])
	}
	assert.Equal(t, strings.Join(lines[3:], "\n"), log.Tail(2))

	_, before := log.Snapshot()
	log.Printf("one more")
	_, after := log.Snapshot()
	assert.NotEqual(t, before, after, "a write changes the version")
}
