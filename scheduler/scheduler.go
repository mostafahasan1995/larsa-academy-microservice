package scheduler

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/samber/do"
)

var Cron *cron.Cron

type Scheduler struct {
	renewalJob  *RenewalJob
	reminderJob *ReminderJob
}

func NewScheduler(i *do.Injector) (*Scheduler, error) {
	locker := NewLocker(i)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := locker.EnsureIndexes(ctx); err != nil {
		return nil, err
	}
	return &Scheduler{
		renewalJob:  NewRenewalJob(i, locker),
		reminderJob: NewReminderJob(i, locker),
	}, nil
}

func (s *Scheduler) Start() {
	Cron = cron.New(cron.WithSeconds())

	Cron.AddJob("0 0 */6 * * *", s.renewalJob)
	// Daily at 08:00 UTC: session and placement test reminders
	Cron.AddJob("0 0 8 * * *", s.reminderJob)
	Cron.Start()

}

func (s *Scheduler) Stop() {
	ctx := Cron.Stop()
	<-ctx.Done()
}
