package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/go-logr/logr"
	"github.com/maxcillius/Distributed-Job-Scheduler/logger"
	"github.com/maxcillius/Distributed-Job-Scheduler/pkg"
	"golang.org/x/sys/unix"
)

func main() {
	mode := flag.String("mode", "manager", "Run mode: 'manager' or 'worker'")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), unix.SIGTERM, unix.SIGINT)
	defer stop()
	l, err := logger.New()
	if err != nil {
		_, ferr := fmt.Fprintf(os.Stderr, "failed to create logger: %s", err)
		if ferr != nil {
			panic(fmt.Sprintf("failed to write log:`%s` original error is:`%s`", ferr, err))
		}
		os.Exit(1)
	}

	switch *mode {
	case "manager":
		runManager(ctx, l)
	case "worker":
		runWorker(ctx, l)
	default:
		fmt.Println("Invalid mode. Use -mode=manager or -mode=worker")
		os.Exit(1)
	}
}

func runManager(ctx context.Context, l logr.Logger) {
	fmt.Println("Starting System [MANAGER MODE]...")
	errChan := make(chan error, 10)
	trigChan := make(chan struct{}, 1)

	mlog := l.WithName("manager")
	mlog.Info("Starting System", "mode", "manager")

	go func() {
		pkg.Watcher(ctx, mlog.WithName("watcher"), trigChan, errChan)
	}()

	go func() {
		pkg.Scheduler(ctx, mlog.WithName("scheduler"), trigChan, errChan)
	}()

	for {
		select {
		case <-ctx.Done():
			mlog.Info("Manager shutting down...")
			return
		case err := <-errChan:
			fmt.Printf("Error: %v\n", err)
			mlog.Error(err, "Component failure detected")
		}
	}
}

func runWorker(ctx context.Context, l logr.Logger) {
	wlog := l.WithName("worker")

	wlog.Info("Starting System", "mode", "worker")
	pkg.StartWorker(ctx, wlog)
	wlog.Info("Worker shutting down...")
}
