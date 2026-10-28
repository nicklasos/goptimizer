package unit

import (
	"testing"

	"app/config"

	"github.com/stretchr/testify/assert"
)

func TestAssertTestDatabasePath(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"relative test database", "./data/gogobackup_test.db", false},
		{"absolute test database", "/var/lib/app/e2e_test.db", false},
		{"real database", "./data/gogobackup.db", true},
		{"suffix only in a directory name", "./data_test.db/gogobackup.db", true},
		{"missing extension", "./data/gogobackup_test", true},
		{"in-memory database", ":memory:", true},
		{"empty path", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := config.AssertTestDatabasePath(tc.path)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUseTestDatabase(t *testing.T) {
	cfg := &config.Config{DatabasePath: "./data/gogobackup.db", TestDatabasePath: "./data/gogobackup_test.db"}
	assert.NoError(t, cfg.UseTestDatabase())
	assert.Equal(t, "./data/gogobackup_test.db", cfg.DatabasePath)

	unsafe := &config.Config{DatabasePath: "./data/gogobackup.db", TestDatabasePath: "./data/gogobackup.db"}
	assert.Error(t, unsafe.UseTestDatabase())
	assert.Equal(t, "./data/gogobackup.db", unsafe.DatabasePath, "a refused path leaves the real one in place")
}
