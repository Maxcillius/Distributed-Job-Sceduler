package types

import "context"

type JobTask struct {
	Name           string            `json:"name" yaml:"name"`
	Command        string            `json:"command" yaml:"command"`
	Args           []string          `json:"args" yaml:"args"`
	WorkDir        string            `json:"workdir" yaml:"workdir"`
	TimeoutSeconds int               `json:"timeout_seconds" yaml:"timeout_seconds"`
	Env            map[string]string `json:"env" yaml:"env"`
}

type JobDefinition struct {
	id     int
	name   string
	status string
}

type JobRepository interface {
	UpdateJobStatus(ctx context.Context, jobRunId int, status string) error
	IsJobActive(ctx context.Context, jobRunId int) (bool, error)
	GetJobDefinitionByHash(ctx context.Context, hash string) (JobDefinition, error)
	UpsertJobDefinition(ctx context.Context, job JobDefinition) error
}
