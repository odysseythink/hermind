package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/agent/memorylayer"
	"github.com/odysseythink/hermind/agent/presence"
	"github.com/odysseythink/hermind/api"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/skills"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/hermind/tool/browser"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/extensions/delegate"
	"github.com/odysseythink/hermind/tool/document"
	"github.com/odysseythink/hermind/tool/embedding"
	"github.com/odysseythink/hermind/tool/file"
	"github.com/odysseythink/hermind/tool/mcp"
	"github.com/odysseythink/hermind/tool/memory"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
	"github.com/odysseythink/hermind/tool/obsidian"
	"github.com/odysseythink/hermind/tool/terminal"
	"github.com/odysseythink/hermind/tool/vision"
	"github.com/odysseythink/hermind/tool/web"
	"github.com/odysseythink/mlog"
)

// attachSkillsTracker constructs a Tracker and runs one initial
// Refresh so the persisted seq matches the current library content
// before any consumer reads it. Refresh failure is logged and
// ignored — the tracker is still returned, callers degrade to the
// last-persisted seq. Returns nil only if `store` is nil.
func attachSkillsTracker(ctx context.Context, store storage.Storage, skillDir string) *skills.Tracker {
	if store == nil {
		return nil
	}
	tr := skills.NewTracker(store, skillDir)
	if _, err := tr.Refresh(ctx); err != nil {
		mlog.Warning("skills.tracker startup refresh failed", mlog.String("err", err.Error()))
	}
	return tr
}

