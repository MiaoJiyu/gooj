package web

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/minicago/gooj/sql_service"
)

func sortUsersByRating(users []sql_service.User) []sql_service.User {
	sorted := append([]sql_service.User(nil), users...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Rating != sorted[j].Rating {
			return sorted[i].Rating > sorted[j].Rating
		}
		return sorted[i].Username < sorted[j].Username
	})
	return sorted
}

func GetRatingLeaderboardHandler(w http.ResponseWriter, r *http.Request) {
	db := sql_service.DB()
	if db == nil {
		http.Error(w, "database not initialized", http.StatusInternalServerError)
		return
	}

	var users []sql_service.User
	// Only select fields needed for ranking (username, rating and group_name)
	if err := db.Select("username, rating, group_name").Order("rating desc, username asc").Find(&users).Error; err != nil {
		http.Error(w, "failed to load ranking", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"rows":  sortUsersByRating(users),
		"total": len(users),
	})
}
