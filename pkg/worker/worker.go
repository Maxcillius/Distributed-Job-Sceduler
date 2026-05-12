package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/go-logr/logr"
	"github.com/maxcillius/Distributed-Job-Scheduler/db"
	"github.com/maxcillius/Distributed-Job-Scheduler/logger"
	"github.com/maxcillius/Distributed-Job-Scheduler/types"
	amqp "github.com/rabbitmq/amqp091-go"
)

func failOnError(err error, msg string, l logr.Logger) {
	if err != nil {
		l.Info("%s %w", msg, err)
	}
}

func StartWorker(ctx context.Context, log logr.Logger, pool *db.Queries) {

	l, err := logger.New()
	if err != nil {
		panic(fmt.Errorf("Failed to build logger: %w", err))
	}
	wlogger := l.WithName("worker")

	amqpURL, ok := os.LookupEnv("RABBITMQ")
	if !ok {
		panic(fmt.Errorf("Cannot get RabbitMQ url"))
	}

	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		panic(fmt.Errorf("Failed to connect to RabbitMQ: %w", err))
	}
	wlogger.Info("Connected to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		panic(fmt.Errorf("Failed to build queue channel: %w", err))
	}
	wlogger.Info("Opened a channel")
	defer ch.Close()

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
	failOnError(err, "Failed to declare queue", wlogger)
	wlogger.Info("Declared a queue")

	jobs, err := ch.Consume(
		q.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		panic(fmt.Errorf("Failed to register a consumer: %w", err))
	}
	wlogger.Info("Registered a consumer")

	for {
		select {
		case <-ctx.Done():
			wlogger.Info("Shutting down the worker")
			return
		default:
			for d := range jobs {
				payload := d.Body
				var task types.JobTask
				if err := json.Unmarshal([]byte(payload), &task); err != nil {
					wlogger.Info("[Worker] Invalid job payload: %v\n", err)
					continue
				}

				wlogger.Info("Received Task", "job_name", task.Name)
				avoid := true // Avoiding running jobs while in development status
				if avoid {
					continue
				} else {
					runLocalJob(ctx, task)
				}
			}
		}
	}
}

func runLocalJob(ctx context.Context, task types.JobTask) {
	jobCtx := ctx
	if task.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		jobCtx, cancel = context.WithTimeout(ctx, time.Duration(task.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(jobCtx, task.Command, task.Args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	for k, v := range task.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("[Worker] Job %s failed: %v\n", task.Name, err)
	} else {
		fmt.Printf("[Worker] Job %s completed.\n", task.Name)
	}
}
