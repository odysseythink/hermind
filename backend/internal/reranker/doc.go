// Package reranker provides relevance-based reordering of retrieved documents.
//
// The default NoopReranker passes through the input order (compatible with
// the previous vectordb.SearchOptions.Rerank fetch-more-then-truncate
// behavior). PantheonReranker delegates to pantheon's rerank model,
// currently backed by Cohere via openaicompat.Client.
package reranker
