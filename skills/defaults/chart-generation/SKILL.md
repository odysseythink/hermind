---
name: chart-generation
description: >
  Generate charts, graphs, and data visualizations to make numeric data
  scannable. Use whenever the user asks to "plot", "chart", "graph",
  "visualize", "show me a trend", or whenever the response would otherwise
  be a wall of numbers (comparisons across categories, time series,
  proportions of a whole, distributions, correlations). Use the `chart`
  tool — do not draw charts in markdown or ASCII. Trigger proactively
  when summarizing tabular data, benchmark results, or anything where a
  picture beats a table.
---

# SKILL: Chart Generation

## When to use the chart tool

Reach for `chart` whenever the answer involves **more than ~5 numeric data
points** and the user would benefit from seeing the shape, not just the
values. Concrete triggers:

- The user explicitly asks for a chart, plot, graph, or visualization.
- You're about to render a markdown table with ≥2 numeric columns and ≥5 rows.
- You're comparing a metric across categories, time, or groups.
- You're showing a part-to-whole breakdown (use `pie` or `treemap`).

Do **not** use the chart tool for:
- Single numbers (just say the number).
- Qualitative content (lists, prose, code).
- Tables with text columns where the value IS the data (e.g. PR titles).

## Picking a chart type

| Goal                              | Type         |
|-----------------------------------|--------------|
| Compare across categories         | `bar`        |
| Trend over ordered/time axis      | `line`       |
| Cumulative trend / filled trend   | `area`       |
| Mix bars + lines on one axis      | `composed`   |
| Correlation between two metrics   | `scatter`    |
| Part-to-whole, ≤6 slices          | `pie`        |
| Multi-axis profile / spider chart | `radar`      |
| Single-metric progress dial       | `radialBar`  |
| Hierarchical proportions          | `treemap`    |
| Stage-by-stage drop-off           | `funnel`     |

## Dataset format

`dataset` is a JSON **string** (not a JSON object) — a top-level array
where each element has:

- `name`: the category / x-axis label (string)
- one or more **numeric** fields for the values to plot

Example:

    [
      {"name": "Q1", "revenue": 120, "cost": 80},
      {"name": "Q2", "revenue": 150, "cost": 95},
      {"name": "Q3", "revenue": 180, "cost": 110}
    ]

Limits and gotchas:
- Maximum 1000 records per dataset; aggregate or sample if larger.
- Every record needs at least one numeric field besides `name`.
- `title` is required and must be non-empty — make it specific
  ("Weekly active users, Jan–Mar" beats "Users").
- `caption` is optional; use it for the data source or units.

## Workflow

1. Decide whether a chart is genuinely better than a table or sentence.
2. Pick the type from the table above.
3. Shape the data into the required array-of-objects form.
4. Call the `chart` tool with `type`, `title`, `dataset`, and optional
   `caption`. Do not render the chart yourself in markdown or ASCII.
5. After the tool call, add at most one sentence of interpretation —
   the chart speaks for itself.
