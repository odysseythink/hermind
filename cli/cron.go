package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/cron"
	"github.com/odysseythink/hermind/logging"
	"github.com/odysseythink/hermind/metrics"
	"github.com/odysseythink/hermind/provider"
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
	logging.Setup(app.Config.Logging.Level)

	if err := ensureStorage(app); err != nil {
		return err
	}
	primary, _, err := buildPrimaryProvider(app.Config)
	if err != nil {
		return err
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

func buildCronJob(jc config.CronJobConfig, sched cron.Schedule, prov provider.Provider, app *App) cron.Job {
	jobName := jc.Name
	prompt := jc.Prompt
	model := jc.Model
	return cron.Job{
		Name:     jobName,
		Schedule: sched,
		Run: func(ctx context.Context) error {
			ctx = logging.WithRequestID(ctx, uuid.NewString())
			eng := agent.NewEngineWithTools(
				prov, app.Storage, nil,
				app.Config.Agent, "cron",
			)
			_, err := eng.RunConversation(ctx, &agent.RunOptions{
				UserMessage: prompt,
				SessionID:   "cron-" + jobName + "-" + uuid.NewString(),
				Model:       model,
			})
			return err
		},
	}
}
