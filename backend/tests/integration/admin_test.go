package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/handlers"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupAdminRouter(t *testing.T) (*gin.Engine, *gorm.DB, *services.AuthService, *config.Config) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{StorageDir: t.TempDir(), JWTSecret: "test", AuthToken: "test-auth-token", MultiUserMode: true}
	db, err := services.NewDB(cfg)
	assert.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	assert.NoError(t, services.AutoMigrate(db))
	enc, _ := utils.NewEncryptionManager(cfg.StorageDir)
	authSvc := services.NewAuthService(db, cfg, enc)
	adminSvc := services.NewAdminService(db)
	sysSvc := services.NewSystemService(db)
	wsSvc := services.NewWorkspaceService(db, cfg)
	apiKeySvc := services.NewAPIKeyService(db)
	r := gin.New()
	handlers.RegisterAdminRoutes(r.Group("/api"), adminSvc, sysSvc, wsSvc, apiKeySvc, authSvc)
	return r, db, authSvc, cfg
}

// seedAdmin inserts an admin user and returns a valid JWT for that user.
func seedAdmin(t *testing.T, db *gorm.DB, cfg *config.Config) (*models.User, string) {
	t.Helper()
	hash, _ := utils.HashPassword("pw")
	u := &models.User{Username: utils.Ptr("root"), Password: hash, Role: "admin"}
	assert.NoError(t, db.Create(u).Error)
	tok, err := utils.GenerateJWT(cfg.JWTSecret, map[string]any{"userId": u.ID}, time.Hour)
	assert.NoError(t, err)
	return u, tok
}

