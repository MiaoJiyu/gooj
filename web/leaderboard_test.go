package web

import (
	"testing"

	"github.com/minicago/gooj/sql_service"
)

func TestSortUsersForRatingLeaderboard(t *testing.T) {
	users := []sql_service.User{
		{Username: "bob", Rating: 1500},
		{Username: "alice", Rating: 1700},
		{Username: "charlie", Rating: 1700},
		{Username: "dave", Rating: 1600},
	}

	sorted := sortUsersByRating(users)
	if len(sorted) != 4 {
		t.Fatalf("expected 4 users, got %d", len(sorted))
	}
	if sorted[0].Username != "alice" || sorted[1].Username != "charlie" {
		t.Fatalf("expected highest rating users first, got %q, %q", sorted[0].Username, sorted[1].Username)
	}
	if sorted[2].Username != "dave" || sorted[3].Username != "bob" {
		t.Fatalf("expected remaining users ordered by rating desc, got %q, %q", sorted[2].Username, sorted[3].Username)
	}
}
