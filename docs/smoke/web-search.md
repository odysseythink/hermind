# Web Search Smoke Verification

Manual verification flow for the multi-provider `web_search` tool.
Each scenario exercises a different provider + config interaction.

## Prerequisites

- A working `hermind` build: `go build -o bin/hermind ./cli`
- A config file at `~/.hermind/config.yaml` (or point `--config` to one).
- Optional: API keys for the paid providers you want to exercise.

## 1. DuckDuckGo fallback (no keys configured)

```bash
unset EXA_API_KEY TAVILY_API_KEY BRAVE_API_KEY
hermind run --prompt "search the web for 'golang tutorials' using web_search and summarize the top 3"
```

Expected:
- Agent invokes `web_search` with a `golang tutorials` query.
- Response JSON payload has `"provider": "ddg"`.
- At least one result present (DDG rate-limiting can cause an empty
  result set — retry with a different query if so).

## 2. Explicit provider via env var (Tavily)

```bash
export TAVILY_API_KEY="<your key>"
hermind run --prompt "search the web for 'kubernetes networking' and list the top 3 with snippets"
```

Expected:
- `"provider": "tavily"` in the tool result.
- Results include non-empty `snippet` fields and a `score` float.

## 3. Explicit provider via config

Edit `~/.hermind/config.yaml`:

```yaml
web:
  search:
    provider: brave
    providers:
      brave:
        api_key: "<your Brave key>"
```

Run:
```bash
hermind run --prompt "find me news about SpaceX from the last week using web_search"
```

Expected:
- `"provider": "brave"` in the tool result.
- Results include URL + description but no `score` field.

## 4. Auto-priority when multiple keys present

Configure keys for Tavily + Exa (no `provider:` pin):

```yaml
web:
  search:
    providers:
      tavily:
        api_key: "<tav>"
      exa:
        api_key: "<exa>"
```

Run any search. Expected: `"provider": "tavily"` (tavily wins per
priority order Tavily > Brave > Exa > DuckDuckGo).

## 5. Cache hit on repeat query

Within 60 seconds, ask the agent twice:
```bash
hermind run --prompt "use web_search to find 'rust async book' — do this twice in a row"
```

Expected: both calls return identical results; on the second call the
tool returns instantly (no HTTP round trip). Verify in logs that only
one `[web_search] provider=<id>` line per unique query is emitted.

## 6. Explicit-but-missing provider

Set `provider: brave` in config without providing `brave.api_key`
and without `BRAVE_API_KEY`:

```yaml
web:
  search:
    provider: brave
```

Run a search. Expected: tool returns an error payload
`{"error":"provider \"brave\" not configured; ..."}` — the agent
sees it and either retries differently or reports it.
