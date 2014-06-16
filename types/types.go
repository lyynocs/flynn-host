package host

import (
	"encoding/json"
	"time"
)

type Job struct {
	ID string

	Attributes map[string]string

	Artifact Artifact
	// Quotas
	// - memory
	Type   string
	Config json.RawMessage
}

type Artifact struct {
	URL  string
	Type string
}

type Host struct {
	ID string

	// Currently running jobs
	Jobs []*Job
	// Host attributes
	Attributes map[string]string
}

type AddJobsReq struct {
	// map of host id -> new jobs
	HostJobs map[string][]*Job
}

type AddJobsRes struct {
	// The state of the cluster after the operation
	State map[string]Host
}

type Event struct {
	Event string
	JobID string
}

type ActiveJob struct {
	Job *Job

	ContainerID string
	Status      JobStatus
	StartedAt   time.Time
	EndedAt     time.Time
	ExitCode    int
	Error       *string
}

type AttachReq struct {
	JobID  string
	Flags  AttachFlag
	Height int
	Width  int
}

type AttachFlag uint8

const (
	AttachFlagStdout AttachFlag = 1 << iota
	AttachFlagStderr
	AttachFlagStdin
	AttachFlagLogs
	AttachFlagStream
)

type JobStatus uint8

func (s JobStatus) String() string {
	return map[JobStatus]string{
		StatusStarting: "starting",
		StatusRunning:  "running",
		StatusDone:     "done",
		StatusCrashed:  "crashed",
		StatusFailed:   "failed",
	}[s]
}

const (
	StatusStarting JobStatus = iota
	StatusRunning
	StatusDone
	StatusCrashed
	StatusFailed
)

const (
	AttachSuccess byte = iota
	AttachWaiting
	AttachError
)
