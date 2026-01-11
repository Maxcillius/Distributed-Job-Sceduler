package pkg

import (
	"context"
	"fmt"
	"os/exec"
)

func Scheduler(ctx context.Context, trigChan <-chan struct{}, errChan chan<- error) {
	select {
	case <-ctx.Done():
	case <-trigChan:
	default:
		cmd := exec.Command("ls", "./Jobs")
		stdout, err := cmd.Output()
		if err != nil {
			errChan <- err
			return
		}

		files := string(stdout)
		fmt.Println(files)
	}
}
