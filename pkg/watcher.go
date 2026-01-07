package pkg

import (
	"context"
	"fmt"
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
			pull := exec.Cmd{
				Path: strings.TrimSpace(string(gitPath)),
				Args: []string{"git", "pull"},
				Dir:  "./Jobs",
			}
			if err := pull.Run(); err != nil {
				errChan <- err
				return
			}
			fmt.Println("Refreshed the repo")
			fmt.Println("Checking for udpate...")
			checkHead := exec.Cmd{
				Path: strings.TrimSpace(string(gitPath)),
				Args: []string{"git", "rev-parse", "HEAD"},
				Dir:  "./Jobs",
			}
			info, err := checkHead.Output()
			if err != nil {
				errChan <- err
				fmt.Printf("Error while checking HEAD: %v\n", err)
				return
			}
			newCommit := string(info)
			if currCommit.commit != newCommit {
				currCommit.commit = newCommit
				trigChan <- struct{}{}
			}
			time.Sleep(time.Minute)
			// Add ticker instead
		}
	}
}
