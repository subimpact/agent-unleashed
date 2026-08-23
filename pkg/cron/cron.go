package cron

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"agent-unleashed/pkg/engine"
)

type CronJob struct {
	ID        string    `json:"id"`
	Schedule  string    `json:"schedule"` // 5-field cron: "min hour dom mon dow" (e.g. "0 9 * * *")
	Prompt    string    `json:"prompt"`
	Channel   string    `json:"channel"`   // 'telegram', 'discord', 'log'
	TargetID  string    `json:"target_id"` // Chat ID or Channel ID
	Enabled   bool      `json:"enabled"`
	LastRun   time.Time `json:"last_run,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type DispatcherFunc func(channel, targetID, response string)

type CronEngine struct {
	db         *sql.DB
	engine     *engine.UnleashedEngine
	dispatcher DispatcherFunc
	jobs       map[string]*CronJob
	mu         sync.RWMutex
}

func NewCronEngine(db *sql.DB, eng *engine.UnleashedEngine, dispatcher DispatcherFunc) (*CronEngine, error) {
	ce := &CronEngine{
		db:         db,
		engine:     eng,
		dispatcher: dispatcher,
		jobs:       make(map[string]*CronJob),
	}

	if err := ce.initDB(); err != nil {
		return nil, err
	}
	if err := ce.loadJobs(); err != nil {
		return nil, err
	}

	return ce, nil
}

func (c *CronEngine) initDB() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	query := `
	CREATE TABLE IF NOT EXISTS cron_jobs (
		id TEXT PRIMARY KEY,
		schedule TEXT NOT NULL,
		prompt TEXT NOT NULL,
		channel TEXT NOT NULL DEFAULT 'log',
		target_id TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 1,
		last_run TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := c.db.Exec(query)
	return err
}

func (c *CronEngine) loadJobs() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	rows, err := c.db.Query("SELECT id, schedule, prompt, channel, target_id, enabled, created_at, last_run FROM cron_jobs")
	if err != nil {
		return err
	}
	defer rows.Close()

	c.jobs = make(map[string]*CronJob)
	for rows.Next() {
		var job CronJob
		var enabledInt int
		var createdStr string
		var lastRunStr sql.NullString
		if err := rows.Scan(&job.ID, &job.Schedule, &job.Prompt, &job.Channel, &job.TargetID, &enabledInt, &createdStr, &lastRunStr); err == nil {
			job.Enabled = (enabledInt == 1)
			job.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdStr)
			// Restoring last_run stops a job whose minute happens to match at
			// startup from firing again after every restart.
			if lastRunStr.Valid && lastRunStr.String != "" {
				job.LastRun, _ = time.Parse("2006-01-02 15:04:05", lastRunStr.String)
			}
			c.jobs[job.ID] = &job
		}
	}
	return nil
}

func (c *CronEngine) AddJob(schedule, prompt, channel, targetID string) (*CronJob, error) {
	if err := validateCron(schedule); err != nil {
		return nil, fmt.Errorf("invalid cron expression %q: %v (e.g. '0 9 * * *', '*/15 * * * *', '0 9 * * mon-fri')", schedule, err)
	}

	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", schedule, prompt, time.Now().UnixNano())))
	id := hex.EncodeToString(hash[:4])

	c.mu.Lock()
	defer c.mu.Unlock()

	query := `INSERT INTO cron_jobs (id, schedule, prompt, channel, target_id, enabled) VALUES (?, ?, ?, ?, ?, 1)`
	if _, err := c.db.Exec(query, id, schedule, prompt, channel, targetID); err != nil {
		return nil, err
	}

	job := &CronJob{
		ID:        id,
		Schedule:  schedule,
		Prompt:    prompt,
		Channel:   channel,
		TargetID:  targetID,
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	c.jobs[id] = job
	return job, nil
}

func (c *CronEngine) RemoveJob(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := c.db.Exec("DELETE FROM cron_jobs WHERE id = ?", id); err != nil {
		return err
	}
	delete(c.jobs, id)
	return nil
}

func (c *CronEngine) ListJobs() []*CronJob {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Copies: the scheduler mutates LastRun on the stored jobs, so live
	// pointers would let a reader observe a half-written value.
	list := make([]*CronJob, 0, len(c.jobs))
	for _, j := range c.jobs {
		snapshot := *j
		list = append(list, &snapshot)
	}
	sort.Slice(list, func(i, k int) bool { return list[i].ID < list[k].ID })
	return list
}

func (c *CronEngine) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	log.Println("[Cron Engine] 24/7 Background Cron & Automation Engine active.")

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			c.checkAndRun(ctx, t)
		}
	}
}

func (c *CronEngine) checkAndRun(ctx context.Context, now time.Time) {
	// Claim due jobs under the write lock. Stamping LastRun outside it raced
	// with ListJobs, and both ticks inside one minute could claim the same job.
	c.mu.Lock()
	var toRun []CronJob
	for _, job := range c.jobs {
		if job.Enabled && matchesCron(job.Schedule, now) {
			// Avoid double-execution within the same minute
			if now.Sub(job.LastRun) > 50*time.Second {
				job.LastRun = now
				toRun = append(toRun, *job)
			}
		}
	}
	c.mu.Unlock()

	for i := range toRun {
		job := toRun[i]
		go c.executeJob(ctx, &job)
	}
}

func (c *CronEngine) executeJob(ctx context.Context, job *CronJob) {
	log.Printf("[Cron Engine] ⚡ Triggering scheduled task [%s]: %s\n", job.ID, job.Prompt)

	sessionID := fmt.Sprintf("cron_%s", job.ID)
	events := make(chan engine.Event)

	go c.engine.Chat(ctx, sessionID, job.Prompt, "cron", events)

	var fullResp strings.Builder
	for ev := range events {
		if ev.Type == engine.EventText {
			fullResp.WriteString(ev.Content)
		}
	}

	resultText := fullResp.String()
	if resultText == "" {
		resultText = "[Scheduled task completed with empty output]"
	}

	// Dispatch to channel
	if c.dispatcher != nil {
		c.dispatcher(job.Channel, job.TargetID, fmt.Sprintf("⏰ **[Automated Scheduled Task: %s]**\n\n%s", job.ID, resultText))
	}

	// Update DB last run
	_, _ = c.db.Exec("UPDATE cron_jobs SET last_run = CURRENT_TIMESTAMP WHERE id = ?", job.ID)
}
