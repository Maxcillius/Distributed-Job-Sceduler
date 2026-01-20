package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/go-logr/logr"
	"github.com/redis/go-redis/v9"
)

func StartWorker(ctx context.Context, log logr.Logger) {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	log.Info("Connected to Redis. Waiting for jobs...")

	for {
		select {
		case <-ctx.Done():
			return
		default:
			result, err := rdb.BRPop(ctx, 0, "job_queue").Result()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				fmt.Printf("[Worker] Redis error: %v\n", err)
				time.Sleep(1 * time.Second)
				continue
			}

			payload := result[1]
			var task JobTask
			if err := json.Unmarshal([]byte(payload), &task); err != nil {
				fmt.Printf("[Worker] Invalid job payload: %v\n", err)
				continue
			}

			log.Info("Received Task", "job_name", task.Name)
			runLocalJob(ctx, task)
		}
	}
}

func runLocalJob(ctx context.Context, task JobTask) {
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
