package job

import (
	"errors"
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type Job struct {
	ID         int       // job id
	Command    string    // job start command
	Status     Status    // job status
	Args       []string  // job's command args
	WorkingDir string    // dir where submit this job
	CreatedAt  time.Time // time when create this job
	StartedAt  time.Time // time when start this job
	FinishedAt time.Time // time when finish this job
	Error      string    // jobs error msg
	ExitCode   *int      // exit code
	LogPath    string    // path of log
	// Queue string // job queue
	// PID int  // job's pid
}

func (j *Job) MarkRunning(startedAt time.Time) error {
	if j.Status != StatusPending {
		return errors.New("only pending job can become running")
	}
	j.Status = StatusRunning
	j.StartedAt = startedAt
	return nil
}

func (j *Job) MarkSucceeded(finishedAt time.Time) error {
	if j.Status != StatusRunning {
		return errors.New("only running job can become succeeded")
	}
	j.Status = StatusSucceeded
	j.FinishedAt = finishedAt
	exitCode := 0
	j.ExitCode = &exitCode
	return nil
}

func (j *Job) MarkFailed(finishedAt time.Time, exitCode *int, errMsg string) error {
	if j.Status != StatusPending && j.Status != StatusRunning {
		return errors.New("only running and pending can become failed")
	}
	j.Status = StatusFailed
	j.FinishedAt = finishedAt
	j.ExitCode = exitCode
	j.Error = errMsg
	return nil
}
