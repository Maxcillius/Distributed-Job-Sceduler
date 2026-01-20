package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
)

func Scheduler(ctx context.Context, log logr.Logger, trigChan <-chan struct{}, errChan chan<- error) {
	log.Info("Reconcile trigger received. Scheduling jobs...")
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	for {
		select {
		case <-ctx.Done():
			return
		case <-trigChan:
			fmt.Println("[Manager] Reconcile trigger received. Scheduling jobs...")

			entries, err := os.ReadDir("./Jobs")
			if err != nil {
				errChan <- fmt.Errorf("failed to read Jobs dir: %w", err)
				continue
			}

			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yml") {
					task, err := parseJobFile(entry.Name())
					if err != nil {
						errChan <- err
						continue
					}

					data, err := json.Marshal(task)
					if err != nil {
						errChan <- err
						continue
					}

					if err := rdb.LPush(ctx, "job_queue", data).Err(); err != nil {
						log.Error(err, "Failed to schedule job", "job_name", task.Name)
						errChan <- err
						continue
					}
					log.Info("Dispatched job", "job_name", task.Name)
				}
			}
		}
	}
}

func parseJobFile(filename string) (*JobTask, error) {
	path := filepath.Join("./Jobs", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var task JobTask
	if err := yaml.Unmarshal(data, &task); err != nil {
		return nil, err
	}
	return &task, nil
}
