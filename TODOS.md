# TODOs

## Design

- [ ] **Create DESIGN.md via /design-consultation**
  - **Why:** The plan has inline design tokens (symbols, colors, spacing) but no canonical design system document. Without DESIGN.md, each contributor makes independent visual decisions.
  - **Pros:** Single source of truth for all CLI visual decisions. Enables consistency across skins. Makes onboarding new contributors easier.
  - **Cons:** Takes ~30 min to run /design-consultation. May need iteration.
  - **Context:** Design tokens are currently embedded in the spec file (Section 7). They should be extracted into a standalone DESIGN.md that covers the full CLI design system.
  - **Depends on:** None. Can be done before or during implementation.

## Observability (v1.1)

- [ ] **Add Prometheus/OpenTelemetry metrics to gateway**
  - **Why:** Production gateway serving 21 platforms needs observability. First outage without metrics = nobody can debug.
  - **Pros:** Visibility into per-platform message rates, provider fallback frequency, tool execution duration, LLM token costs. Enables alerting and capacity planning.
  - **Cons:** Adds dependencies (`prometheus/client_golang` or `go.opentelemetry.io/otel`). ~2-3 days of work.
  - **Context:** The plan defers metrics to post-v1 to keep initial scope manageable. v1.1 should add: HTTP metrics endpoint, per-platform counters, per-provider histograms, session cost gauges.
  - **Depends on:** v1.0 ship. Should be tracked as first item after launch.

## Testing Infrastructure

- [ ] **Record and commit LLM provider HTTP cassettes**
  - **Why:** Integration tests need reproducible LLM responses. Without a shared cassette library, each contributor re-records locally with their own API keys (duplicated effort, non-reproducible CI).
  - **Pros:** Reproducible integration tests across contributors and CI. No secrets required in CI (cassettes replay without real API calls). Lower test costs.
  - **Cons:** Cassettes go stale when API responses change. Need a refresh policy (probably per-release).
  - **Context:** The plan mentions `go-vcr` for cassette recording but doesn't specify ownership. Should: (1) create a bootstrap Go program that records cassettes for each provider's Complete() and Stream() methods against a real test account, (2) commit to `testdata/cassettes/`, (3) document how to refresh.
  - **Depends on:** Provider interface implementation (must exist to record against it).
