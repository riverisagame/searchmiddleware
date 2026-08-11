package scheduler

import (
	stdsync "sync"
	"time"

	"github.com/robfig/cron/v3"

	"searchmiddleware/internal/logx"
	"searchmiddleware/internal/metadata"
	"searchmiddleware/internal/sync"
)

type Scheduler struct {
	cron     *cron.Cron
	meta     *metadata.DB
	engine   *sync.Engine
	indexes  map[string]bool
	mu       stdsync.Mutex
	entryIDs map[uint]cron.EntryID // schedule.ID → cron entryID（run 更新 next_run 用）
}

func New(meta *metadata.DB, engine *sync.Engine, indexes map[string]bool) *Scheduler {
	return &Scheduler{
		cron:     cron.New(cron.WithSeconds()),
		meta:     meta,
		engine:   engine,
		indexes:  indexes,
		entryIDs: make(map[uint]cron.EntryID),
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

	// 幂等：同 id 已有 entry 先移除（更新/重复注册场景）
	s.unregister(sch.ID)

	entryID, err := s.cron.AddFunc(spec, func() {
		s.run(sch)
	})
	if err != nil {
		logx.Errorf("scheduler", "register schedule failed: %s %s %v", sch.IndexName, sch.CronExpr, err)
		return
	}

	s.mu.Lock()
	s.entryIDs[sch.ID] = entryID
	s.mu.Unlock()

	entry := s.cron.Entry(entryID)
	if entry.Schedule != nil {
		next := entry.Schedule.Next(time.Now())
		s.meta.Model(&metadata.Schedule{}).Where("id = ?", sch.ID).
			Update("next_run", next)
	}
}

// unregister 移除 cron entry 并清理映射（幂等）
func (s *Scheduler) unregister(id uint) {
	s.mu.Lock()
	entryID, ok := s.entryIDs[id]
	delete(s.entryIDs, id)
	s.mu.Unlock()
	if ok {
		s.cron.Remove(entryID)
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
		logx.Errorf("scheduler", "scheduled sync %s/%s failed: %v", sch.IndexName, sch.Type, err)
	}

	entry := s.cron.Entry(s.getEntryID(sch))
	if entry.Schedule != nil {
		next := entry.Schedule.Next(time.Now())
		s.meta.Model(&metadata.Schedule{}).Where("id = ?", sch.ID).Update("next_run", next)
	}
}

func (s *Scheduler) getEntryID(sch metadata.Schedule) cron.EntryID {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.entryIDs[sch.ID]
}

func (s *Scheduler) AddSchedule(sch *metadata.Schedule) error {
	if err := s.meta.Create(sch).Error; err != nil {
		return err
	}
	s.register(*sch)
	return nil
}

func (s *Scheduler) RemoveSchedule(id uint) error {
	var sch metadata.Schedule
	if err := s.meta.First(&sch, id).Error; err != nil {
		return err
	}
	if err := s.meta.Delete(&sch).Error; err != nil {
		return err
	}
	s.unregister(id)
	return nil
}

// ToggleSchedule 启停：更新 DB + 同步 cron（停=移除 entry，启=重新注册）
func (s *Scheduler) ToggleSchedule(id uint, enabled bool) error {
	var sch metadata.Schedule
	if err := s.meta.First(&sch, id).Error; err != nil {
		return err
	}
	if err := s.meta.Model(&metadata.Schedule{}).Where("id = ?", id).Update("enabled", enabled).Error; err != nil {
		return err
	}
	if enabled {
		sch.Enabled = true
		s.register(sch)
	} else {
		s.unregister(id)
	}
	return nil
}

func (s *Scheduler) Reload() {
	s.cron.Stop()
	s.cron = cron.New(cron.WithSeconds())
	s.loadSchedules()
	s.cron.Start()
}
