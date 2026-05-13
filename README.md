# Distributed Job Scheduler

A distributed system for orchestrating containerized jobs across a cluster of workers. This scheduler uses a GitOps workflow to sync job definitions, RabbitMQ for robust message distribution, and PostgreSQL for state management and tracking.

## Architecture

The system decouples job scheduling from execution using a manager-worker pattern, ensuring high availability and horizontal scalability.

* **Manager:** Watches a Git repository for changes. When the repository updates, the manager parses job configuration files, registers them in PostgreSQL as waiting, and pushes the task payloads to a RabbitMQ Quorum queue.
* **Worker:** Listens to the RabbitMQ queue, updates the job state to running in the database, and executes the tasks natively using the Docker Engine API. Upon completion or failure, it updates the final state in the database and acknowledges the message.


<img width="2816" height="1536" alt="Gemini_Generated_Image_vdue1uvdue1uvdue" src="https://github.com/user-attachments/assets/ceb99fca-7d65-4d6f-877c-6df8aa5f479b" />



## Features

* **GitOps Sync:** Job configurations are version-controlled in Git. The system automatically detects commits and updates the schedule.
* **Docker Native Execution:** Workers interface directly with the Docker socket via the Go SDK, cleanly handling image pulls, container creation, environment injection, and lifecycle cleanup without relying on host shell scripts.
* **Resilient Infrastructure:** Features exponential backoff for database/broker connections on startup.
* **Poison Pill Protection:** Utilizes RabbitMQ Dead Letter Exchanges (DLX) and delivery limits to route malformed payloads or endlessly failing infrastructure tasks to a Dead Letter Queue (jobs_dlq).
* **State Tracking:** PostgreSQL provides a reliable source of truth for job statuses (waiting, running, done, failed).
* **Graceful Shutdown:** The system handles termination signals (SIGTERM/SIGINT) to allow running containers to complete or forcefully terminate via context cancellation.
* **Structured Logging:** All components emit structured JSON logs suitable for Datadog, Splunk, or CloudWatch.

## Prerequisites

* Go (version 1.25.0 or higher)
* RabbitMQ (version 3.10+ recommended for Quorum queue features)
* stgreSQL (version 15+)
* Docker Engine (required on Worker nodes, worker must have socket access)
* Git

## Installation

1.  **Clone the repository and install dependencies:**

    ```bash
    git clone https://github.com/Maxcillius/Distributed-Job-Scheduler.git
    cd Distributed-Job-Scheduler
    go mod tidy
    ```

2.  **Environment Variables:** Create a .env file in the root directory:

    ```bash
    DATABASE_URL="postgres://user:password@localhost:5432/jobsdb?sslmode=disable"
    RABBITMQ="amqp://guest:guest@localhost:5672/"
    REPO_URL="https://github.com/yourusername/your-job-repo.git"
    ```

3.  **Ensure Infrastructure is running:** 
    You can easily start Postgres and RabbitMQ locally using Docker:

    ```bash
    docker run -d --name rmq -p 5672:5672 -p 15672:15672 rabbitmq:3-management
    docker run -d --name pg -p 5432:5432 -e POSTGRES_PASSWORD=password -e POSTGRES_DB=jobsdb postgres:15
    ```

## Usage

The application binary runs in one of two modes: **manager** or **worker**.

### Manager Mode
The manager connects to the configured Git repository and schedules jobs.

```bash
go run main.go -mode=manager
```

### Worker Mode
The worker connects to RabbitMQ and executes jobs using the host's Docker daemon. You can run multiple worker instances to increase throughput.

```bash
go run main.go -mode=worker
```

## Job Configuration

Jobs are defined as .yml files located in the Jobs directory of the watched Git repository.
Example: job.yml

```bash
name: production-service
image: nginx:alpine
ports: 
  - "8080:80"
workdir: /usr/share/nginx/html
timeout_seconds: 300
env:
  APP_ENV: "production"
  API_KEY: "secret-value"
```

### Configuration Fields

* name: A unique identifier for the job.
* image: The Docker image to pull and run (e.g., alpine:latest, python:3.9).
* command: (Optional) The specific executable to run inside the container. If omitted, uses the image's default CMD/ENTRYPOINT.
* args: (Optional) A list of arguments passed to the command.
* ports: (Optional) Port mappings in the format "host:container" (e.g., "8080:80").
* workdir: (Optional) The absolute working directory inside the container.
* timeout_seconds: The maximum duration the container is allowed to run before the worker forcefully kills it.
* env: A map of environment variables natively injected into the container.

## Observability
Logs are written to standard error (stderr) and standard output (stdout) in JSON format.

```bash
{
  "level": "info",
  "timestamp": "2026-05-13T16:08:06.134Z",
  "logger": "worker",
  "caller": "worker/worker.go:143",
  "message": "Received Task",
  "job_name": "nginx-service-job"
}
```

## Project Structure
* main.go: Entry point handling command-line flags, retries, and signal interrupts.
* db/: Auto-generated sqlc code for type-safe PostgreSQL queries.
* repository/: Database connection pooling and repository interfaces.
* pkg/scheduler.go: Logic for hashing job definitions and publishing to RabbitMQ.
* pkg/watcher.go: Handles Git repository synchronization.
* pkg/worker/worker.go: Logic for consuming RabbitMQ messages, DLX routing, and Docker Engine API execution.
* types/types.go: Shared data structures for YAML job definitions.
* logger/logger.go: Configuration for the structured zap logger.

## Contributing
1. Fork the repository.
2. Create a feature branch.
3. Commit your changes.
4. Push to the branch.
5. Open a pull request.

# License
  MIT
