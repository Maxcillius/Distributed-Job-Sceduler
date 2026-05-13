# Distributed Job Scheduler

A distributed system for orchestrating containerized jobs across a cluster of workers. This scheduler uses a GitOps workflow to sync job definitions and Redis for task distribution.

## Architecture

The system decouples job scheduling from execution using a manager-worker pattern.

* **Manager:** Watches a Git repository for changes. When the repository updates, the manager parses job configuration files and pushes tasks to a Redis queue.
* **Worker:** Listens to the Redis queue, pulls tasks, and executes them locally (typically using Docker). Workers can be scaled horizontally across multiple machines.


<img width="2816" height="1536" alt="Gemini_Generated_Image_vdue1uvdue1uvdue" src="https://github.com/user-attachments/assets/ceb99fca-7d65-4d6f-877c-6df8aa5f479b" />



## Features

* **GitOps Sync:** Job configurations are version-controlled in Git. The system automatically detects commits and updates the schedule.
* **Atomic Updates:** Repository changes are applied using atomic directory swaps to prevent partial reads or race conditions.
* **Distributed Execution:** Jobs run on separate worker nodes, allowing for scalable processing power.
* **Graceful Shutdown:** The system handles termination signals (`SIGTERM`/`SIGINT`) to allow running jobs to complete or cancel safely.
* **Structured Logging:** All components emit structured JSON logs suitable for ingestion by observability tools like Datadog, Splunk, or CloudWatch.

## Prerequisites

* Go (version 1.21 or higher)
* Redis (version 6.0 or higher)
* Docker (required on Worker nodes)
* Git

## Installation

1.  **Clone the repository and install dependencies:**

    ```bash
    git clone https://github.com/Maxcillius/Distributed-Job-Sceduler.git
    cd Distributed-Job-Scheduler
    go mod tidy
    ```

2.  **Ensure Redis is running.** You can start a local instance using Docker:

    ```bash
    docker run -d -p 6379:6379 redis
    ```

## Usage

The application binary runs in one of two modes: **manager** or **worker**.

### Manager Mode
The manager connects to the configured Git repository and schedules jobs.

```bash
go run main.go -mode=manager
```

### Worker Mode
The worker connects to Redis and executes jobs. You can run multiple worker instances to increase throughput.

```bash
go run main.go -mode=manager
```

## Job Configuration

Jobs are defined as .yml files located in the Jobs directory of the watched repository.
Example: job.yml

```bash
name: production-service
command: docker
args:
  - run
  - --rm
  - --name
  - my-service
  - -p
  - "8080:80"
  - nginx:latest
workdir: .
timeout_seconds: 300
env:
  APP_ENV: "production"
  API_KEY: "secret-value"
```

### Configuration Fields

* name: A unique identifier for the job.
* command: The executable to run (e.g., docker, bash).
* args: A list of arguments passed to the command.
* workdir: The working directory for execution.
* timeout_seconds: The maximum duration the job is allowed to run before being killed.
*   env: A map of environment variables injected into the process.

## Observability
Logs are written to standard output (stdout) in JSON format.

```bash
{
  "level": "info",
  "timestamp": "2026-01-20T15:30:00.000Z",
  "logger": "worker",
  "caller": "pkg/worker.go:45",
  "message": "Received Task",
  "job_name": "crash-test-job"
}
```

## Project Structure
* main.go: Entry point handling command-line flags and signal interrupts.
* pkg/scheduler.go: Logic for parsing job files and enqueuing tasks to Redis.
* pkg/watcher.go: Handles Git repository synchronization and atomic directory management.
* pkg/worker.go: Logic for dequeuing tasks from Redis and executing subprocesses.
* pkg/types.go: Shared data structures for job definitions.
* logger/logger.go: Configuration for the structured logger.

## Contributing
1. Fork the repository.
2. Create a feature branch.
3. Commit your changes.
4. Push to the branch.
5. Open a pull request.

# License
  MIT
