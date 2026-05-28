package services

import (
	"context"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"gorm.io/gorm"
)

const MaxLevenshteinDistance = 3

type SearchService struct {
	db *gorm.DB
}

func NewSearchService(db *gorm.DB) *SearchService {
	return &SearchService{db: db}
}

func (s *SearchService) SearchWorkspaceAndThreads(ctx context.Context, searchTerm string, userID *int) (*dto.SearchResults, error) {
	searchTerm = strings.TrimSpace(searchTerm)
	if len(searchTerm) < 3 {
		return &dto.SearchResults{Workspaces: []dto.WorkspaceSearchResult{}, Threads: []dto.ThreadSearchResult{}}, nil
	}
	term := strings.ToLower(searchTerm)
	likePattern := "%" + utils.EscapeLike(searchTerm) + "%"

	var workspaces []models.Workspace
	wsQuery := s.db.WithContext(ctx)
	if userID != nil {
		wsQuery = wsQuery.Joins("JOIN workspace_users ON workspace_users.workspace_id = workspaces.id").
			Where("workspace_users.user_id = ?", *userID)
	}
	// DB-level LIKE pre-filter to avoid loading entire table
	wsQuery = wsQuery.Where("LOWER(name) LIKE ?", likePattern)
	if err := wsQuery.Find(&workspaces).Error; err != nil {
		return nil, err
	}

	var threads []models.WorkspaceThread
	threadQuery := s.db.WithContext(ctx).Preload("Workspace")
	if userID != nil {
		threadQuery = threadQuery.Where("user_id = ?", *userID)
	}
	threadQuery = threadQuery.Where("LOWER(name) LIKE ?", likePattern)
	if err := threadQuery.Find(&threads).Error; err != nil {
		return nil, err
	}

	workspaceSet := make(map[string]dto.WorkspaceSearchResult)
	threadSet := make(map[string]dto.ThreadSearchResult)

	for _, ws := range workspaces {
		name := strings.ToLower(ws.Name)
		if matchesSearch(name, term) {
			workspaceSet[ws.Slug] = dto.WorkspaceSearchResult{Slug: ws.Slug, Name: ws.Name}
		}
	}

	for _, th := range threads {
		name := strings.ToLower(th.Name)
		if matchesSearch(name, term) {
			wsSlug := ""
			wsName := ""
			if th.Workspace != nil {
				wsSlug = th.Workspace.Slug
				wsName = th.Workspace.Name
			}
			key := th.Slug + "@" + wsSlug
			threadSet[key] = dto.ThreadSearchResult{
				Slug:      th.Slug,
				Name:      th.Name,
				Workspace: dto.WorkspaceSearchResult{Slug: wsSlug, Name: wsName},
			}
		}
	}

	results := &dto.SearchResults{
		Workspaces: make([]dto.WorkspaceSearchResult, 0, len(workspaceSet)),
		Threads:    make([]dto.ThreadSearchResult, 0, len(threadSet)),
	}
	for _, ws := range workspaceSet {
		results.Workspaces = append(results.Workspaces, ws)
	}
	for _, th := range threadSet {
		results.Threads = append(results.Threads, th)
	}
	return results, nil
}

func matchesSearch(name, term string) bool {
	// strings.Contains already covers prefix and suffix
	if strings.Contains(name, term) {
		return true
	}
	return utils.Levenshtein(name, term) <= MaxLevenshteinDistance
}
