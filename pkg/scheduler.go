package pkg

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/maxcillius/Distributed-Job-Scheduler/db"
	"github.com/maxcillius/Distributed-Job-Scheduler/repository"
	"github.com/maxcillius/Distributed-Job-Scheduler/types"
	amqp "github.com/rabbitmq/amqp091-go"
	"gopkg.in/yaml.v3"
)

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

func Scheduler(ctx context.Context, log logr.Logger, trigChan <-chan struct{}, errChan chan<- error, pool *repository.DbCall) {
	log.Info("Reconcile trigger received. Scheduling jobs...")

	amqpURL, ok := os.LookupEnv("RABBITMQ")
	if !ok {
		panic(fmt.Errorf("cannot get RabbitMQ url"))
	}

	var conn *amqp.Connection

	err := executeWithBackoff(ctx, log, "RabbitMQ Connection", 5, 2*time.Second, func() error {
		var connErr error
		conn, connErr = amqp.Dial(amqpURL)
		return connErr
	})

	if err != nil {
		errChan <- fmt.Errorf("failed to connect to RabbitMQ: %w", err)
		return
	}
	defer conn.Close()
	log.Info("Connected to RabbitMQ")

	ch, err := conn.Channel()
	if err != nil {
		errChan <- fmt.Errorf("failed to open a channel: %w", err)
		return
	}
	defer ch.Close()
	log.Info("Opened a channel")

	q, err := ch.QueueDeclare(
		"jobs",
		true,
		false,
		false,
		false,
		amqp.Table{
			amqp.QueueTypeArg: amqp.QueueTypeQuorum,
		},
	)
	if err != nil {
		errChan <- fmt.Errorf("failed to declare producer queue: %w", err)
		return
	}
	log.Info("Declared a queue")

	for {
		select {
		case <-ctx.Done():
			return
		case <-trigChan:
			fmt.Println("[Manager] Reconcile trigger received. Scheduling jobs...")

			entries, err := os.ReadDir("./Jobs")
			if err != nil {
				errChan <- fmt.Errorf("failed to read jobs dir: %w", err)
				continue
			}

			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yml") {
					task, err := parseJobFile(entry.Name())
					if err != nil {
						errChan <- fmt.Errorf("failed to parse job file: %w", err)
						continue
					}

					h := sha256.New()
					h.Write([]byte(fmt.Sprintf("%s%s%s%s", task.Name, task.Command, task.WorkDir, task.Args)))
					hash := h.Sum(nil)

					exist, err := pool.IsJobActive(ctx, string(hash))
					if err != nil {
						log.Error(err, fmt.Sprintf("failed to check the status of %s", task.Name))
						continue
					}
					if exist {
						continue
					}

					data, err := json.Marshal(task)
					if err != nil {
						errChan <- fmt.Errorf("failed to marshal the data of %s: %w", task.Name, err)
						continue
					}

					jobDetails := db.InsertJobParams{
						ID:             string(hash),
						Name:           task.Name,
						Command:        task.Command,
						Args:           task.Args,
						Workdir:        pgtype.Text{task.WorkDir, true},
						Timeoutseconds: pgtype.Int4{int32(task.TimeoutSeconds), true},
						Status:         "waiting",
					}

					err = pool.UpsertJobDefinition(ctx, jobDetails)
					if err != nil {
						log.Error(err, fmt.Sprintf("failed to insert in database %s", task.Name))
						// logging error at scheduler level
						errChan <- fmt.Errorf("failed to insert job in database %s: %w", task.Name, err)
						continue
						// Avoid pushing jobs into queue if failed to insert into the database
					}

					if err := ch.PublishWithContext(ctx, "", q.Name, false, false,
						amqp.Publishing{
							ContentType: "text/plain",
							Body:        []byte(data),
						}); err != nil {
						log.Error(err, "failed to schedule job", "job_name", task.Name)
						errChan <- fmt.Errorf("failed to schedule job %s: %w", task.Name, err)
						continue
					}
					log.Info("Dispatched job", "job_name", task.Name)
				}
			}
		}
	}
}

func parseJobFile(filename string) (*types.JobTask, error) {
	path := filepath.Join("./Jobs", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var task types.JobTask
	if err := yaml.Unmarshal(data, &task); err != nil {
		return nil, err
	}
	return &task, nil
}
