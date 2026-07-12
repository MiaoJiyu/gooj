package manage

import "testing"

func TestParseUserCSVImport(t *testing.T) {
	rows, err := ParseUserCSVImport([]byte("username,group,password\nalice,super,pass123\nbob,student,pass456\n"))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Username != "alice" || rows[0].GroupName != "super" || rows[0].Password != "pass123" {
		t.Fatalf("unexpected first row: %+v", rows[0])
	}
}
