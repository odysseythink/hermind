// Package flow executes agent flows composed of api-call, llm-instruction,
// and web-scraping blocks. See server/utils/agentFlows/executor.js for the
// Node reference. Variables flow between steps via {{varname}} interpolation.
package flow
