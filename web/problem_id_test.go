package web

import "testing"

func TestParseProblemIDOnlyAcceptsNumericID(t *testing.T) {
	if _, err := parseProblemID("abc"); err == nil {
		t.Fatal("expected non-numeric problem id to be rejected")
	}

	if _, err := parseProblemID("0"); err == nil {
		t.Fatal("expected zero problem id to be rejected")
	}

	id, err := parseProblemID("7")
	if err != nil {
		t.Fatalf("expected numeric id to pass, got error: %v", err)
	}
	if id != 7 {
		t.Fatalf("expected id 7, got %d", id)
	}
}
