package pkg

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

func Watcher(ctx context.Context, errChan chan<- error) {
	for {
		pull := exec.Cmd{
			Path: "/usr/bin/git",
			Args: []string{"git", "pull"},
			Dir:  "./Jobs",
		}
		if err := pull.Run(); err != nil {
			errChan <- err
			fmt.Printf("%v\n", err)
			panic("cannot pull repo")
		}
		fmt.Println("Refreshed the repo")
		fmt.Println("Checking for udpate...")
		checkHead := exec.Cmd{
			Path: "usr/bin/git",
			Args: []string{"git", "show", "HEAD"},
			Dir:  "./Jobs",
		}
		if err := checkHead.Run(); err != nil {
			errChan <- err
			fmt.Printf("Error while checking HEAD: %v", err)
		}
		time.Sleep(time.Minute)
	}
}