func TestAdmin_CreateUserNew(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	_, tok := seedAdmin(t, db, cfg)

	body, _ := json.Marshal(map[string]any{
		"username": "newbie",
		"password": "Password123!",
		"role":     "default",
	})
	req, _ := http.NewRequest("POST", "/api/admin/users/new", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp struct {
		User  *models.User `json:"user"`
		Error *string      `json:"error"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp.User)
	assert.Nil(t, resp.Error)
	assert.Equal(t, "newbie", *resp.User.Username)
}

func TestAdmin_CreateUserNew_managerCannotMakeAdmin(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	hash, _ := utils.HashPassword("pw")
	mgr := &models.User{Username: utils.Ptr("mgr"), Password: hash, Role: "manager"}
	assert.NoError(t, db.Create(mgr).Error)
	tok, _ := utils.GenerateJWT(cfg.JWTSecret, map[string]any{"userId": mgr.ID}, time.Hour)

	body, _ := json.Marshal(map[string]any{
		"username": "evilroot", "password": "Password123!", "role": "admin",
	})
	req, _ := http.NewRequest("POST", "/api/admin/users/new", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp struct {
		User  any    `json:"user"`
		Error string `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Nil(t, resp.User)
	assert.Equal(t, "Invalid role selection for user.", resp.Error)
}

func TestAdmin_CreateUserNew_unauthorized(t *testing.T) {
	r, _, _, _ := setupAdminRouter(t)
	req, _ := http.NewRequest("POST", "/api/admin/users/new", bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
}

func TestAdmin_UpdateUser_byAdmin(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	_, tok := seedAdmin(t, db, cfg)
	// create a target user
	hash, _ := utils.HashPassword("pw")
	target := &models.User{Username: utils.Ptr("u1"), Password: hash, Role: "default"}
	assert.NoError(t, db.Create(target).Error)

	body, _ := json.Marshal(map[string]any{"role": "manager"})
	req, _ := http.NewRequest("POST", "/api/admin/user/"+itoa(target.ID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Success)

	var refreshed models.User
	db.First(&refreshed, target.ID)
	assert.Equal(t, "manager", refreshed.Role)
}

func TestAdmin_UpdateUser_managerCannotDemoteAdmin(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	hash, _ := utils.HashPassword("pw")
	mgr := &models.User{Username: utils.Ptr("mgr"), Password: hash, Role: "manager"}
	assert.NoError(t, db.Create(mgr).Error)
	adm := &models.User{Username: utils.Ptr("adm"), Password: hash, Role: "admin"}
	assert.NoError(t, db.Create(adm).Error)
	tok, _ := utils.GenerateJWT(cfg.JWTSecret, map[string]any{"userId": mgr.ID}, time.Hour)

	body, _ := json.Marshal(map[string]any{"role": "default"})
	req, _ := http.NewRequest("POST", "/api/admin/user/"+itoa(adm.ID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Cannot perform that action on user.", resp.Error)
}

func TestAdmin_UpdateUser_lastAdminLockout(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	soleAdmin, tok := seedAdmin(t, db, cfg)

	body, _ := json.Marshal(map[string]any{"role": "default"})
	req, _ := http.NewRequest("POST", "/api/admin/user/"+itoa(soleAdmin.ID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Error, "No system admins")
}

func TestAdmin_DeleteWorkspace_cascade(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	_, tok := seedAdmin(t, db, cfg)

	ws := &models.Workspace{Name: "Doomed", Slug: "doomed"}
	assert.NoError(t, db.Create(ws).Error)
	assert.NoError(t, db.Create(&models.WorkspaceChat{WorkspaceID: ws.ID}).Error)
	assert.NoError(t, db.Create(&models.WorkspaceDocument{WorkspaceID: ws.ID, Filename: "f.txt", DocId: "d1"}).Error)
	assert.NoError(t, db.Create(&models.DocumentVector{DocId: "d1", VectorId: "v1"}).Error)

	req, _ := http.NewRequest("DELETE", "/api/admin/workspaces/"+itoa(ws.ID), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp struct {
		Success bool `json:"success"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Success)

	// confirm all related rows gone
	var chatCount, docCount, wsCount, vecCount int64
	db.Model(&models.WorkspaceChat{}).Where("workspace_id = ?", ws.ID).Count(&chatCount)
	db.Model(&models.WorkspaceDocument{}).Where("workspace_id = ?", ws.ID).Count(&docCount)
	db.Model(&models.Workspace{}).Where("id = ?", ws.ID).Count(&wsCount)
	db.Model(&models.DocumentVector{}).Where("doc_id = ?", "d1").Count(&vecCount)
	assert.Zero(t, chatCount)
	assert.Zero(t, docCount)
	assert.Zero(t, wsCount)
	assert.Zero(t, vecCount)
}

func TestAdmin_DeleteWorkspace_notFound(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	_, tok := seedAdmin(t, db, cfg)
	_ = db

	req, _ := http.NewRequest("DELETE", "/api/admin/workspaces/99999", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 404, w.Code)
}

func TestAdmin_DeleteUser_byAdmin(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	_, tok := seedAdmin(t, db, cfg)
	hash, _ := utils.HashPassword("pw")
	target := &models.User{Username: utils.Ptr("v1"), Password: hash, Role: "default"}
	assert.NoError(t, db.Create(target).Error)

	req, _ := http.NewRequest("DELETE", "/api/admin/user/"+itoa(target.ID), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp struct {
		Success bool `json:"success"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Success)

	var stillThere models.User
	err := db.First(&stillThere, target.ID).Error
	assert.Error(t, err) // record not found
}

func TestAdmin_CreateWorkspace(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	root, tok := seedAdmin(t, db, cfg)

	body, _ := json.Marshal(map[string]any{"name": "ProjectX"})
	req, _ := http.NewRequest("POST", "/api/admin/workspaces/new", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp struct {
		Workspace *models.Workspace `json:"workspace"`
		Error     *string           `json:"error"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Workspace)
	assert.Nil(t, resp.Error)
	assert.Equal(t, "ProjectX", resp.Workspace.Name)

	// creator should be linked as admin
	var wu models.WorkspaceUser
	assert.NoError(t, db.Where("user_id = ? AND workspace_id = ?", root.ID, resp.Workspace.ID).First(&wu).Error)
	assert.Equal(t, "admin", wu.Role)
}

func TestAdmin_ListWorkspaceUsers(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	root, tok := seedAdmin(t, db, cfg)

	hash, _ := utils.HashPassword("pw")
	alice := &models.User{Username: utils.Ptr("alice"), Password: hash, Role: "default"}
	assert.NoError(t, db.Create(alice).Error)
	ws := &models.Workspace{Name: "W", Slug: "w-slug"}
	assert.NoError(t, db.Create(ws).Error)
	assert.NoError(t, db.Create(&models.WorkspaceUser{WorkspaceID: ws.ID, UserID: root.ID, Role: "admin"}).Error)
	assert.NoError(t, db.Create(&models.WorkspaceUser{WorkspaceID: ws.ID, UserID: alice.ID, Role: "default"}).Error)

	req, _ := http.NewRequest("GET", "/api/admin/workspaces/"+itoa(ws.ID)+"/users", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp struct {
		Users []map[string]any `json:"users"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Users, 2)
	// each entry has userId/username/role
	for _, u := range resp.Users {
		assert.Contains(t, u, "userId")
		assert.Contains(t, u, "username")
		assert.Contains(t, u, "role")
	}
}

func TestAdmin_UpdateWorkspaceUsers(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	_, tok := seedAdmin(t, db, cfg)

	hash, _ := utils.HashPassword("pw")
	u1 := &models.User{Username: utils.Ptr("u1"), Password: hash, Role: "default"}
	u2 := &models.User{Username: utils.Ptr("u2"), Password: hash, Role: "default"}
	u3 := &models.User{Username: utils.Ptr("u3"), Password: hash, Role: "default"}
	assert.NoError(t, db.Create(u1).Error)
	assert.NoError(t, db.Create(u2).Error)
	assert.NoError(t, db.Create(u3).Error)
	ws := &models.Workspace{Name: "W", Slug: "w-slug"}
	assert.NoError(t, db.Create(ws).Error)
	// Initially has u1 and u2
	assert.NoError(t, db.Create(&models.WorkspaceUser{WorkspaceID: ws.ID, UserID: u1.ID, Role: "default"}).Error)
	assert.NoError(t, db.Create(&models.WorkspaceUser{WorkspaceID: ws.ID, UserID: u2.ID, Role: "default"}).Error)

	body, _ := json.Marshal(map[string]any{"userIds": []int{u2.ID, u3.ID}})
	req, _ := http.NewRequest("POST", "/api/admin/workspaces/"+itoa(ws.ID)+"/update-users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Success)

	var ids []int
	db.Model(&models.WorkspaceUser{}).Where("workspace_id = ?", ws.ID).Pluck("user_id", &ids)
	assert.ElementsMatch(t, []int{u2.ID, u3.ID}, ids)
}

func TestAdmin_DeleteUser_managerCannotDeleteAdmin(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	hash, _ := utils.HashPassword("pw")
	mgr := &models.User{Username: utils.Ptr("mgr"), Password: hash, Role: "manager"}
	assert.NoError(t, db.Create(mgr).Error)
	adm := &models.User{Username: utils.Ptr("adm"), Password: hash, Role: "admin"}
	assert.NoError(t, db.Create(adm).Error)
	tok, _ := utils.GenerateJWT(cfg.JWTSecret, map[string]any{"userId": mgr.ID}, time.Hour)

	req, _ := http.NewRequest("DELETE", "/api/admin/user/"+itoa(adm.ID), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Cannot perform that action on user.", resp.Error)
}

func TestAdmin_APIKey_GenerateListDelete(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	_, tok := seedAdmin(t, db, cfg)
	_ = db

	// 1. generate
	body, _ := json.Marshal(map[string]any{"name": "ci-key"})
	req, _ := http.NewRequest("POST", "/api/admin/generate-api-key", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var genResp struct {
		APIKey *models.APIKey `json:"apiKey"`
		Error  *string        `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &genResp)
	assert.NotNil(t, genResp.APIKey)
	assert.NotNil(t, genResp.APIKey.Secret)

	// 2. list
	req, _ = http.NewRequest("GET", "/api/admin/api-keys", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var listResp struct {
		APIKeys []map[string]any `json:"apiKeys"`
		Error   any              `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &listResp)
	assert.Len(t, listResp.APIKeys, 1)
	createdBy := listResp.APIKeys[0]["createdBy"]
	assert.NotNil(t, createdBy)
	cb, ok := createdBy.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "root", cb["username"])

	// 3. delete
	id := int(listResp.APIKeys[0]["id"].(float64))
	req, _ = http.NewRequest("DELETE", "/api/admin/delete-api-key/"+itoa(id), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Empty(t, w.Body.String()) // Node returns empty 200 body

	// 4. confirm gone
	req, _ = http.NewRequest("GET", "/api/admin/api-keys", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &listResp)
	assert.Len(t, listResp.APIKeys, 0)
}

func TestAdmin_APIKey_managerForbidden(t *testing.T) {
	r, db, _, cfg := setupAdminRouter(t)
	hash, _ := utils.HashPassword("pw")
	mgr := &models.User{Username: utils.Ptr("mgr"), Password: hash, Role: "manager"}
	assert.NoError(t, db.Create(mgr).Error)
	tok, _ := utils.GenerateJWT(cfg.JWTSecret, map[string]any{"userId": mgr.ID}, time.Hour)

	req, _ := http.NewRequest("GET", "/api/admin/api-keys", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code)
}

func itoa(i int) string { return fmt.Sprintf("%d", i) }
