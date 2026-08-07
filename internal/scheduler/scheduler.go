package scheduler

import (
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"searchmiddleware/internal/metadata"
	"searchmiddleware/internal/sync"
)

type Scheduler struct {
	cron    *cron.Cron
	meta    *metadata.DB
	engine  *sync.Engine
	indexes map[string]bool
}

func New(meta *metadata.DB, engine *sync.Engine, indexes map[string]bool) *Scheduler {
	return &Scheduler{
		cron:    cron.New(cron.WithSeconds()),
		meta:    meta,
		engine:  engine,
		indexes: indexes,
	}
}

func (s *Scheduler) Start() {
	s.ensureDefaults()
	s.loadSchedules()
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
	}
}

func (s *Scheduler) ensureDefaults() {
	for indexName := range s.indexes {
		var count int64
		s.meta.Model(&metadata.Schedule{}).Where("index_name = ?", indexName).Count(&count)
		if count > 0 {
			continue
		}

		s.meta.Create(&metadata.Schedule{
			IndexName: indexName,
			Type:      "incremental",
			CronExpr:  "*/5 * * * * *",
			Enabled:   true,
		})
		s.meta.Create(&metadata.Schedule{
			IndexName: indexName,
			Type:      "full",
			CronExpr:  "0 0 2 * * *",
			Enabled:   true,
		})
	}
}

func (s *Scheduler) loadSchedules() {
	var schedules []metadata.Schedule
	s.meta.Where("enabled = ?", true).Find(&schedules)

	for _, sch := range schedules {
		s.register(sch)
	}
}

func (s *Scheduler) register(sch metadata.Schedule) {
	spec := sch.CronExpr
	if sch.Type == "incremental" && spec == "" {
		spec = "*/5 * * * * *"
	}
	if sch.Type == "full" && spec == "" {
		spec = "0 0 2 * * *"
	}

	entryID, err := s.cron.AddFunc(spec, func() {
		s.run(sch)
	})
	if err != nil {
		log.Printf("register schedule failed: %s %s %v", sch.IndexName, sch.CronExpr, err)
		return
	}

	entry := s.cron.Entry(entryID)
	if entry.Schedule != nil {
		next := entry.Schedule.Next(time.Now())
		s.meta.Model(&metadata.Schedule{}).Where("id = ?", sch.ID).
			Update("next_run", next)
	}
}

func (s *Scheduler) run(sch metadata.Schedule) {
	now := time.Now()
	s.meta.Model(&metadata.Schedule{}).Where("id = ?", sch.ID).
		Updates(map[string]interface{}{"last_run": now})

	var err error
	switch sch.Type {
	case "full":
		err = s.engine.TriggerFullRebuild(sch.IndexName)
	case "incremental":
		err = s.engine.TriggerIncremental(sch.IndexName)
	default:
		err = s.engine.TriggerIncremental(sch.IndexName)
	}

	if err != nil && err.Error() != "sync already running for "+sch.IndexName {
		log.Printf("scheduled sync %s/%s failed: %v", sch.IndexName, sch.Type, err)
	}

	entry := s.cron.Entry(s.getEntryID(sch))
	if entry.Schedule != nil {
		next := entry.Schedule.Next(time.Now())
		s.meta.Model(&metadata.Schedule{}).Where("id = ?", sch.ID).Update("next_run", next)
	}
}

func (s *Scheduler) getEntryID(sch metadata.Schedule) cron.EntryID {
	for _, entry := range s.cron.Entries() {
		if entry.Job != nil {
			if j, ok := entry.Job.(cron.FuncJob); ok {
				_ = j
			}
		}
	}
	return 0
}

func (s *Scheduler) AddSchedule(sch metadata.Schedule) error {
	if err := s.meta.Create(&sch).Error; err != nil {
		return err
	}
	s.register(sch)
	return nil
}

func (s *Scheduler) RemoveSchedule(id uint) error {
	var sch metadata.Schedule
	if err := s.meta.First(&sch, id).Error; err != nil {
		return err
	}
	return s.meta.Delete(&sch).Error
}

func (s *Scheduler) ToggleSchedule(id uint, enabled bool) error {
	return s.meta.Model(&metadata.Schedule{}).Where("id = ?", id).Update("enabled", enabled).Error
}

func (s *Scheduler) Reload() {
	s.cron.Stop()
	s.cron = cron.New(cron.WithSeconds())
	s.loadSchedules()
	s.cron.Start()
}
