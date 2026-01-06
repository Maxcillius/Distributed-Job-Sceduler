package main

import (
	"context"
	"fmt"
	"os"

	"github.com/maxcillius/Distributed-Job-Scheduler/pkg"
)

func main() {
	fmt.Println("Starting scheduler...")
	os.Exit(run())
}

func run() int {

	ctx, _ := context.WithCancel(context.Background())
	errChan := make(chan error)

	go func() {
		pkg.Watcher(ctx, errChan)
	}()

	select {
	case <-ctx.Done():
		return 0
	case err := <-errChan:
		fmt.Printf("error occurred: %v", err)
		return 1
	}
}
