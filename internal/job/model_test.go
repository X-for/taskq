package job

import (
	"testing"
	"time"
)

// Test MarkRunning
func TestMarkRunning(t *testing.T) {
	startedAt := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)

	job := Job{
		Status: StatusPending,
	}

	err := job.MarkRunning(startedAt)
	if err != nil {
		t.Fatalf("MarkRunning() returned error: %v", err)
	}

	if job.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", job.Status, StatusRunning)
	}

	if !job.StartedAt.Equal(startedAt) {
		t.Errorf("StartedAt = %v, want %v", job.StartedAt, startedAt)
	}
}

func TestMarkRunningRejectsSucceededJob(t *testing.T) {
	finishedAt := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	job := Job{
		Status:     StatusSucceeded,
		FinishedAt: finishedAt,
	}

	err := job.MarkRunning(time.Now())

	if err == nil {
		t.Fatal("MarkRunning() returned nil, want an error")
	}

	if job.Status != StatusSucceeded {
		t.Errorf("Status changed to %q, want %q", job.Status, StatusSucceeded)
	}

	if job.FinishedAt != finishedAt {
		t.Errorf("FinishedAt = %q, want %q", job.FinishedAt, finishedAt)
	}
}

func TestMarkRunningRejectsFailedJob(t *testing.T) {
	finishedAt := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	job := Job{
		Status:     StatusFailed,
		FinishedAt: finishedAt,
	}
	err := job.MarkRunning(time.Now())
	if err == nil {
		t.Fatal("MarkRunning() returned nil, want an error")
	}
	if job.Status != StatusFailed {
		t.Errorf("Status = %q, want %q", job.Status, StatusFailed)
	}
	if job.FinishedAt != finishedAt {
		t.Errorf("FinishedAt = %q, want %q", job.FinishedAt, finishedAt)
	}
}

func TestMarkRunningRejectsRunningJob(t *testing.T) {
	finishedAt := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	job := Job{
		Status:     StatusRunning,
		FinishedAt: finishedAt,
	}
	err := job.MarkRunning(time.Now())
	if err == nil {
		t.Fatal("MarkRunning() returned nil, want an error")
	}
	if job.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", job.Status, StatusRunning)
	}
	if job.FinishedAt != finishedAt {
		t.Errorf("FinishedAt = %q, want %q", job.FinishedAt, finishedAt)
	}
}

func TestMarkSucceeded(t *testing.T) {
	job := Job{
		Status: StatusRunning,
	}
	finishedAt := time.Now()
	err := job.MarkSucceeded(finishedAt)

	if err != nil {
		t.Fatalf("MarkSucceeded() returned error: %v", err)
	}

	if job.Status != StatusSucceeded {
		t.Errorf("Status = %q, want %q", job.Status, StatusSucceeded)
	}

	if job.FinishedAt != finishedAt {
		t.Errorf("FinishedAt = %q, want %q", job.FinishedAt, finishedAt)
	}
	sc := 0
	var succeededCode *int = &sc
	if job.ExitCode != (succeededCode) {
		t.Errorf("ExitCode = %v, want %v", succeededCode, job.ExitCode)
	}
}

func TestMarkSucceededRejectsPendingJob(t *testing.T) {
	job := Job{
		Status: StatusPending,
	}
	finishedAt := time.Now()
	err := job.MarkSucceeded(finishedAt)
	if err == nil {
		t.Fatalf("MarkSucceeded() return nil, want an error")
	}
	if job.Status != StatusPending {
		t.Errorf("Status = %q, want %q", job.Status, StatusPending)
	}
	if job.FinishedAt.Equal(finishedAt) {
		t.Errorf("FinishedAt should not be %q", job.FinishedAt)
	}
}

func TestMarkSucceededRejectsSucceeded(t *testing.T) {
	succeededAt := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	job := Job{
		Status:     StatusSucceeded,
		FinishedAt: succeededAt,
	}
	finishedAt := time.Now()
	err := job.MarkSucceeded(finishedAt)
	if err == nil {
		t.Fatalf("MarkSucceeded() return nil, want an error")
	}
	if job.Status != StatusSucceeded {
		t.Errorf("Status = %q, want %q", job.Status, StatusSucceeded)

	}
	if job.FinishedAt != succeededAt {
		t.Errorf("FinishedAt = %q, want %q", job.FinishedAt, succeededAt)
	}
}