// BuildEngineDeps constructs the shared provider + aux + tool registry +
// skills bundle used by both the TUI (cli/repl.go) and the web server
// (cli/web.go). Callers invoke Cleanup on shutdown to release
// lifecycle-bearing resources (terminal backend, mcp manager,
// external memory provider).
//
// Hub is not set here — callers attach their own EventPublisher per
// request.
//
// This is a pragmatic extraction: repl.go historically inlined all of
// this. Leaving repl.go's copy in place keeps TUI behaviour stable
// while web gets a sharable builder. Plan 5 deletes the TUI so the
// duplicate disappears naturally.
func BuildEngineDeps(ctx context.Context, app *App) (api.EngineDeps, func(), error) {
	cleanupFns := []func(){}
	cleanup := func() {
		for i := len(cleanupFns) - 1; i >= 0; i-- {
			cleanupFns[i]()
		}
	}

	p, auxModel, err := buildProviders(ctx, app.Config)
	if err != nil {
		return api.EngineDeps{}, cleanup, err
	}
	if p == nil {
		primaryName := app.Config.Model
		if idx := strings.Index(app.Config.Model, "/"); idx >= 0 {
			primaryName = app.Config.Model[:idx]
		}
		fmt.Fprintf(os.Stderr, "%v: provider %q. Set api_key in <instance>/config.yaml or ANTHROPIC_API_KEY env var\n", errMissingAPIKey, primaryName)
		fmt.Fprintln(os.Stderr, "hermind: starting in degraded mode. Chat will fail until you configure a provider.")
	}

	displayModel := defaultModelFromString(app.Config.Model)

	toolRegistry := tool.NewRegistry()
	file.RegisterAll(toolRegistry)
	obsidian.RegisterAll(toolRegistry)
	document.RegisterAll(toolRegistry, app.InstanceRoot)
	tool.RegisterChart(toolRegistry)

	termCfg := terminal.Config{
		Cwd:              app.Config.Terminal.Cwd,
		DockerImage:      app.Config.Terminal.DockerImage,
		DockerVolumes:    app.Config.Terminal.DockerVolumes,
		SSHHost:          app.Config.Terminal.SSHHost,
		SSHUser:          app.Config.Terminal.SSHUser,
		SSHKey:           app.Config.Terminal.SSHKey,
		SingularityImage: app.Config.Terminal.SingularityImage,
		ModalBaseURL:     app.Config.Terminal.ModalBaseURL,
		ModalToken:       app.Config.Terminal.ModalToken,
		DaytonaBaseURL:   app.Config.Terminal.DaytonaBaseURL,
		DaytonaToken:     app.Config.Terminal.DaytonaToken,
	}
	if app.Config.Terminal.Timeout > 0 {
		termCfg.Timeout = time.Duration(app.Config.Terminal.Timeout) * time.Second
	}
	backend, err := terminal.New(app.Config.Terminal.Backend, termCfg)
	if err != nil {
		return api.EngineDeps{}, cleanup, fmt.Errorf("hermind: create terminal backend %q: %w", app.Config.Terminal.Backend, err)
	}
	cleanupFns = append(cleanupFns, func() { backend.Close() })
	terminal.RegisterShellExecute(toolRegistry, backend)

	web.RegisterAll(toolRegistry, web.Options{
		SearchProvider:       app.Config.Web.Search.Provider,
		TavilyAPIKey:         app.Config.Web.Search.Providers.Tavily.APIKey,
		BraveAPIKey:          app.Config.Web.Search.Providers.Brave.APIKey,
		ExaAPIKey:            app.Config.Web.Search.Providers.Exa.APIKey,
		DDGProxyConfig:       app.Config.Web.Search.Providers.DuckDuckGo,
		FirecrawlAPIKey:   os.Getenv("FIRECRAWL_API_KEY"),
		BingMarket:        app.Config.Web.Search.Providers.Bing.Market,
		SearXNGBaseURL:    app.Config.Web.Search.Providers.SearXNG.BaseURL,
		DefaultNumResults: app.Config.Web.Search.DefaultNumResults,
		MaxNumResults:        app.Config.Web.Search.MaxNumResults,
	})

	if app.Storage != nil {
		memory.RegisterAll(toolRegistry, app.Storage)
	}

	browserProvider := browser.NewBrowserbase(app.Config.Browser.Browserbase)
	browser.RegisterAll(toolRegistry, browserProvider)

	visionModel := app.Config.Auxiliary.Model
	if visionModel == "" {
		visionModel = displayModel
	}
	vision.Register(toolRegistry, auxModel, visionModel)

	// Register browser extension tools
	toolRegistry.Register(&tool.Entry{
		Name:        "browser_extension_read",
		Toolset:     "browser_extension",
		Description: "Read content previously scraped by the browser extension. Lists recent items or reads a specific document by ID.",
		Emoji:       "🔖",
		Handler:     api.NewBrowserExtensionReadHandler(app.InstanceRoot),
		Schema: core.ToolDefinition{
			Name:        "browser_extension_read",
			Description: "Read web page content that was previously scraped by the browser extension. Use this when the user refers to something they 'sent from the browser' or when you need to access content from a logged-in page that was captured via the extension.",
			Parameters:  core.MustSchemaFromJSON([]byte(api.BrowserExtensionReadSchema)),
		},
	})

	toolRegistry.Register(&tool.Entry{
		Name:        "browser_control",
		Toolset:     "browser_extension",
		Description: "Control the user's browser through the browser extension to navigate, click, fill forms, scroll, and extract content from web pages including those that require login.",
		Emoji:       "🌐",
		Handler:     api.NewBrowserControlHandler("", ""),
		Schema: core.ToolDefinition{
			Name:        "browser_control",
			Description: "Control the user's browser through the browser extension. Can navigate to URLs, click elements, fill forms, scroll pages, and extract text or HTML content. USE THIS for websites that require login or when you need to interact with a web page dynamically. The extension must be installed and running in the user's browser.",
			Parameters:  core.MustSchemaFromJSON([]byte(api.BrowserControlSchema)),
		},
	})

	// Use a stable session prefix for delegate sub-conversations spawned
	// from web requests. Each sub-conversation gets its own UUID suffix.
	sessionPrefix := uuid.NewString()

	// Set up embedder for skills retriever and memory provider
	var emb embedding.Embedder
	if p != nil {
		if ec, ok := p.(provider.EmbedCapable); ok {
			emb = embedding.NewProviderEmbedder(ec, "text-embedding-3-small")
		}
		// TODO: pantheon core.LanguageModel does not expose embedding yet;
		// embedding-dependent features are disabled when the provider is not
		// EmbedCapable.
	}

	// Set up skills evolver if auto-extract is enabled.
	//
	// Note: when AutoExtract is false, no Evolver is created. This means
	// the A-spec ConversationJudge path will call Extract on a nil
	// evolver (which the engine guards against), and no skill.extracted
	// events will be emitted. To enable judge-driven extraction without
	// AutoExtract's legacy LLM-extraction path, AutoExtract must be on
	// (the verdict path in evolver.go takes precedence when present).
	skillsDir := filepath.Join(app.InstanceRoot, "skills")
	var evolver *skills.Evolver
	if app.Config.Skills.AutoExtract {
		evolver = skills.NewEvolver(p, skillsDir)
		if app.Storage != nil {
			evolver.SetStorage(app.Storage)
		}
	}

	skillsTracker := attachSkillsTracker(ctx, app.Storage, skillsDir)
	if evolver != nil {
		evolver.WithTracker(skillsTracker)
	}

	// Set up skills retriever
	retriever := skills.NewRetriever(skillsDir, emb)

	extMem, err := memprovider.New(app.Config.Memory,
		memprovider.WithStorage(app.Storage),
		memprovider.WithLLM(p),
		memprovider.WithEmbedder(emb),
		memprovider.WithSkillsConfig(&app.Config.Skills),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hermind: memory provider: %v\n", err)
	}
	if extMem != nil {
		if mc, ok := extMem.(*memprovider.MetaClaw); ok {
			mc.SetSummaryEvery(app.Config.Memory.MetaClaw.SummaryEvery)
		}
		if err := extMem.Initialize(ctx, sessionPrefix); err != nil {
			fmt.Fprintf(os.Stderr, "hermind: memory provider %s init: %v\n", extMem.Name(), err)
		} else {
			extMem.RegisterTools(toolRegistry)
			cleanupFns = append(cleanupFns, func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = extMem.Shutdown(shutdownCtx)
			})
		}
	}

	// MemoryLayer is constructed when hybrid retrieval is enabled and the
	// underlying provider implements Recaller. A nil layer is a no-op.
	var memLayer *memorylayer.MemoryLayer
	if app.Config.MemoryLayer.Hybrid.Enabled {
		if r, ok := extMem.(memprovider.Recaller); ok && app.Storage != nil {
			memLayer = memorylayer.New(
				app.Storage,
				emb,
				r,
				p,
				translateMemoryLayerConfig(app.Config.MemoryLayer),
			)
		} else {
			mlog.Info("memorylayer: skipped (provider has no Recaller or no storage)")
		}
	}
	if memLayer != nil {
		cleanupFns = append(cleanupFns, func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			memLayer.Flush(shutdownCtx)
		})
	}

	delegate.RegisterDelegate(toolRegistry, func(subCtx context.Context, task, extra string, maxTurns int) (*delegate.SubagentResult, error) {
		// Sub-agent runs are ephemeral — they should not write to the
		// main conversation history.
		subEngine := agent.NewEngineWithToolsAndAux(
			p, auxModel, nil, toolRegistry,
			config.AgentConfig{
				MaxTurns:    maxTurns,
				Compression: app.Config.Agent.Compression,
			},
			"subagent",
		)
		result, err := subEngine.RunConversation(subCtx, &agent.RunOptions{
			UserMessage: task + "\n\n" + extra,
			Model:       displayModel,
			Ephemeral:   true,
		})
		if err != nil {
			return nil, err
		}
		return &delegate.SubagentResult{
			Response:   result.Response,
			Iterations: result.Iterations,
			ToolCalls:  0,
		}, nil
	})

	if len(app.Config.MCP.Servers) > 0 {
		mcpManager := mcp.NewManager("0.1.0", toolRegistry)
		var serverCfgs []mcp.ServerConfig
		for name, srv := range app.Config.MCP.Servers {
			if !srv.IsEnabled() {
				continue
			}
			serverCfgs = append(serverCfgs, mcp.ServerConfig{
				Name:    name,
				Command: srv.Command,
				Args:    srv.Args,
				Env:     srv.Env,
			})
		}
		if err := mcpManager.Start(ctx, serverCfgs); err != nil {
			fmt.Fprintf(os.Stderr, "hermind: mcp warning: %v\n", err)
		}
		cleanupFns = append(cleanupFns, func() { mcpManager.Close() })
	}

	skillsReg, _ := loadSkills(app)

	// Resolve HTTP idle threshold with deprecation alias.
	absentAfter := app.Config.Presence.HTTPIdleAbsentAfterSeconds
	if absentAfter == 0 && app.Config.Memory.ConsolidateIdleAfterSeconds > 0 {
		mlog.Warning("config.deprecated_field",
			mlog.String("field", "memory.consolidate_idle_after_seconds"),
			mlog.String("replacement", "presence.http_idle_absent_after_seconds"))
		absentAfter = app.Config.Memory.ConsolidateIdleAfterSeconds
	}
	if absentAfter == 0 {
		absentAfter = 300 // default 5 minutes
	}
	httpIdle := presence.NewHTTPIdle(time.Duration(absentAfter) * time.Second)

	sources := []presence.Source{httpIdle}
	if app.Config.Presence.SleepWindow.Enabled {
		sw, err := presence.NewSleepWindow(app.Config.Presence.SleepWindow)
		if err != nil {
			return api.EngineDeps{}, cleanup, fmt.Errorf("presence: sleep window: %w", err)
		}
		sources = append(sources, sw)
	}
	composite := presence.NewComposite(sources...)

	return api.EngineDeps{
		Provider:        p,
		AuxProvider:     auxModel,
		Storage:         app.Storage,
		ToolReg:         toolRegistry,
		SkillsReg:       skillsReg,
		AgentCfg:        app.Config.Agent,
		Platform:        "web",
		SkillsEvolver:   evolver,
		SkillsRetriever: retriever,
		MemProvider:     extMem,
		MemoryLayer:     memLayer,
		SkillsTracker:   skillsTracker,
		HTTPIdle:        httpIdle,
		Presence:        composite,
	}, cleanup, nil
}

