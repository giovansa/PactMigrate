package pactmigrate

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestMigration_VerifyContent_ShortHash(t *testing.T) {
	t.Parallel()

	sql := []byte("CREATE TABLE t (id INT);")
	sum := sha256.Sum256(sql)
	short := hex.EncodeToString(sum[:])[:8]

	m := Migration{
		Filename:    "x.sql",
		ContentHash: short,
		SQL:         sql,
	}
	if err := m.VerifyContent(); err != nil {
		t.Fatal(err)
	}
}

func TestMigration_VerifyContent_FullHash(t *testing.T) {
	t.Parallel()

	sql := []byte("hello")
	sum := sha256.Sum256(sql)
	full := hex.EncodeToString(sum[:])

	m := Migration{
		Filename:    "x.sql",
		ContentHash: full,
		SQL:         sql,
	}
	if err := m.VerifyContent(); err != nil {
		t.Fatal(err)
	}
}

func TestMigration_VerifyContent_Mismatch(t *testing.T) {
	t.Parallel()

	m := Migration{
		Filename:    "x.sql",
		ContentHash: "00000000",
		SQL:         []byte("not matching"),
	}
	if err := m.VerifyContent(); err == nil {
		t.Fatal("expected error")
	}
}

func TestMigration_Key(t *testing.T) {
	t.Parallel()

	m := Migration{Timestamp: "1", ContentHash: "a", Title: "b"}
	if m.Key() != "1_a_b" {
		t.Fatalf("got %q", m.Key())
	}
}

func TestMigration_VerifyContent_HashTooShort(t *testing.T) {
	t.Parallel()

	m := Migration{
		Filename:    "x.sql",
		ContentHash: "abcd",
		SQL:         []byte("x"),
	}
	if err := m.VerifyContent(); err == nil || !strings.Contains(err.Error(), "too short") {
		t.Fatalf("got %v", err)
	}
}
