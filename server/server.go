package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/minicago/gooj/cmd"
	"github.com/minicago/gooj/config"
	"github.com/minicago/gooj/file_service"
	"github.com/minicago/gooj/judge"
	"github.com/minicago/gooj/manage"
	"github.com/minicago/gooj/sql_service"
	"github.com/minicago/gooj/web"
	"github.com/sevlyar/go-daemon"
)

func listen(cmdChan chan string) {
	handler := web.NewRouter()

	addr := fmt.Sprintf(":%d", config.GetServerPort())
	srv := http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("%v", err)
		}
	}()
	fmt.Printf("listening on %s\n", addr)

	for {
		cmdStr := <-cmdChan
		cmdStr = strings.TrimSpace(cmdStr)
		if strings.EqualFold(cmdStr, "shutdown") {
			break
		}
		if strings.EqualFold(cmdStr, "clear message") {
			if err := web.ClearMessages(); err != nil {
				log.Printf("clear messages failed: %v", err)
			} else {
				log.Printf("messages cleared")
			}
		}
	}
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Fatalf("%v", err)
	}
}

func StartServer(isbackground bool) {

	if isbackground {
		cntxt := &daemon.Context{
			WorkDir: "./",
		}
		d, err := cntxt.Reborn()
		if err != nil {
			log.Fatal(err)
		}
		if d != nil {
			return
		}
	}

	// Start services based on configuration
	if config.IsSQLEnabled() {
		if err := sql_service.Init(); err != nil {
			panic(err)
		}
		log.Println("SQL service started")
	} else {
		log.Println("SQL service disabled")
	}

	if config.IsFileEnabled() {
		file_service.StartDefault()
		log.Println("File service started")
	} else {
		log.Println("File service disabled")
	}

	if config.IsJudgeEnabled() {
		judge.StartJudge()
		log.Println("Judge service started")
	} else {
		log.Println("Judge service disabled")
	}

	// Start background rating calculator
	StartRatingCalculator()
	log.Println("Rating calculator started")

	manage.Init()
	cmdChan := make(chan string)
	shutdownChan := make(chan int)
	go cmd.StartCmdServer(cmdChan, shutdownChan)

	listen(cmdChan)
	shutdownChan <- 0
}

// StartRatingCalculator starts a background goroutine that checks for ended contests
// and automatically calculates rating changes for participants
func StartRatingCalculator() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			<-ticker.C
			contests, err := sql_service.GetEndedContestsWithoutRating()
			if err != nil {
				log.Printf("Rating calculator: failed to get ended contests: %v", err)
				continue
			}

			for _, contest := range contests {
				if err := sql_service.CalculateContestRating(contest.ID); err != nil {
					log.Printf("Rating calculator: failed to calculate rating for contest %d: %v", contest.ID, err)
				} else {
					log.Printf("Rating calculator: calculated ratings for contest %d (%s)", contest.ID, contest.Title)
				}
			}
		}
	}()
}