func translateMemoryLayerConfig(cfg config.MemoryLayerConfig) memorylayer.Config {
	return memorylayer.Config{
		Hybrid: memorylayer.HybridConfig{
			RRFConstant:             cfg.Hybrid.RRFConstant,
			BM25TopNMultiplier:      cfg.Hybrid.BM25TopNMultiplier,
			VectorTopNMultiplier:    cfg.Hybrid.VectorTopNMultiplier,
			PreRerankTopKMultiplier: cfg.Hybrid.PreRerankTopKMultiplier,
			ReinforcementAlpha:      cfg.Hybrid.ReinforcementAlpha,
			NeglectPenalty:          cfg.Hybrid.NeglectPenalty,
		},
		Reranker: memorylayer.RerankerConfig{
			Enabled:   cfg.Reranker.Enabled,
			BatchSize: cfg.Reranker.BatchSize,
			Timeout:   time.Duration(cfg.Reranker.TimeoutMS) * time.Millisecond,
		},
		Boundary: memorylayer.BoundaryConfig{
			HardTokenLimit:            cfg.Boundary.HardTokenLimit,
			HardTurnLimit:             cfg.Boundary.HardTurnLimit,
			SoftTokenThreshold:        cfg.Boundary.SoftTokenThreshold,
			IdleGap:                   time.Duration(cfg.Boundary.IdleGapMinutes) * time.Minute,
			EnableTopicShift:          cfg.Boundary.EnableTopicShift,
			TopicShiftCosineThreshold: cfg.Boundary.TopicShiftCosineThreshold,
		},
		Taxonomy: memorylayer.TaxonomyConfig{
			Enabled:      cfg.Taxonomy.Enabled,
			MaxOutputs:   cfg.Taxonomy.MaxOutputs,
			Timeout:      time.Duration(cfg.Taxonomy.TimeoutMS) * time.Millisecond,
			AllowedTypes: cfg.Taxonomy.AllowedTypes,
		},
		Agentic: memorylayer.AgenticConfig{
			Enabled:            cfg.Agentic.Enabled,
			MaxExtraRounds:     cfg.Agentic.MaxExtraRounds,
			ExpansionQueries:   cfg.Agentic.ExpansionQueries,
			ShortcutThreshold:  cfg.Agentic.ShortcutThreshold,
			PerTurnTokenCap:    cfg.Agentic.PerTurnTokenCap,
			PerSessionTokenCap: cfg.Agentic.PerSessionTokenCap,
			Timeout:            time.Duration(cfg.Agentic.TimeoutMS) * time.Millisecond,
		},
		Lifecycle: memorylayer.LifecycleConfig{
			InjectCoreOnStart:      cfg.Lifecycle.InjectCoreOnStart,
			CoreMaxCount:           cfg.Lifecycle.CoreMaxCount,
			CoreMaxTokens:          cfg.Lifecycle.CoreMaxTokens,
			InjectForesightOnStart: cfg.Lifecycle.InjectForesightOnStart,
			ForesightMaxCount:      cfg.Lifecycle.ForesightMaxCount,
			ForesightDaysAhead:     cfg.Lifecycle.ForesightDaysAhead,
		},
		RecallLimit: cfg.RecallLimit,
	}
}
