package pkg

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/maxcillius/Distributed-Job-Scheduler/db"
	"github.com/maxcillius/Distributed-Job-Scheduler/types"
	amqp "github.com/rabbitmq/amqp091-go"
	"gopkg.in/yaml.v3"
)

func failOnError(err error, msg string, errChan chan<- error) {
	if err != nil {
		errChan <- fmt.Errorf("%s %w", msg, err)
	}
}

func Scheduler(ctx context.Context, log logr.Logger, trigChan <-chan struct{}, errChan chan<- error, pool *db.Queries) {
	log.Info("Reconcile trigger received. Scheduling jobs...")

	amqpURL, ok := os.LookupEnv("RABBITMQ")
	if !ok {
		panic(fmt.Errorf("cannot get RabbitMQ url"))
	}

	conn, err := amqp.Dial(amqpURL)
	failOnError(err, "failed to connect to RabbitMQ", errChan)
	defer conn.Close()
	log.Info("Connected to RabbitMQ")

	ch, err := conn.Channel()
	failOnError(err, "failed to open a channel", errChan)
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
	failOnError(err, "failed to declare a queue", errChan)
	log.Info("Declared a queue")

	for {
		select {
		case <-ctx.Done():
			return
		case <-trigChan:
			fmt.Println("[Manager] Reconcile trigger received. Scheduling jobs...")

			entries, err := os.ReadDir("./Jobs")
			if err != nil {
				failOnError(err, "failed to read Jobs dir: ", errChan)
				continue
			}

			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yml") {
					task, err := parseJobFile(entry.Name())
					if err != nil {
						failOnError(err, "failed to parse job file", errChan)
						continue
					}

					h := sha256.New()
					h.Write([]byte(fmt.Sprintf("%s%s%s%s", task.Name, task.Command, task.WorkDir, task.Args)))
					hash := h.Sum(nil)

					exist, err := pool.GetJobStatus(ctx, string(hash))
					if err != nil {
						log.Error(err, fmt.Sprintf("failed to get the status of %s", task.Name))
						continue
					}

					if exist.Status == "Done" {
						continue
					}

					data, err := json.Marshal(task)
					if err != nil {
						failOnError(err, "Failed to marshal the task data", errChan)
						continue
					}

					if err := ch.PublishWithContext(ctx, "", q.Name, false, false,
						amqp.Publishing{
							ContentType: "text/plain",
							Body:        []byte(data),
						}); err != nil {
						log.Error(err, "failed to schedule job", "job_name", task.Name)
						failOnError(err, fmt.Sprintf("failed to schedule job: %s", task.Name), errChan)
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
