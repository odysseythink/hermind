package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/pantheon/agent/trajectory"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/cron"
	"github.com/odysseythink/hermind/logging"
	"github.com/odysseythink/pantheon/observability/metrics"
	"github.com/odysseythink/hermind/pantheonadapter"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/pantheon/core"
	"github.com/spf13/cobra"
)

func newCronCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "cron",
		Short: "Run scheduled cron jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCron(cmd.Context(), app)
		},
	}
}

func runCron(ctx context.Context, app *App) error {
	if err := EnsureStorage(app); err != nil {
		return err
	}

	// Resolve primary provider config
	primaryName := app.Config.Model
	if idx := strings.Index(app.Config.Model, "/"); idx >= 0 {
		primaryName = app.Config.Model[:idx]
	}
	primaryCfg, ok := app.Config.Providers[primaryName]
	if !ok {
		primaryCfg = config.ProviderConfig{Provider: primaryName}
	}
	if primaryCfg.Provider == "" {
		primaryCfg.Provider = primaryName
	}
	if primaryName == "anthropic" && primaryCfg.APIKey == "" {
		if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" {
			primaryCfg.APIKey = envKey
		}
	}
	if primaryCfg.Model == "" {
		primaryCfg.Model = defaultModelFromString(app.Config.Model)
	}

	var primary core.LanguageModel
	if primaryCfg.APIKey != "" {
		var err error
		primary, err = pantheonadapter.BuildPrimaryModel(ctx, primaryCfg)
		if err != nil {
			return err
		}
	}

	sched := cron.NewScheduler()

	// Optional /metrics HTTP server.
	var metricsSrv *http.Server
	if addr := app.Config.Metrics.Addr; addr != "" {
		metricsReg := metrics.NewRegistry()
		sched.SetMetrics(metricsReg)
		mux := http.NewServeMux()
		mux.Handle("/metrics", metricsReg)
		metricsSrv = &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		go func() {
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "cron: metrics server: %v\n", err)
			}
		}()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = metricsSrv.Shutdown(shutdownCtx)
		}()
	}

	for _, jc := range app.Config.Cron.Jobs {
		if jc.Name == "" || jc.Schedule == "" || jc.Prompt == "" {
			fmt.Fprintf(os.Stderr, "cron: skipping malformed job %q\n", jc.Name)
			continue
		}
		schedule, err := cron.ParseSchedule(jc.Schedule)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cron: skipping %s: %v\n", jc.Name, err)
			continue
		}
		sched.Add(buildCronJob(jc, schedule, primary, app))
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-sig; cancel() }()

	return sched.Start(runCtx)
}

func buildCronJob(jc config.CronJobConfig, sched cron.Schedule, prov core.LanguageModel, app *App) cron.Job {
	jobName := jc.Name
	prompt := jc.Prompt
	model := jc.Model
	return cron.Job{
		Name:     jobName,
		Schedule: sched,
		Run: func(ctx context.Context) error {
			ctx = logging.WithRequestID(ctx, uuid.NewString())
			// Isolated engine with no storage — cron runs do not touch
			// the main conversation's messages table.
			eng := agent.NewEngineWithTools(
				prov, nil, nil,
				app.Config.Agent, "cron",
			)

			// Each job gets its own trajectory file under <instance>/trajectories/.
			root, err := config.InstancePath("trajectories")
			if err == nil {
				tw, twErr := trajectory.New(
					root,
					fmt.Sprintf("cron-%s-%d", jobName, time.Now().Unix()),
				)
				if twErr == nil {
					defer tw.Close()
					eng.SetStreamDeltaCallback(func(d *provider.StreamDelta) {
						_ = tw.Write(trajectory.Event{
							Kind:    "assistant",
							Content: d.Content,
						})
					})
				}
			}

			_, err = eng.RunConversation(ctx, &agent.RunOptions{
				UserMessage: prompt,
				Model:       model,
				Ephemeral:   true,
			})
			return err
		},
	}
}
