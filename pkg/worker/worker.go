package worker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/go-logr/logr"
	"github.com/maxcillius/Distributed-Job-Scheduler/repository"
	"github.com/maxcillius/Distributed-Job-Scheduler/types"
	amqp "github.com/rabbitmq/amqp091-go"
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

func StartWorker(ctx context.Context, log logr.Logger, pool *repository.DbCall) {
	amqpURL, ok := os.LookupEnv("RABBITMQ")
	if !ok {
		panic(fmt.Errorf("Cannot get RabbitMQ url"))
	}

	var conn *amqp.Connection

	err := executeWithBackoff(ctx, log, "RabbitMQ Connection", 5, 2*time.Second, func() error {
		var connErr error
		conn, connErr = amqp.Dial(amqpURL)
		return connErr
	})

	if err != nil {
		log.Error(err, "failed to connect to RabbitMQ")
		return
	}
	defer conn.Close()
	log.Info("Connected to RabbitMQ")

	ch, err := conn.Channel()
	if err != nil {
		panic(fmt.Errorf("Failed to build queue channel: %w", err))
	}
	log.Info("Opened a channel")
	defer ch.Close()

	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(fmt.Errorf("Failed to initialize Docker client: %w", err))
	}
	defer dockerCli.Close()
	log.Info("Connected to Docker Engine")

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
		panic(fmt.Errorf("Failed to declare consumer queue: %w", err))
	}
	log.Info("Declared a queue")

	jobs, err := ch.Consume(
		q.Name,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		panic(fmt.Errorf("Failed to register a consumer: %w", err))
	}
	log.Info("Registered a consumer")

	for {
		select {
		case <-ctx.Done():
			log.Info("Shutting down the worker")
			return
		case d := <-jobs:
			payload := d.Body
			var task types.JobTask
			if err := json.Unmarshal([]byte(payload), &task); err != nil {
				log.Error(err, "[Worker] Invalid job payload")
				_ = d.Reject(false) // Drop malformed messages
				continue
			}

			h := sha256.New()
			h.Write([]byte(fmt.Sprintf("%s%s%s%s", task.Name, task.Command, task.WorkDir, task.Args)))
			hash := h.Sum(nil)
			jobID := string(hash)

			log.Info("Received Task", "job_name", task.Name)

			if err := pool.UpdateJobStatus(ctx, jobID, "running"); err != nil {
				log.Error(err, fmt.Sprintf("failed to update job status. job: %s", task.Name))
				_ = d.Nack(false, true)
				continue
			}

			err = runDockerJob(ctx, dockerCli, task)
			if err != nil {
				log.Error(err, fmt.Sprintf("job execution failed: %s", task.Name))
				_ = pool.UpdateJobStatus(ctx, jobID, "failed")
				_ = d.Ack(false)
				continue
			}

			if err := pool.UpdateJobStatus(ctx, jobID, "done"); err != nil {
				log.Error(err, fmt.Sprintf("failed to update job status to done: %s", task.Name))
			}

			if err := d.Ack(false); err != nil {
				log.Error(err, "failed to acknowledge job: %s", task.Name)
			} else {
				log.Info("Job finished and acknowledged successfully", "job_name", task.Name)
			}
		}
	}
}

func runDockerJob(ctx context.Context, cli *client.Client, task types.JobTask) error {
	jobCtx := ctx
	if task.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		jobCtx, cancel = context.WithTimeout(ctx, time.Duration(task.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	imageName := task.Image
	if imageName == "" {
		imageName = "alpine:latest"
	}

	reader, err := cli.ImagePull(jobCtx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer reader.Close()
	_, _ = io.Copy(io.Discard, reader)

	cmd := []string{task.Command}
	cmd = append(cmd, task.Args...)

	var envs []string
	for k, v := range task.Env {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	resp, err := cli.ContainerCreate(jobCtx, &container.Config{
		Image:      imageName,
		Cmd:        cmd,
		Env:        envs,
		WorkingDir: task.WorkDir,
	}, nil, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	defer func() {
		_ = cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
	}()

	if err := cli.ContainerStart(jobCtx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	out, err := cli.ContainerLogs(jobCtx, resp.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err == nil {
		go func() {
			defer out.Close()
			_, _ = stdcopy.StdCopy(os.Stdout, os.Stderr, out)
		}()
	}

	statusCh, errCh := cli.ContainerWait(jobCtx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("container wait error: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("container exited with non-zero status: %d", status.StatusCode)
		}
	case <-jobCtx.Done():
		return fmt.Errorf("job timed out or was cancelled: %w", jobCtx.Err())
	}

	return nil
}
