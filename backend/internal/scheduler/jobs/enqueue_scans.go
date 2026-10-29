package jobs

import (
	"context"
	"time"

	"app/internal/logger"
	"app/internal/scans"
)

// EnqueueScansJob queues the scan each enabled folder is due for. It is safe to run late or twice:
// a folder never gets a second scan while one is queued or running.
type EnqueueScansJob struct {
	service *scans.ScanService
	logger  *logger.Logger
}

func NewEnqueueScansJob(service *scans.ScanService, logger *logger.Logger) *EnqueueScansJob {
	return &EnqueueScansJob{service: service, logger: logger}
}

func (j *EnqueueScansJob) Execute(ctx context.Context) error {
	queued, err := j.service.EnqueueDue(ctx, time.Now())
	if queued > 0 {
		j.logger.InfoContext(ctx, "Queued scheduled scans", "scans", queued)
	}
	return err
}

func (j *EnqueueScansJob) Name() string {
	return "enqueue-scans"
}

func (j *EnqueueScansJob) Description() string {
	return "Queues a quick or full scan of each enabled folder whose interval has passed"
}
