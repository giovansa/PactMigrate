package pactmigrate

import (
	"testing"
	"testing/fstest"
)

func TestLoad_SortAndParse(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"202401021200_bbbbbbbb_second.sql":       &fstest.MapFile{Data: []byte("SELECT 2;")},
		"202401011200_aaaaaaaa_first.sql":        &fstest.MapFile{Data: []byte("SELECT 1;")},
		"nested/202401031200_cccccccc_third.sql": &fstest.MapFile{Data: []byte("SELECT 3;")},
	}

	ms, err := Load(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 3 {
		t.Fatalf("got %d migrations", len(ms))
	}
	if ms[0].Title != "first" || ms[1].Title != "second" || ms[2].Title != "third" {
		t.Fatalf("order wrong: %#v", ms)
	}
}

func TestLoad_InvalidNameStopsWalk(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"202401011200_bad_short_hash_x.sql": &fstest.MapFile{Data: []byte("SELECT 1;")},
	}
	_, err := Load(fsys, ".")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_EmptyFS(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{}
	ms, err := Load(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 0 {
		t.Fatalf("got %d", len(ms))
	}
}

func TestLoad_SkipNonSQL(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"readme.txt":                   &fstest.MapFile{Data: []byte("x")},
		"202401011200_a1b2c3d4_ok.sql": &fstest.MapFile{Data: []byte("SELECT 1;")},
	}
	ms, err := Load(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 || ms[0].Title != "ok" {
		t.Fatalf("got %#v", ms)
	}
}

func TestLoad_SubdirRoot(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"migrations/202401011200_a1b2c3d4_x.sql": &fstest.MapFile{Data: []byte("SELECT 1;")},
	}
	ms, err := Load(fsys, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 || ms[0].Filename != "migrations/202401011200_a1b2c3d4_x.sql" {
		t.Fatalf("got %#v", ms)
	}
}

func TestLoad_MissingRoot(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{}
	_, err := Load(fsys, "does_not_exist")
	if err == nil {
		t.Fatal("expected error")
	}
}