func TestMarkSucceededRejectsFailedJob(t *testing.T) {
	succeededAt := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	exitCode := 0
	job := Job{
		Status:     StatusFailed,
		FinishedAt: succeededAt,
		ExitCode:   &exitCode,
	}
	err := job.MarkSucceeded(time.Now())
	if err == nil {
		t.Fatal("MarkSucceeded() returned nil, want an error")
	}
	if job.Status != StatusFailed {
		t.Errorf("Status = %q, want %q", job.Status, StatusFailed)
	}
	if job.FinishedAt != succeededAt {
		t.Errorf("FinishedAt = %q, want %q", job.FinishedAt, succeededAt)
	}
}

func TestMarkFailedFromPendingJob(t *testing.T) {
	job := Job{
		Status: StatusPending,
	}
	var exitCode *int = nil
	finishedAt := time.Now()
	errMsg := "Run time error"
	err := job.MarkFailed(finishedAt, exitCode, errMsg)
	if err != nil {
		t.Fatalf("MarkFailed() returned error: %v", err)
	}
	if job.Status != StatusFailed {
		t.Errorf("Status = %q, want %q", job.Status, StatusFailed)
	}
	if job.FinishedAt != finishedAt {
		t.Errorf("FinishedAt = %v, want %v", job.FinishedAt, finishedAt)
	}
	if job.Error != errMsg {
		t.Errorf("Error = %q, want %q", job.Error, errMsg)
	}
	if job.ExitCode != nil {
		t.Errorf("ExitCode = %d, want %v", job.ExitCode, nil)
	}
}
func TestMarkFailedFromRunningJob(t *testing.T) {
	job := Job{
		Status: StatusRunning,
	}
	a := 1
	var exitCode *int = &a
	finishedAt := time.Now()
	errMsg := "Run time error"
	err := job.MarkFailed(finishedAt, exitCode, errMsg)
	if err != nil {
		t.Fatalf("MarkFailed() returned error: %v", err)
	}
	if job.Status != StatusFailed {
		t.Errorf("Status = %q, want %q", job.Status, StatusFailed)
	}
	if job.FinishedAt != finishedAt {
		t.Errorf("FinishedAt = %v, want %v", job.FinishedAt, finishedAt)
	}
	if job.Error != errMsg {
		t.Errorf("Error = %q, want %q", job.Error, errMsg)
	}
	if *job.ExitCode != a {
		t.Errorf("ExitCode = %d, want %d", job.ExitCode, a)
	}
}

func TestMarkFailedRejectsFailed(t *testing.T) {
	failedAt := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	a := 1
	job := Job{
		Status:     StatusFailed,
		FinishedAt: failedAt,
		ExitCode:   &a,
	}
	b := 2 // change it, make sure that we can catch error
	var exitCode *int = &b
	finishedAt := time.Now()
	err := job.MarkFailed(finishedAt, exitCode, "Run time error")
	if err == nil {
		t.Fatalf("MarkFailed() returned nil, want an error")
	}
	if job.Status != StatusFailed {
		t.Errorf("Status = %q, want %q", job.Status, StatusFailed)
	}
	if job.FinishedAt != failedAt {
		t.Errorf("FinishedAt = %v, want %v", job.FinishedAt, failedAt)
	}
	if *job.ExitCode != a {
		t.Errorf("ExitCode = %d, want %d", job.ExitCode, a)
	}
}

func TestMarkFailedRejectsSucceeded(t *testing.T) {
	succeededAt := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	a := 0
	job := Job{
		Status:     StatusSucceeded,
		FinishedAt: succeededAt,
		ExitCode:   &a,
	}
	b := 1
	var exitCode *int = &b
	finishedAt := time.Now()
	err := job.MarkFailed(finishedAt, exitCode, "Run time error")
	if err == nil {
		t.Fatalf("MarkFailed() returned nil, want an error")
	}
	if job.Status != StatusSucceeded {
		t.Errorf("Status = %q, want %q", job.Status, StatusSucceeded)
	}
	if job.FinishedAt != succeededAt {
		t.Errorf("FinishedAt = %v, want %v", job.FinishedAt, succeededAt)
	}
	if *job.ExitCode != a {
		t.Errorf("ExitCode = %d, want %d", job.ExitCode, a)
	}
}
