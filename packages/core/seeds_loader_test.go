package pactmigrate

import (
	"testing"
	"testing/fstest"
)

func TestLoadStaticSeedInventory(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"seeds/required/core/roles.seed.json": &fstest.MapFile{Data: []byte(`{
  "version": 1,
  "kind": "required",
  "id": "core.roles",
  "table": "roles",
  "identity": { "columns": ["code"] },
  "delete_policy": "ignore",
  "rows": [{"code":"admin","name":"Administrator"}]
}`)},
	}

	got, err := LoadStaticSeedInventory(fsys, "seeds")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries", len(got))
	}
	if got[0].ID != "core.roles" || got[0].RowCount != 1 {
		t.Fatalf("unexpected entry: %#v", got[0])
	}
	if len(got[0].ValidationIssues) != 0 {
		t.Fatalf("unexpected validation issues: %#v", got[0].ValidationIssues)
	}
	if got[0].IdentityColumns == nil || got[0].SupportedEnvironments == nil {
		t.Fatalf("expected normalized slices, got %#v", got[0])
	}
}

func TestLoadStaticSeedInventory_InvalidFileStillReturned(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"seeds/required/core/bad.seed.json": &fstest.MapFile{Data: []byte(`{
  "version": 1,
  "kind": "required",
  "table": "roles",
  "identity": { "columns": ["code"] },
  "rows": [{"name":"missing code"}]
}`)},
	}

	got, err := LoadStaticSeedInventory(fsys, "seeds")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries", len(got))
	}
	if len(got[0].ValidationIssues) == 0 {
		t.Fatalf("expected validation issues, got %#v", got[0])
	}
	if got[0].ValidationIssues[0].Code == "" || got[0].ValidationIssues[0].Message == "" {
		t.Fatalf("expected structured issue, got %#v", got[0].ValidationIssues)
	}
}

func TestLoadStaticSeedInventory_UnknownFieldReported(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"seeds/required/core/bad.seed.json": &fstest.MapFile{Data: []byte(`{
  "version": 1,
  "kind": "required",
  "id": "core.roles",
  "table": "roles",
  "identity": { "columns": ["code"] },
  "rows": [],
  "extra": true
}`)},
	}

	got, err := LoadStaticSeedInventory(fsys, "seeds")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].ValidationIssues) == 0 {
		t.Fatalf("expected schema issue, got %#v", got)
	}
}

func TestLoadStaticSeedInventory_DuplicateSeedID(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"seeds/required/core/a.seed.json": &fstest.MapFile{Data: []byte(`{
  "version": 1,
  "kind": "required",
  "id": "core.roles",
  "table": "roles",
  "identity": { "columns": ["code"] },
  "rows": [{"code":"admin"}]
}`)},
		"seeds/required/core/b.seed.json": &fstest.MapFile{Data: []byte(`{
  "version": 1,
  "kind": "required",
  "id": "core.roles",
  "table": "roles_backup",
  "identity": { "columns": ["code"] },
  "rows": [{"code":"viewer"}]
}`)},
	}

	got, err := LoadStaticSeedInventory(fsys, "seeds")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d entries", len(got))
	}
	if len(got[0].ValidationIssues) == 0 || len(got[1].ValidationIssues) == 0 {
		t.Fatalf("expected duplicate id issues, got %#v", got)
	}
}

func TestLoadStaticSeedInventoryByID(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"seeds/required/core/roles.seed.json": &fstest.MapFile{Data: []byte(`{
  "version": 1,
  "kind": "required",
  "id": "core.roles",
  "table": "roles",
  "identity": { "columns": ["code"] },
  "rows": [{"code":"admin"}]
}`)},
	}

	got, err := LoadStaticSeedInventoryByID(fsys, "seeds", "core.roles")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != "core.roles" {
		t.Fatalf("unexpected result: %#v", got)
	}
}
