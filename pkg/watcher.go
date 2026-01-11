package pkg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type head struct {
	commit string
}

// func getPath(key string) (string, bool) {
// 	value, ok := os.LookupEnv(key)
// 	if !ok {
// 		return "", false
// 	}
// 	return value, true
// }

func Watcher(ctx context.Context, trigChan chan<- struct{}, errChan chan<- error) {
	currCommit := &head{
		"",
	}

	// directory, ok := getPath("DIR")
	// fmt.Println(directory)
	// if !ok {
	// 	err := fmt.Errorf("No jobs directory found\n")
	// 	errChan <- err
	// 	return
	// }

	gitPath, err := exec.Command("which", "git").Output()
	if err != nil {
		errChan <- err
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Add functionality to pull at the start of the program
			_, err := os.Stat("temp_jobs")
			if os.IsNotExist(err) {
				create := exec.Command("mkdir", "temp_jobs").Run()
				if create != nil {
					errChan <- create
					return
				}
			} else if err != nil {
				errChan <- err
			}
			pull := exec.Cmd{
				Path: strings.TrimSpace(string(gitPath)),
				Args: []string{"git", "pull"},
				Dir:  "./temp_jobs",
			}
			checkHead := exec.Cmd{
				Path: strings.TrimSpace(string(gitPath)),
				Args: []string{"git", "rev-parse", "HEAD"},
				Dir:  "./temp_jobs",
			}
			if err := pull.Run(); err != nil {
				errChan <- err
				return
			}
			fmt.Println("Refreshed the repo")
			fmt.Println("Checking for udpate...")
			headInfo, err := checkHead.Output()
			if err != nil {
				errChan <- err
				fmt.Printf("Error while checking HEAD: %v\n", err)
				return
			}
			newCommit := string(headInfo)
			if currCommit.commit != newCommit {
				currCommit.commit = newCommit
				err = exec.Command("mv", "temp_jobs", "Jobs").Run()
				if err != nil {
					errChan <- err
					return
				}
				trigChan <- struct{}{}
			}
			time.Sleep(time.Minute)
			// Add ticker instead
		}
	}
}
