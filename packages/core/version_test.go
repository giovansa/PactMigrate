package pactmigrate

import (
	"strings"
	"testing"
)

func TestParseFilename(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		input      string
		wantTS     string
		wantHash   string
		wantTitle  string
		wantErr    bool
		errContain string
	}{
		{
			name:      "example from spec",
			input:     "202403221500_a1b2c3d4_add_users_table.sql",
			wantTS:    "202403221500",
			wantHash:  "a1b2c3d4",
			wantTitle: "add_users_table",
		},
		{
			name:      "title with underscores",
			input:     "202401011200_deadbeef12345678_add_users_and_roles.sql",
			wantTS:    "202401011200",
			wantHash:  "deadbeef12345678",
			wantTitle: "add_users_and_roles",
		},
		{
			name:      "full sha256 hash",
			input:     "20240322150000_" + strings.Repeat("a", 64) + "_init.sql",
			wantTS:    "20240322150000",
			wantHash:  strings.Repeat("a", 64),
			wantTitle: "init",
		},
		{
			name:      "path with directory",
			input:     "db/migrations/202403221500_a1b2c3d4_add_users_table.sql",
			wantTS:    "202403221500",
			wantHash:  "a1b2c3d4",
			wantTitle: "add_users_table",
		},
		{
			name:       "wrong extension",
			input:      "202403221500_a1b2c3d4_add_users.txt",
			wantErr:    true,
			errContain: ".sql",
		},
		{
			name:       "missing underscores",
			input:      "202403221500.sql",
			wantErr:    true,
			errContain: "underscores",
		},
		{
			name:       "only two segments",
			input:      "202403221500_hash.sql",
			wantErr:    true,
			errContain: "underscores",
		},
		{
			name:       "bad timestamp letters",
			input:      "20x403221500_a1b2c3d4_add.sql",
			wantErr:    true,
			errContain: "timestamp",
		},
		{
			name:       "hash too short",
			input:      "202403221500_a1b2c3_add.sql",
			wantErr:    true,
			errContain: "8-64",
		},
		{
			name:       "hash not hex",
			input:      "202403221500_g1b2c3d4_add.sql",
			wantErr:    true,
			errContain: "content hash",
		},
		{
			name:       "empty title",
			input:      "202403221500_a1b2c3d4_.sql",
			wantErr:    true,
			errContain: "title",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ts, h, title, err := ParseFilename(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errContain != "" && !strings.Contains(err.Error(), tc.errContain) {
					t.Fatalf("error %v should contain %q", err, tc.errContain)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ts != tc.wantTS || h != tc.wantHash || title != tc.wantTitle {
				t.Fatalf("got ts=%q hash=%q title=%q want ts=%q hash=%q title=%q", ts, h, title, tc.wantTS, tc.wantHash, tc.wantTitle)
			}
		})
	}
}

func TestParseFilename_HashTooLong(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 65)
	_, _, _, err := ParseFilename("202403221500_" + long + "_x.sql")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "8-64") {
		t.Fatalf("got %v", err)
	}
}
