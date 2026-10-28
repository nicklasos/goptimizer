package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// testDBFileSuffix is the mandatory ending of any database file the test tooling may touch,
// so a TEST_DATABASE_PATH set by hand or inherited from a shell cannot point at real data.
const testDBFileSuffix = "_test.db"

// AssertTestDatabasePath rejects a path that does not clearly name a throwaway test database.
// The E2E suite creates and deletes data unconditionally, so pointing it at the real
// database is destructive rather than merely wrong.
func AssertTestDatabasePath(path string) error {
	name := filepath.Base(path)
	if !strings.HasSuffix(name, testDBFileSuffix) {
		return fmt.Errorf(
			"refusing to use database %q: TEST_DATABASE_PATH must name a file ending in %q",
			path, testDBFileSuffix,
		)
	}
	return nil
}

// UseTestDatabase points the config at TEST_DATABASE_PATH after checking it is safe.
func (c *Config) UseTestDatabase() error {
	if c.TestDatabasePath == "" {
		return fmt.Errorf("TEST_DATABASE_PATH environment variable is required")
	}
	if err := AssertTestDatabasePath(c.TestDatabasePath); err != nil {
		return err
	}
	c.DatabasePath = c.TestDatabasePath
	return nil
}
