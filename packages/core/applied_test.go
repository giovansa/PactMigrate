package pactmigrate

import "testing"

func TestTimestampFromMigrationKey(t *testing.T) {
	t.Parallel()
	key := "202401011200_a1b2c3d4_add_users_table"
	if got := timestampFromMigrationKey(key); got != "202401011200" {
		t.Fatalf("got %q", got)
	}
}

func TestMaxAppliedTimestamp(t *testing.T) {
	t.Parallel()
	applied := map[string]AppliedRecord{
		"202401021200_aaa_x": {},
		"202401011200_bbb_y": {},
	}
	if max := maxAppliedTimestamp(applied); max != "202401021200" {
		t.Fatalf("got %q", max)
	}
}
