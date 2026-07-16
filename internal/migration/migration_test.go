package migration

import (
	"strings"
	"testing"
	"testing/fstest"

	migrationfiles "go-mall/migrations"
)

func TestLoadFilesOrdersPairsAndBuildsChecksum(t *testing.T) {
	files := fstest.MapFS{
		"000002_second.up.sql":   {Data: []byte("ALTER TABLE demo ADD value BIGINT;")},
		"000001_first.down.sql":  {Data: []byte("DROP TABLE demo;")},
		"000001_first.up.sql":    {Data: []byte("CREATE TABLE demo (id BIGINT);")},
		"000002_second.down.sql": {Data: []byte("ALTER TABLE demo DROP COLUMN value;")},
	}
	migrations, err := LoadFiles(files)
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if len(migrations) != 2 || migrations[0].Version != 1 || migrations[1].Version != 2 {
		t.Fatalf("migrations were not ordered: %+v", migrations)
	}
	if len(migrations[0].Checksum) != 64 {
		t.Fatalf("unexpected checksum: %s", migrations[0].Checksum)
	}
}

func TestLoadFilesRequiresUpAndDownPair(t *testing.T) {
	_, err := LoadFiles(fstest.MapFS{"000001_first.up.sql": {Data: []byte("SELECT 1;")}})
	if err == nil || !strings.Contains(err.Error(), "up 和 down") {
		t.Fatalf("expected pair validation error, got %v", err)
	}
}

func TestValidateHistoryRejectsChangedMigration(t *testing.T) {
	local := []Migration{testMigration(1, "first")}
	applied := []AppliedMigration{{Version: 1, Name: "first", Checksum: strings.Repeat("0", 64)}}
	if err := ValidateHistory(local, applied); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("expected checksum error, got %v", err)
	}
}

func TestValidateHistoryRejectsDirtyAndNonPrefixHistory(t *testing.T) {
	local := []Migration{testMigration(1, "first"), testMigration(2, "second")}
	if err := ValidateHistory(local, []AppliedMigration{{Version: 1, Name: "first", Checksum: local[0].Checksum, Dirty: true}}); err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("expected dirty error, got %v", err)
	}
	if err := ValidateHistory(local, []AppliedMigration{{Version: 2, Name: "second", Checksum: local[1].Checksum}}); err == nil || !strings.Contains(err.Error(), "有序前缀") {
		t.Fatalf("expected prefix error, got %v", err)
	}
}

func TestPendingReturnsOnlyUnappliedSuffix(t *testing.T) {
	local := []Migration{
		testMigration(1, "first"),
		testMigration(2, "second"),
	}
	pending, err := Pending(local, []AppliedMigration{{Version: 1, Name: "first", Checksum: local[0].Checksum}})
	if err != nil {
		t.Fatalf("pending migrations: %v", err)
	}
	if len(pending) != 1 || pending[0].Version != 2 {
		t.Fatalf("unexpected pending migrations: %+v", pending)
	}
}

func TestEmbeddedMigrationsAreValidAndOrdered(t *testing.T) {
	items, err := LoadFiles(migrationfiles.Files)
	if err != nil {
		t.Fatalf("load embedded migrations: %v", err)
	}
	if err := ValidateMigrations(items); err != nil {
		t.Fatalf("validate embedded migrations: %v", err)
	}
	if len(items) != 12 || items[0].Version != 1 || items[len(items)-1].Version != 12 {
		t.Fatalf("unexpected embedded migration set: %+v", items)
	}
}

func TestValidateMigrationsRejectsUnorderedOrInvalidItems(t *testing.T) {
	first := testMigration(1, "first")
	second := testMigration(2, "second")
	if err := ValidateMigrations([]Migration{second, first}); err == nil || !strings.Contains(err.Error(), "严格递增") {
		t.Fatalf("expected ordering error, got %v", err)
	}
	first.Checksum = "invalid"
	if err := ValidateMigrations([]Migration{first}); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("expected checksum error, got %v", err)
	}
}

func testMigration(version int64, name string) Migration {
	files := fstest.MapFS{
		"000001_item.up.sql":   {Data: []byte("SELECT 1;")},
		"000001_item.down.sql": {Data: []byte("SELECT 2;")},
	}
	items, err := LoadFiles(files)
	if err != nil {
		panic(err)
	}
	item := items[0]
	item.Version = version
	item.Name = name
	return item
}
