package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/odysseythink/hermind/backend/internal/agent/flow"
	"github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
	"github.com/odysseythink/hermind/backend/internal/chunker"
	"github.com/odysseythink/hermind/backend/internal/collector"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/embedder"
	"github.com/odysseythink/hermind/backend/internal/handlers"
	"github.com/odysseythink/hermind/backend/internal/mcp"
	"github.com/odysseythink/hermind/backend/internal/providers"
	"github.com/odysseythink/hermind/backend/internal/reranker"
	"github.com/odysseythink/hermind/backend/internal/scheduler"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/internal/tts"
	"github.com/unidoc/unioffice/common/license"
	"github.com/odysseythink/hermind/backend/internal/vectordb"
	"github.com/odysseythink/hermind/backend/internal/workers"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/odysseythink/mlog"
)

type spaFileSystem struct {
	fs http.FileSystem
}

func (s *spaFileSystem) Open(name string) (http.File, error) {
	f, err := s.fs.Open(name)
	if err != nil {
		if f2, err2 := s.fs.Open("index.html"); err2 == nil {
			return f2, nil
		}
		return s.fs.Open("_index.html")
	}
	return f, nil
}

func main() {
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logDir := cfg.StorageDir + "/logs"
	os.MkdirAll(logDir, 0755)
	utils.InitLogger(logDir)
	defer utils.SyncLogger()

	if f := flag.Lookup("logtostderr"); f != nil && f.Value.String() == "true" {
		mlog.SetLogDir("")
	}

	enc, err := utils.NewEncryptionManager(cfg.StorageDir)
	if err != nil {
		mlog.Fatal("failed to init encryption", mlog.Err(err))
	}

	db, err := services.NewDB(cfg)
	if err != nil {
		mlog.Fatal("failed to connect db", mlog.Err(err))
	}
	if err := services.AutoMigrate(db); err != nil {
		mlog.Fatal("failed to migrate db", mlog.Err(err))
	}
	if err := services.SeedDefaults(db); err != nil {
		mlog.Fatal("failed to seed db", mlog.Err(err))
	}

	// unioffice license (pptx generation)
	if cfg.UnidocMeteredKey != "" {
		if err := license.SetMeteredKey(cfg.UnidocMeteredKey); err != nil {
			mlog.Warning("failed to set unioffice metered key", mlog.Err(err))
		}
	}

	authSvc := services.NewAuthService(db, cfg, enc)
	eventLogSvc := services.NewEventLogService(db)
	tempTokenSvc := services.NewTemporaryAuthTokenService(db)
	sysSvc := services.NewSystemService(db)
	webpushSvc := services.NewWebPushService(db, sysSvc, enc,
		services.WebPushOptions{
			MailTo: cfg.WebPushMailTo,
			TTL:    cfg.WebPushTTLSeconds,
		})
	if err := webpushSvc.Init(context.Background()); err != nil {
		mlog.Fatal("webpush init failed", mlog.Err(err))
	}
	webpushSvc.Boot(eventLogSvc)
	phSvc := services.NewPromptHistoryService(db)
	wsSvc := services.NewWorkspaceService(db, cfg, phSvc)
	searchSvc := services.NewSearchService(db)
	vectorSvc := services.NewVectorService(cfg)
	dbSettings, _ := sysSvc.GetAllSettings(context.Background())
	llmProv := providers.NewLLMProvider(cfg, dbSettings)
	coll, err := collector.NewLocalCollector(cfg.StorageDir)
	if err != nil {
		mlog.Warning("collector client init failed, continuing without collector", mlog.Err(err))
	}

	ch := chunker.NewChunker(1000, 20, "")
	var emb embedder.Embedder
	if e, err := embedder.NewEmbedder(cfg, dbSettings); err == nil {
		emb = e
	} else {
		mlog.Warning("embedder init failed, continuing without embedding support", mlog.Err(err))
	}

	var rerankerSvc reranker.Reranker
	if r, err := reranker.NewReranker(cfg, dbSettings); err == nil {
		rerankerSvc = r
	} else {
		mlog.Warning("reranker init failed, continuing without reranking", mlog.Err(err))
		rerankerSvc = &reranker.NoopReranker{}
	}

	var vectorSearchSvc *services.VectorSearchService
	if emb != nil {
		vectorSearchSvc = services.NewVectorSearchService(vectorSvc, emb, rerankerSvc)
	}

	embedSvc := services.NewEmbedService(db, cfg, vectorSvc, llmProv, emb, rerankerSvc)

	var vectorDB vectordb.VectorDatabase
	switch cfg.VectorDB {
	case "pgvector":
		if cfg.DatabaseURL == "" {
			mlog.Warning("pgvector configured but DATABASE_URL is empty")
		} else {
			pgv := vectordb.NewPGVector(cfg.DatabaseURL)
			if err := pgv.Connect(context.Background()); err != nil {
				mlog.Warning("pgvector connect failed: ", err)
			} else {
				vectorDB = pgv
				vectorSvc.SetProvider(pgv)
			}
		}
	case "lancedb":
		ldb := vectordb.NewLanceDB(cfg.StorageDir)
		if err := ldb.Connect(nil); err != nil {
			mlog.Warning("lancedb connect failed: ", err)
		} else {
			vectorDB = ldb
			vectorSvc.SetProvider(ldb)
		}
	}

	fsSvc := services.NewFileSystemService(cfg.StorageDir)
	docSvc := services.NewDocumentService(db, cfg, coll, emb, ch, vectorDB, fsSvc)
	adminSvc := services.NewAdminService(db)
	threadSvc := services.NewThreadService(db)
	agentFlowSvc := services.NewAgentFlowService(cfg.StorageDir)
	apiKeySvc := services.NewAPIKeyService(db)
	promptPresetSvc := services.NewPromptPresetService(db)
	promptVariableSvc := services.NewPromptVariableService(db)
	wsChatSvc := services.NewWorkspaceChatService(db)
	mcpHyp := mcp.Instance(cfg)
	if err := mcpHyp.Boot(context.Background()); err != nil {
		mlog.Warning("mcp boot warning", mlog.Err(err))
	}
	mcpSvc := services.NewMCPService(mcpHyp)
	bridgeClient := oauth.NewBridgeClient(30 * time.Second)
	tokenStore := oauth.NewTokenStore(db, enc)
	outlookOAuth := oauth.NewOutlookOAuth(tokenStore, cfg.PublicBaseURL, cfg.OutlookAuthority, nil)
	stateSecret := []byte(cfg.JWTSecret)
	oauthHandler := handlers.NewOAuthHandler(outlookOAuth, tokenStore, sysSvc, enc, stateSecret, cfg.PublicBaseURL)
	whitelistSvc := services.NewAgentSkillWhitelistService(sysSvc)
	flowExec := flow.New(llmProv.LanguageModel(), cfg.AgentFlowAllowPrivateIPs)
	agentRuntime := agent.NewRuntime(agent.Deps{
		DB:              db,
		Cfg:             cfg,
		TempTokenSvc:    tempTokenSvc,
		AuthSvc:         authSvc,
		SysSvc:          sysSvc,
		VectorSearchSvc: vectorSearchSvc,
		DocSvc:          docSvc,
		MCPHv:           mcpHyp,
		FlowSvc:         agentFlowSvc,
		FlowExecutor:    flowExec,
		EventLog:        eventLogSvc,
		Bridge:          bridgeClient,
		OutlookOAuth:    outlookOAuth,
		OutlookStore:    tokenStore,
		WhitelistSvc:    whitelistSvc,
	})
	sjSvc := services.NewScheduledJobService(db)
	agentRunner := scheduler.NewRuntimeAgentRunner(agentRuntime, eventLogSvc)
	sched := scheduler.NewJobScheduler(db, sjSvc, agentRunner, eventLogSvc, scheduler.Options{
		MaxConcurrent: cfg.SchedJobMaxConcurrent,
		Timeout:       time.Duration(cfg.SchedJobTimeoutMS) * time.Millisecond,
	})
	if err := sched.Boot(context.Background()); err != nil {
		mlog.Fatal("scheduler boot failed", mlog.Err(err))
	}

	contSvc := services.NewScheduledJobContinueService(db, sjSvc)

	memSvc := services.NewMemoryService(db)
	memInj := services.NewMemoryInjector(memSvc, sysSvc, rerankerSvc)
	memExt := services.NewMemoryExtractor(memSvc, llmProv.LanguageModel(), "", "")

	workerMgr := workers.NewManager(db, cfg)
	workerMgr.Register(
		workers.NewCleanupOrphanJob(db, cfg),
		workers.NewCleanupGeneratedJob(db, cfg),
		workers.NewSyncWatchedJob(db, cfg, coll),
		workers.NewEmbedWorkerJob(db, cfg, emb, vectorDB),
		workers.NewExtractMemoriesJob(db, memSvc, memExt, sysSvc),
	)
	if err := workerMgr.Start(); err != nil {
		mlog.Fatal("failed to start worker manager", mlog.Err(err))
	}
	chatSvc := services.NewChatService(db, cfg, vectorSvc, llmProv, emb, agentRuntime, rerankerSvc, memInj)
	progressMgr := services.NewEmbeddingProgressManager()

	// Boot migration: encrypt plaintext secrets in SystemSetting
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		keys := []struct{ setting, field string }{
			{"outlook_agent_config", "clientSecret"},
			{"gmail_agent_config", "apiKey"},
			{"google_calendar_agent_config", "apiKey"},
		}
		for _, k := range keys {
			raw, err := sysSvc.GetSetting(ctx, k.setting)
			if err != nil || raw == "" {
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal([]byte(raw), &obj); err != nil {
				continue
			}
			v, _ := obj[k.field].(string)
			if v == "" || strings.HasPrefix(v, services.EncryptedPrefix) {
				continue
			}
			if err := sysSvc.SetSecretField(ctx, k.setting, k.field, v, enc); err != nil {
				mlog.Warning("migrate ", k.setting, ".", k.field, ": ", err)
				continue
			}
			mlog.Info("migrated ", k.setting, ".", k.field, " to encrypted form")
		}
	}()

	ttsProvider := tts.NewProvider(cfg, dbSettings)
	ttsHandler := handlers.NewTTSHandler(chatSvc, ttsProvider)

	if !cfg.DebugMode {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api")
	{
		handlers.RegisterSystemRoutes(api, sysSvc, apiKeySvc, cfg, authSvc, adminSvc, fsSvc, coll, vectorSvc, promptPresetSvc, promptVariableSvc, wsChatSvc)
		handlers.RegisterAuthRoutes(api, authSvc, cfg, eventLogSvc, tempTokenSvc)
		handlers.RegisterWorkspaceRoutes(api, wsSvc, authSvc, db, searchSvc, vectorSearchSvc, docSvc, progressMgr)
		handlers.RegisterPromptHistoryRoutes(api, phSvc, wsSvc, authSvc, db)
		handlers.RegisterChatRoutes(api, chatSvc, authSvc, db)
		handlers.RegisterTTSRoutes(api, ttsHandler, authSvc)
		handlers.RegisterDocumentRoutes(api, docSvc, authSvc)
		handlers.RegisterAdminRoutes(api, adminSvc, sysSvc, wsSvc, apiKeySvc, authSvc)
		handlers.RegisterThreadRoutes(api, threadSvc, authSvc, db)
		handlers.RegisterAgentFlowRoutes(api, agentFlowSvc, authSvc)
		handlers.RegisterAgentSkillRoutes(api, sysSvc, authSvc, cfg)
		handlers.RegisterMCPRoutes(api, authSvc, mcpSvc, eventLogSvc, cfg)
		handlers.RegisterAgentTokenRoutes(api, tempTokenSvc, authSvc)
		handlers.RegisterAgentRoutes(api, agentRuntime, authSvc, tempTokenSvc)
		handlers.RegisterOAuthRoutes(api, oauthHandler, authSvc)
		whitelistHandler := handlers.NewAgentSkillWhitelistHandler(whitelistSvc)
		handlers.RegisterAgentSkillWhitelistRoutes(api, whitelistHandler, authSvc)
		handlers.RegisterTelegramRoutes(api, cfg, authSvc)
		handlers.RegisterBrowserExtensionRoutes(api, authSvc)
		handlers.RegisterEmbedRoutes(api, embedSvc, db)
		handlers.RegisterEmbedManagementRoutes(api, embedSvc, authSvc, db)
		handlers.RegisterAPIEmbedRoutes(api, embedSvc, apiKeySvc, db)
		handlers.RegisterScheduledJobsRoutes(api, sjSvc, sched, contSvc, authSvc)
		handlers.RegisterMemoryRoutes(api, memSvc, wsSvc, authSvc)
		handlers.RegisterWebPushRoutes(api, webpushSvc, authSvc)

		// API v1 routes (API key auth)
		handlers.RegisterAPIAuthRoutes(api, apiKeySvc)
		handlers.RegisterAPIUserRoutes(api, apiKeySvc, adminSvc, tempTokenSvc, cfg)
		handlers.RegisterAPIAdminRoutes(api, apiKeySvc, adminSvc, sysSvc, wsSvc, wsChatSvc, cfg)
		apiSystemHandler := handlers.NewSystemHandler(sysSvc, apiKeySvc, adminSvc, authSvc, cfg, fsSvc, coll, vectorSvc, promptPresetSvc, promptVariableSvc, wsChatSvc)
		handlers.RegisterAPISystemRoutes(api, apiKeySvc, sysSvc, vectorSvc, docSvc, wsChatSvc, apiSystemHandler)

		// PR4: workspace / document / thread v1 routes
		handlers.RegisterAPIWorkspaceRoutes(api, apiKeySvc, wsSvc, chatSvc, vectorSearchSvc, docSvc, db)
		handlers.RegisterAPIDocumentRoutes(api, apiKeySvc, docSvc, fsSvc, progressMgr)
		handlers.RegisterAPIThreadRoutes(api, apiKeySvc, threadSvc, chatSvc, db)

		// PR5: OpenAI compat layer
		handlers.RegisterAPIOpenAIRoutes(api, apiKeySvc, wsSvc, chatSvc, threadSvc, emb, db, cfg)
	}

	frontendFS, err := fs.Sub(FrontendFS, "frontend/dist")
	if err != nil {
		mlog.Fatal("failed to sub frontend fs", mlog.Err(err))
	}
	spa := &spaFileSystem{fs: http.FS(frontendFS)}
	staticServer := http.FileServer(spa)
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(404, gin.H{"error": "Not found"})
			return
		}
		staticServer.ServeHTTP(c.Writer, c.Request)
	})

	addr := ":" + cfg.ServerPort
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}
	go func() {
		mlog.Info("server starting", mlog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			mlog.Fatal("server failed", mlog.Err(err))
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	mlog.Info("shutdown signal received", mlog.String("signal", sig.String()))

	// Order matters: stop accepting requests → drain MCP children → stop scheduler → stop workers → exit.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		mlog.Warning("http shutdown error", mlog.Err(err))
	}
	if err := mcpHyp.PruneAll(); err != nil {
		mlog.Warning("mcp prune error", mlog.Err(err))
	}
	if err := sched.Stop(shutdownCtx); err != nil {
		mlog.Warning("scheduler stop error", mlog.Err(err))
	}
	if err := workerMgr.Stop(shutdownCtx); err != nil {
		mlog.Warning("worker manager stop error", mlog.Err(err))
	}
	if err := agentRuntime.Shutdown(shutdownCtx); err != nil {
		mlog.Warning("agent runtime shutdown error", mlog.Err(err))
	}
	mlog.Info("server shutdown complete")
}
