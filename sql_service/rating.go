package sql_service

import (
	"errors"
	"log"
	"math"
	"sort"
	"time"

	"gorm.io/gorm"
)

const (
	// K-factor determines rating volatility (higher = more change per game)
	DefaultKFactor = 32
	// Initial rating for new users
	InitialRating = 1500
)

// CalculateExpectedScore returns expected score based on Elo formula
func CalculateExpectedScore(ratingA, ratingB int) float64 {
	return 1.0 / (1.0 + math.Pow(10, float64(ratingB-ratingA)/400))
}

// CalculateNewRating calculates new rating after a contest
func CalculateNewRating(currentRating, opponentAvgRating int, score float64, kFactor int) int {
	expected := CalculateExpectedScore(currentRating, opponentAvgRating)
	newRating := currentRating + int(float64(kFactor)*(score-expected))
	return newRating
}

// CalculateContestRating calculates and stores rating changes for all participants in a contest
// This function uses a transaction to ensure rating settlement only happens once, even if steps fail
func CalculateContestRating(contestID uint) error {
	if db == nil {
		return errors.New("db not initialized")
	}

	// Get contest details
	contest, err := GetContestByID(contestID)
	if err != nil {
		return err
	}

	// Check if contest has ended
	if time.Now().Before(contest.EndAt) {
		return errors.New("contest has not ended yet")
	}

	// Check if rating has already been settled (using the flag for atomic check-and-set)
	if contest.RatingSettled {
		return errors.New("rating already settled for this contest")
	}

	// Atomically mark as settled to prevent concurrent or retry attempts
	// Use raw SQL update to ensure it's atomic even if transaction fails later
	if err := db.Model(&Contest{}).Where("id = ? AND rating_settled = ?", contestID, false).Update("rating_settled", true).Error; err != nil {
		return errors.New("failed to mark contest as settled")
	}

	// Now proceed with the actual calculation (wrapped in transaction)
	// If anything fails from here, rating_settled stays true so no retry
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get leaderboard with scores
	leaderboard, err := GetContestLeaderboard(contestID)
	if err != nil {
		tx.Rollback()
		return err
	}

	if len(leaderboard) == 0 {
		tx.Rollback()
		return errors.New("no participants in contest")
	}

	// Calculate average rating of all participants
	totalRating := 0
	for _, row := range leaderboard {
		totalRating += row.Rating
	}
	avgRating := totalRating / len(leaderboard)

	// Calculate ranks considering ties (same total score = same rank)
	// Sort by total descending, username ascending for stable ordering
	sortedLeaderboard := make([]ContestRankingRow, len(leaderboard))
	copy(sortedLeaderboard, leaderboard)
	sort.Slice(sortedLeaderboard, func(i, j int) bool {
		if sortedLeaderboard[i].Total != sortedLeaderboard[j].Total {
			return sortedLeaderboard[i].Total > sortedLeaderboard[j].Total
		}
		return sortedLeaderboard[i].Username < sortedLeaderboard[j].Username
	})

	// Assign ranks with tie handling
	rankings := make(map[string]int)
	for i := range sortedLeaderboard {
		username := sortedLeaderboard[i].Username
		if i == 0 {
			rankings[username] = 1
		} else {
			// Check if tied with previous
			if sortedLeaderboard[i].Total == sortedLeaderboard[i-1].Total {
				rankings[username] = rankings[sortedLeaderboard[i-1].Username]
			} else {
				rankings[username] = i + 1
			}
		}
	}

	// Calculate and store rating changes
	histories := make([]ContestRatingHistory, 0, len(leaderboard))
	for _, row := range leaderboard {
		username := row.Username

		// Get current user rating
		var user User
		if err := tx.Where("username = ?", username).First(&user).Error; err != nil {
			continue
		}

		ratingBefore := user.Rating
		rank := rankings[username]

		// Calculate score ratio (0 to 1) based on rank
		// Rank 1 gets 1.0, last place gets 0.0, linear interpolation
		scoreRatio := 1.0 - (float64(rank-1) / float64(len(leaderboard)-1))

		// Calculate new rating using Elo-based formula
		ratingAfter := CalculateNewRating(ratingBefore, avgRating, scoreRatio, DefaultKFactor)

		history := ContestRatingHistory{
			Username:     username,
			ContestID:    contestID,
			ContestName:  contest.Title,
			Rank:         rank,
			TotalScore:   row.Total,
			RatingBefore: ratingBefore,
			RatingAfter:  ratingAfter,
			RatingChange: ratingAfter - ratingBefore,
			CreatedAt:    time.Now(),
		}
		histories = append(histories, history)

		// Update user's rating in user table
		if err := tx.Model(&User{}).Where("username = ?", username).Update("rating", ratingAfter).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// Batch insert all rating histories
	if err := tx.Create(&histories).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Reveal contest problem test results to non-editors
	if err := revealContestProblemTestsTx(tx, contest.ID); err != nil {
		log.Printf("Warning: failed to reveal problem tests for contest %d: %v", contest.ID, err)
		// Don't fail the transaction for this warning
	}

	return tx.Commit().Error
}

// revealContestProblemTestsTx sets TestVisible=true for all problems in a contest (within transaction)
func revealContestProblemTestsTx(tx *gorm.DB, contestID uint) error {
	// Get contest with its problems
	var contest Contest
	if err := tx.Preload("Problems").First(&contest, contestID).Error; err != nil {
		return err
	}

	if len(contest.Problems) == 0 {
		return nil
	}

	// Build problem IDs
	problemIDs := make([]uint, len(contest.Problems))
	for i, p := range contest.Problems {
		problemIDs[i] = p.ID
	}

	// Update all problems in the contest to reveal test results
	return tx.Model(&Problem{}).Where("id IN ?", problemIDs).Update("test_visible", true).Error
}

// revealContestProblemTests sets TestVisible=true for all problems in a contest
func revealContestProblemTests(contestID uint) error {
	if db == nil {
		return errors.New("db not initialized")
	}

	// Get contest with its problems
	contest, err := GetContestByID(contestID)
	if err != nil {
		return err
	}

	if len(contest.Problems) == 0 {
		return nil
	}

	// Build problem IDs
	problemIDs := make([]uint, len(contest.Problems))
	for i, p := range contest.Problems {
		problemIDs[i] = p.ID
	}

	// Update all problems in the contest to reveal test results
	return db.Model(&Problem{}).Where("id IN ?", problemIDs).Update("test_visible", true).Error
}

// GetUserRatingHistory returns all rating changes for a user
func GetUserRatingHistory(username string) ([]ContestRatingHistory, error) {
	if db == nil {
		return nil, errors.New("db not initialized")
	}

	var histories []ContestRatingHistory
	if err := db.Where("username = ?", username).Order("created_at desc").Find(&histories).Error; err != nil {
		return nil, err
	}
	return histories, nil
}

// GetEndedContestsWithoutRating returns contests that have ended but rating not yet settled
func GetEndedContestsWithoutRating() ([]Contest, error) {
	if db == nil {
		return nil, errors.New("db not initialized")
	}

	now := time.Now()
	var contests []Contest

	// Find contests that have ended and not yet settled
	if err := db.Where("end_at < ? AND rating_settled = ?", now, false).Find(&contests).Error; err != nil {
		return nil, err
	}

	return contests, nil
}

// GetStartedContestsWithoutReveal returns contests that have started but problems are not yet visible
func GetStartedContestsWithoutReveal() ([]Contest, error) {
	if db == nil {
		return nil, errors.New("db not initialized")
	}

	now := time.Now()
	var contests []Contest

	// Find contests that have started but still have hidden problems
	// A contest needs reveal if it has started AND has at least one problem with problem_visible=false
	if err := db.Where("start_at <= ?", now).
		Where("EXISTS (SELECT 1 FROM contest_problems cp JOIN problems p ON p.id = cp.problem_id WHERE cp.contest_id = contests.id AND p.problem_visible = false)").
		Find(&contests).Error; err != nil {
		return nil, err
	}

	return contests, nil
}
