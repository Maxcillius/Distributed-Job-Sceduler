package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"github.com/go-logr/logr"
	"github.com/joho/godotenv"
	"github.com/maxcillius/Distributed-Job-Scheduler/logger"
	"github.com/maxcillius/Distributed-Job-Scheduler/pkg"
	"github.com/maxcillius/Distributed-Job-Scheduler/pkg/worker"
	"github.com/maxcillius/Distributed-Job-Scheduler/repository"
	"golang.org/x/sys/unix"
)

func loadenv() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("No .env file found")
	}
}

func executeWithBackoff(ctx context.Context, log logr.Logger, operationName string, maxRetries int, baseDelay time.Duration, fn func() error) error {
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := fn()
		if err == nil {
			if attempt > 0 {
				log.Info("Operation succeeded after retries", "operation", operationName)
			}
			return nil
		}

		if attempt == maxRetries-1 {
			return fmt.Errorf("%s failed permanently after %d attempts: %w", operationName, maxRetries, err)
		}

		backoff := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))

		jitter := time.Duration(rand.Float64() * float64(backoff) * 0.2)
		sleepDuration := backoff + jitter

		log.Error(err, "Operation failed, backing off and retrying",
			"operation", operationName,
			"attempt", attempt+1,
			"sleep", sleepDuration.String())

		select {
		case <-time.After(sleepDuration):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func main() {
	loadenv()

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

	var db *repository.DbCall

	err = executeWithBackoff(ctx, l, "Database Connection", 5, 2*time.Second, func() error {
		var connErr error
		db, connErr = repository.NewConnection(ctx)
		return connErr
	})

	if err != nil {
		l.Error(err, "critical initialization failed")
		os.Exit(1)
	}

	switch *mode {
	case "manager":
		runManager(ctx, l, db)
	case "worker":
		runWorker(ctx, l, db)
	default:
		fmt.Println("Invalid mode. Use -mode=manager or -mode=worker")
		os.Exit(1)
	}
}

func runManager(ctx context.Context, l logr.Logger, pool *repository.DbCall) {
	errChan := make(chan error, 10)
	trigChan := make(chan struct{}, 1)

	mlog := l.WithName("manager")
	mlog.Info("Starting System", "mode", "manager")

	go func() {
		pkg.Watcher(ctx, mlog.WithName("watcher"), trigChan, errChan)
	}()

	go func() {
		pkg.Scheduler(ctx, mlog.WithName("scheduler"), trigChan, errChan, pool)
	}()

	for {
		select {
		case <-ctx.Done():
			mlog.Info("Manager shutting down...")
			return
		case err := <-errChan:
			fmt.Printf("Error: %v\n", err)
			mlog.Error(err, "component failure detected")
		}
	}
}

func runWorker(ctx context.Context, l logr.Logger, pool *repository.DbCall) {
	wlog := l.WithName("worker")

	wlog.Info("Starting System", "mode", "worker")
	worker.StartWorker(ctx, wlog, pool)
	wlog.Info("Worker shutting down...")
}
