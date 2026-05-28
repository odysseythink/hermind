package agent_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type agentTestEnv struct {
	DB           *gorm.DB
	Cfg          *config.Config
	TempTokenSvc *services.TemporaryAuthTokenService
	AuthSvc      *services.AuthService
	Runtime      *agent.Runtime
	Server       *httptest.Server
	User         *models.User
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, services.AutoMigrate(db))
	return db
}

func seedAdminUser(t *testing.T, db *gorm.DB) *models.User {
	t.Helper()
	u := &models.User{
		Username: utils.Ptr("admin"),
		Password: "",
		Role:     "admin",
	}
	require.NoError(t, db.Create(u).Error)
	return u
}

func seedWorkspace(t *testing.T, db *gorm.DB) *models.Workspace {
	t.Helper()
	ws := &models.Workspace{
		Name: "Test Workspace",
		Slug: "test-workspace",
	}
	require.NoError(t, db.Create(ws).Error)
	return ws
}

func newAgentTestEnv(t *testing.T) *agentTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	cfg := &config.Config{StorageDir: t.TempDir()}
	enc, _ := utils.NewEncryptionManager("test-key")
	authSvc := services.NewAuthService(db, cfg, enc)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)
	rt := agent.NewRuntime(agent.Deps{
		DB: db, Cfg: cfg, TempTokenSvc: tempTokenSvc, AuthSvc: authSvc,
	})
	eng := gin.New()
	api := eng.Group("/api")
	// Register routes using the handlers package; we can't import handlers here
	// because it would create a cycle (handlers imports agent).
	// Instead we wire WS routes directly for e2e testing.
	api.GET("/agent-invocation/:uuid", func(c *gin.Context) {
		// bypass auth for direct e2e; auth is tested in middleware tests
		rt.HandleWS(c)
	})
	srv := httptest.NewServer(eng)
	t.Cleanup(srv.Close)
	return &agentTestEnv{
		DB:           db,
		Cfg:          cfg,
		TempTokenSvc: tempTokenSvc,
		AuthSvc:      authSvc,
		Runtime:      rt,
		Server:       srv,
		User:         seedAdminUser(t, db),
	}
}

func (e *agentTestEnv) IssueTempToken(t *testing.T, userID int, ttl time.Duration) string {
	t.Helper()
	tok, err := e.TempTokenSvc.IssueWithTTL(context.Background(), userID, ttl)
	require.NoError(t, err)
	return tok
}

func (e *agentTestEnv) DialWS(t *testing.T, path string, token string) (*websocket.Conn, *http.Response) {
	t.Helper()
	u, _ := url.Parse(e.Server.URL)
	u.Scheme = "ws"
	u.Path = path
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn, resp
}

func TestRuntime_CreateInvocation_ReturnsUUID(t *testing.T) {
	env := newAgentTestEnv(t)
	ws := seedWorkspace(t, env.DB)
	uid, err := env.Runtime.CreateInvocation(context.Background(), ws, env.User, nil, "@agent hello")
	require.NoError(t, err)
	_, err = uuid.Parse(uid)
	require.NoError(t, err, "must be a valid uuid v4")
}

func TestRuntime_GetInvocation_NotFound(t *testing.T) {
	env := newAgentTestEnv(t)
	inv, err := env.Runtime.GetInvocation(context.Background(), "nonexistent-uuid")
	require.ErrorIs(t, err, agent.ErrInvocationNotFound)
	require.Nil(t, inv)
}

func TestRuntime_GetInvocation_RejectsClosed(t *testing.T) {
	env := newAgentTestEnv(t)
	ws := seedWorkspace(t, env.DB)
	uid, err := env.Runtime.CreateInvocation(context.Background(), ws, env.User, nil, "@agent hello")
	require.NoError(t, err)
	err = env.Runtime.CloseInvocation(context.Background(), uid)
	require.NoError(t, err)
	inv, err := env.Runtime.GetInvocation(context.Background(), uid)
	require.ErrorIs(t, err, agent.ErrInvocationClosed)
	require.Nil(t, inv)
}

func TestRuntime_CloseInvocation_Idempotent(t *testing.T) {
	env := newAgentTestEnv(t)
	ws := seedWorkspace(t, env.DB)
	uid, err := env.Runtime.CreateInvocation(context.Background(), ws, env.User, nil, "@agent hello")
	require.NoError(t, err)
	err = env.Runtime.CloseInvocation(context.Background(), uid)
	require.NoError(t, err)
	err = env.Runtime.CloseInvocation(context.Background(), uid)
	require.NoError(t, err)
}

func TestRuntime_DeleteInvocation_RemovesRow(t *testing.T) {
	env := newAgentTestEnv(t)
	ws := seedWorkspace(t, env.DB)
	uid, err := env.Runtime.CreateInvocation(context.Background(), ws, env.User, nil, "@agent hello")
	require.NoError(t, err)
	err = env.Runtime.DeleteInvocation(context.Background(), uid)
	require.NoError(t, err)
	_, err = env.Runtime.GetInvocation(context.Background(), uid)
	require.ErrorIs(t, err, agent.ErrInvocationNotFound)
}

func TestRuntime_DeleteInvocation_IdempotentOnMissing(t *testing.T) {
	env := newAgentTestEnv(t)
	err := env.Runtime.DeleteInvocation(context.Background(), "nonexistent-uuid")
	require.NoError(t, err)
}
