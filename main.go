package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/maxcillius/Distributed-Job-Scheduler/pkg"
	"golang.org/x/sys/unix"
)

func main() {
	fmt.Println("Starting scheduler...")
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), unix.SIGTERM, unix.SIGINT)
	defer stop()
	errChan := make(chan error)
	trigChan := make(chan struct{})

	go func() {
		for range trigChan {
			fmt.Println("Got the update request")
			// Signal scheduler for updation
		}
	}()

	go func() {
		pkg.Watcher(ctx, trigChan, errChan)
	}()

	select {
	case <-ctx.Done():
		return 0
	case err := <-errChan:
		fmt.Printf("error occurred: %v", err)
		return 1
	}
}
