package sql_service

import (
	"errors"
	"sort"
	"time"

	"gorm.io/gorm"
)

// ContestRankingRow describes one contestant's aggregate score in a contest.
type ContestRankingRow struct {
	Username string `json:"username"`
	Score    int    `json:"score"`
	Solved   int    `json:"solved"`
}

// ListContests returns all contest definitions sorted by start time.
func ListContests() ([]Contest, error) {
	if db == nil {
		return nil, errors.New("db not initialized")
	}
	var contests []Contest
	if err := db.Order("start_at asc").Find(&contests).Error; err != nil {
		return nil, err
	}
	return contests, nil
}

// GetContestByID returns a contest by ID.
func GetContestByID(id uint) (Contest, error) {
	if db == nil {
		return Contest{}, errors.New("db not initialized")
	}
	var contest Contest
	if err := db.First(&contest, id).Error; err != nil {
		return Contest{}, err
	}
	return contest, nil
}

// CreateContest stores a contest and its linked problem IDs.
func CreateContest(title, description, createdBy string, startAt, endAt time.Time, problemIDs []uint) (Contest, error) {
	if db == nil {
		return Contest{}, errors.New("db not initialized")
	}
	contest := Contest{Title: title, Description: description, CreatedBy: createdBy, StartAt: startAt, EndAt: endAt}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&contest).Error; err != nil {
			return err
		}
		for _, problemID := range problemIDs {
			if problemID == 0 {
				continue
			}
			if err := tx.Create(&ContestProblem{ContestID: contest.ID, ProblemID: problemID}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return Contest{}, err
	}
	return contest, nil
}

// DeleteContest removes a contest together with its problem link records.
func DeleteContest(id uint) error {
	if db == nil {
		return errors.New("db not initialized")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("contest_id = ?", id).Delete(&ContestProblem{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&Contest{}, id).Error; err != nil {
			return err
		}
		return nil
	})
}

// ListContestProblems returns all problems linked to a contest.
func ListContestProblems(contestID uint) ([]Problem, error) {
	if db == nil {
		return nil, errors.New("db not initialized")
	}
	var problemIDs []uint
	if err := db.Model(&ContestProblem{}).Where("contest_id = ?", contestID).Pluck("problem_id", &problemIDs).Error; err != nil {
		return nil, err
	}
	if len(problemIDs) == 0 {
		return []Problem{}, nil
	}
	var problems []Problem
	if err := db.Where("id IN ?", problemIDs).Order("id asc").Find(&problems).Error; err != nil {
		return nil, err
	}
	return problems, nil
}

// GetContestLeaderboard computes a simple scoreboard for the problems in a contest.
func GetContestLeaderboard(contestID uint) ([]ContestRankingRow, error) {
	if db == nil {
		return nil, errors.New("db not initialized")
	}
	contest, err := GetContestByID(contestID)
	if err != nil {
		return nil, err
	}
	problemIDs, err := listContestProblemIDs(contestID)
	if err != nil {
		return nil, err
	}
	if len(problemIDs) == 0 {
		return []ContestRankingRow{}, nil
	}

	var submissions []Submission
	if err := db.Where("problem_id IN ?", problemIDs).
		Where("created_at >= ? AND created_at <= ?", contest.StartAt, contest.EndAt).
		Where("status IN ?", []string{"ok", "accepted", "wa", "tle", "mle", "runtime_error", "compile_error"}).
		Order("username asc, problem_id asc, score desc, created_at desc").
		Find(&submissions).Error; err != nil {
		return nil, err
	}

	bestScoreByUserProblem := make(map[string]map[uint]int)
	for _, submission := range submissions {
		if submission.Username == "" {
			continue
		}
		if _, ok := bestScoreByUserProblem[submission.Username]; !ok {
			bestScoreByUserProblem[submission.Username] = make(map[uint]int)
		}
		currentBest, exists := bestScoreByUserProblem[submission.Username][submission.ProblemID]
		if !exists || submission.Score > currentBest {
			bestScoreByUserProblem[submission.Username][submission.ProblemID] = submission.Score
		}
	}

	leaderboard := make(map[string]*ContestRankingRow)
	for username, problemScore := range bestScoreByUserProblem {
		row := &ContestRankingRow{Username: username}
		for _, score := range problemScore {
			row.Score += score
			if score > 0 {
				row.Solved++
			}
		}
		leaderboard[username] = row
	}

	result := make([]ContestRankingRow, 0, len(leaderboard))
	for _, row := range leaderboard {
		result = append(result, *row)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		if result[i].Solved != result[j].Solved {
			return result[i].Solved > result[j].Solved
		}
		return result[i].Username < result[j].Username
	})
	return result, nil
}

func listContestProblemIDs(contestID uint) ([]uint, error) {
	if db == nil {
		return nil, errors.New("db not initialized")
	}
	var ids []uint
	if err := db.Model(&ContestProblem{}).Where("contest_id = ?", contestID).Order("problem_id asc").Pluck("problem_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
