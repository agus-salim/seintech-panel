package system

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// JobStatus represents the current state of a background job.
type JobStatus string

const (
	JobPending  JobStatus = "pending"
	JobRunning  JobStatus = "running"
	JobDone     JobStatus = "done"
	JobFailed   JobStatus = "failed"
)

// Job holds the state of an asynchronous service operation.
type Job struct {
	ID          string    `json:"id"`
	Service     string    `json:"service"`
	Action      string    `json:"action"`
	Status      JobStatus `json:"status"`
	Message     string    `json:"message"`
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// JobStore manages background jobs in memory.
type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

var globalJobStore = &JobStore{jobs: make(map[string]*Job)}

// GetJobStore returns the global job store singleton.
func GetJobStore() *JobStore {
	return globalJobStore
}

// Create creates a new pending job and returns its ID.
func (js *JobStore) Create(service, action string) *Job {
	job := &Job{
		ID:        uuid.NewString(),
		Service:   service,
		Action:    action,
		Status:    JobPending,
		Message:   fmt.Sprintf("Preparing to %s %s...", action, service),
		CreatedAt: time.Now(),
	}
	js.mu.Lock()
	js.jobs[job.ID] = job
	js.mu.Unlock()
	return job
}

// Get retrieves a job by ID.
func (js *JobStore) Get(id string) (*Job, bool) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	j, ok := js.jobs[id]
	return j, ok
}

// Update updates a job's status and message.
func (js *JobStore) Update(id string, status JobStatus, message string) {
	js.mu.Lock()
	defer js.mu.Unlock()
	if j, ok := js.jobs[id]; ok {
		j.Status = status
		j.Message = message
		if status == JobDone || status == JobFailed {
			j.CompletedAt = time.Now()
		}
	}
}

// RunInstallJob starts the install operation asynchronously and tracks its state.
func (sm *ServiceManager) RunInstallJob(jobID string, serviceName string) {
	js := GetJobStore()
	js.Update(jobID, JobRunning, fmt.Sprintf("Installing %s via APT... this may take a few minutes.", serviceName))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	err := sm.InstallService(ctx, serviceName)
	if err != nil {
		js.Update(jobID, JobFailed, fmt.Sprintf("Installation failed: %v", err))
		return
	}
	js.Update(jobID, JobDone, fmt.Sprintf("%s installed and started successfully.", serviceName))
}

// RunUninstallJob starts the uninstall operation asynchronously and tracks its state.
func (sm *ServiceManager) RunUninstallJob(jobID string, serviceName string) {
	js := GetJobStore()
	js.Update(jobID, JobRunning, fmt.Sprintf("Removing %s via APT...", serviceName))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err := sm.UninstallService(ctx, serviceName)
	if err != nil {
		js.Update(jobID, JobFailed, fmt.Sprintf("Uninstall failed: %v", err))
		return
	}
	js.Update(jobID, JobDone, fmt.Sprintf("%s has been completely removed.", serviceName))
}
