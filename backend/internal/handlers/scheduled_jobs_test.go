package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/scheduler"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type immediateRunner struct{}

func (immediateRunner) RunOnce(_ context.Context, _ *models.ScheduledJob) (*scheduler.AgentRunResult, error) {
	return &scheduler.AgentRunResult{Text: "ok"}, nil
}

func newSJHandlerEnv(t *testing.T) (*gin.Engine, *gorm.DB, *services.ScheduledJobService, *scheduler.JobScheduler) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ScheduledJob{}, &models.ScheduledJobRun{}, &models.EventLog{}, &models.User{}, &models.WorkspaceUser{}, &models.Workspace{}, &models.WorkspaceThread{}, &models.WorkspaceChat{}))
	sjSvc := services.NewScheduledJobService(db)
	evt := services.NewEventLogService(db)
	sched := scheduler.NewJobScheduler(db, sjSvc, immediateRunner{}, evt,
		scheduler.Options{MaxConcurrent: 1, Timeout: 1 * time.Second})
	require.NoError(t, sched.Boot(context.Background()))
	authSvc := services.NewAuthService(db, &config.Config{JWTSecret: "t"}, nil)
	contSvc := services.NewScheduledJobContinueService(db, sjSvc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &models.User{ID: 1, Role: "admin"})
		c.Next()
	})
	api := r.Group("/api")
	RegisterScheduledJobsRoutes(api, sjSvc, sched, contSvc, authSvc)
	return r, db, sjSvc, sched
}

func TestScheduledJobs_CreateListGetDelete(t *testing.T) {
	r, _, sjSvc, _ := newSJHandlerEnv(t)

	body, _ := json.Marshal(map[string]any{
		"name": "weekly", "prompt": "do thing", "schedule": "0 9 * * 1",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/scheduled-jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var created struct{ Job models.ScheduledJob }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.NotZero(t, created.Job.ID)

	// LIST
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/scheduled-jobs", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// DELETE
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/scheduled-jobs/"+strconv.Itoa(created.Job.ID), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	_, err := sjSvc.Get(context.Background(), created.Job.ID)
	assert.Error(t, err)
}

func TestScheduledJobs_TriggerProducesRun(t *testing.T) {
	r, _, sjSvc, _ := newSJHandlerEnv(t)
	job, _ := sjSvc.Create(context.Background(), services.ScheduledJobInput{
		Name: "trigger", Prompt: "p", Schedule: "* * * * *",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		"/api/scheduled-jobs/"+strconv.Itoa(job.ID)+"/trigger", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	require.Eventually(t, func() bool {
		runs, _ := sjSvc.ListRuns(context.Background(), job.ID, 10, 0)
		return len(runs) > 0 && runs[0].Status == models.JobRunCompleted
	}, 2*time.Second, 50*time.Millisecond)
}

func TestScheduledJobs_InvalidCronReturns400(t *testing.T) {
	r, _, _, _ := newSJHandlerEnv(t)
	body, _ := json.Marshal(map[string]any{"name": "bad", "prompt": "p", "schedule": "🚫"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/scheduled-jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScheduledJobs_ContinueInThread_Endpoint(t *testing.T) {
	r, db, sjSvc, _ := newSJHandlerEnv(t)
	require.NoError(t, db.AutoMigrate(&models.WorkspaceThread{}))
	job, _ := sjSvc.Create(context.Background(), services.ScheduledJobInput{
		Name: "c", Prompt: "P", Schedule: "* * * * *",
	})
	run, _ := sjSvc.StartRun(context.Background(), job.ID)
	_ = sjSvc.Complete(context.Background(), run.ID, `{"text":"answer"}`)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		"/api/scheduled-jobs/runs/"+strconv.Itoa(run.ID)+"/continue", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}
