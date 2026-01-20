package pkg

type JobTask struct {
    Name           string            `json:"name" yaml:"name"`
    Command        string            `json:"command" yaml:"command"`
    Args           []string          `json:"args" yaml:"args"`
    WorkDir        string            `json:"workdir" yaml:"workdir"`
    TimeoutSeconds int               `json:"timeout_seconds" yaml:"timeout_seconds"`
    Env            map[string]string `json:"env" yaml:"env"`
}