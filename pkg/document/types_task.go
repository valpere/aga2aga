package document

// TaskRequest asks an agent to perform a unit of work.
// Wire type: task.request.
type TaskRequest struct {
	Step string `yaml:"step"`
}

// TaskResult reports successful completion of a task.
// Wire type: task.result.
type TaskResult struct {
	Step string `yaml:"step"`
}

// TaskFail reports that a task could not be completed.
// Wire type: task.fail.
type TaskFail struct {
	Step  string `yaml:"step"`
	Error string `yaml:"error,omitempty"`
}

// TaskProgress reports partial completion of a long-running task.
// Wire type: task.progress.
type TaskProgress struct {
	Step    string  `yaml:"step"`
	Percent float64 `yaml:"percent,omitempty"`
}
