package jobs

import (
	"context"
	"time"

	"app/internal/logger"
	"app/internal/runs"
)

// EnqueueRunsJob queues the scheduled run of every enabled plan whose slot has come. It is safe
// to run late or twice: a slot is queued once, and after downtime only the latest missed slot.
type EnqueueRunsJob struct {
	service *runs.RunService
	logger  *logger.Logger
}

func NewEnqueueRunsJob(service *runs.RunService, logger *logger.Logger) *EnqueueRunsJob {
	return &EnqueueRunsJob{service: service, logger: logger}
}

func (j *EnqueueRunsJob) Execute(ctx context.Context) error {
	queued, err := j.service.EnqueueDue(ctx, time.Now())
	if queued > 0 {
		j.logger.InfoContext(ctx, "Queued scheduled runs", "runs", queued)
	}
	return err
}

func (j *EnqueueRunsJob) Name() string {
	return "enqueue-runs"
}

func (j *EnqueueRunsJob) Description() string {
	return "Queues the scheduled run of each enabled backup plan whose time has come"
}
