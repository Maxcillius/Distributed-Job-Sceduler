package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/go-logr/logr"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/maxcillius/Distributed-Job-Scheduler/db"
	"github.com/maxcillius/Distributed-Job-Scheduler/logger"
	"github.com/maxcillius/Distributed-Job-Scheduler/pkg"
	"github.com/maxcillius/Distributed-Job-Scheduler/pkg/worker"
	"golang.org/x/sys/unix"
)

func loadenv() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("No .env file found")
	}
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

	pool, err := NewDatabase(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to the databse: %w", err))
	}

	switch *mode {
	case "manager":
		runManager(ctx, l, pool)
	case "worker":
		runWorker(ctx, l, pool)
	default:
		fmt.Println("Invalid mode. Use -mode=manager or -mode=worker")
		os.Exit(1)
	}
}

func runManager(ctx context.Context, l logr.Logger, pool *db.Queries) {
	fmt.Println("Starting System [MANAGER MODE]...")
	errChan := make(chan error, 10)
	trigChan := make(chan struct{}, 1)

	mlog := l.WithName("manager")
	mlog.Info("Starting System", "mode", "manager")

	go func() {
		pkg.Watcher(ctx, mlog.WithName("watcher"), trigChan, errChan, pool)
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

func runWorker(ctx context.Context, l logr.Logger, pool *db.Queries) {
	wlog := l.WithName("worker")

	wlog.Info("Starting System", "mode", "worker")
	worker.StartWorker(ctx, wlog, pool)
	wlog.Info("Worker shutting down...")
}

func NewDatabase(ctx context.Context) (*db.Queries, error) {
	dbUrl, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		return nil, fmt.Errorf("invalid DATABASE_URL")
	}

	dbConn, err := pgxpool.New(ctx, dbUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to the database: %w", err)
	}

	err = dbConn.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("error pinging datbase: %w", err)
	}

	db := db.New(dbConn)

	return db, nil
}
