package repository

import (
	"context"

	"github.com/maxcillius/Distributed-Job-Scheduler/db"
)

type JobRepository interface {
	UpdateJobStatus(ctx context.Context, jobRunId string, status string) error
	IsJobActive(ctx context.Context, jobRunId string) (bool, error)
	UpsertJobDefinition(ctx context.Context, job db.InsertJobParams) error
}
