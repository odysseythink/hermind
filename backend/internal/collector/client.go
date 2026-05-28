package collector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/collector/extensions"
	"github.com/odysseythink/hermind/backend/internal/collector/external"
	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
	"github.com/odysseythink/hermind/backend/internal/collector/processors"
	"github.com/odysseythink/hermind/backend/internal/collector/scraper"
	"github.com/odysseythink/hermind/backend/internal/collector/utils"
)

// Client is a local collector client that processes documents directly.
type Client struct {
	pipeline        *pipeline.Pipeline
	registry        *processors.Registry
	extRegistry     *extensions.Registry
	scraper         *scraper.Manager
	tokenizer       *utils.Tokenizer
	storageDir      string
	chromedpAdapter *external.ChromedpAdapter
	options         Options
}

// NewLocalCollector creates a local Collector client.
func NewLocalCollector(storageDir string) (*Client, error) {
	tokenizer, err := utils.NewTokenizer()
	if err != nil {
		return nil, fmt.Errorf("init tokenizer: %w", err)
	}

	opts := attachOptions()

	shellRunner := utils.NewShellRunner()

	// External adapters
	tesseract := external.NewTesseractAdapter("", shellRunner)
	whisperLocal := external.NewWhisperLocalAdapter(opts.WhisperModelPref, shellRunner)
	whisperOpenAI := external.NewWhisperOpenAIAdapter(opts.OpenAiKey)
	chromedp := external.NewChromedpAdapter(opts.RuntimeSettings.BrowserLaunchArgs)

	// Processors registry
	procReg := processors.NewRegistry()
	procReg.Register(".txt", processors.NewTxtExtractor())
	procReg.Register(".md", processors.NewTxtExtractor())
	procReg.Register(".org", processors.NewTxtExtractor())
	procReg.Register(".adoc", processors.NewTxtExtractor())
	procReg.Register(".rst", processors.NewTxtExtractor())
	procReg.Register(".csv", processors.NewTxtExtractor())
	procReg.Register(".json", processors.NewTxtExtractor())
	procReg.Register(".html", processors.NewHTMLExtractor())
	pdfExt := processors.NewPDFExtractor(tesseract, shellRunner)
	procReg.Register(".pdf", pdfExt)
	procReg.Register(".docx", processors.NewDocxExtractor())
	procReg.Register(".xlsx", processors.NewXlsxExtractor())
	procReg.Register(".pptx", processors.NewPptxExtractor())
	procReg.Register(".odt", processors.NewODFExtractor())
	procReg.Register(".odp", processors.NewODFExtractor())
	procReg.Register(".epub", processors.NewEPubExtractor())
	procReg.Register(".mbox", processors.NewMboxExtractor())
	procReg.Register(".png", processors.NewImageExtractor(tesseract))
	procReg.Register(".jpg", processors.NewImageExtractor(tesseract))
	procReg.Register(".jpeg", processors.NewImageExtractor(tesseract))
	procReg.Register(".webp", processors.NewImageExtractor(tesseract))
	audioExt := processors.NewAudioExtractor(whisperLocal, whisperOpenAI, shellRunner)
	procReg.Register(".mp3", audioExt)
	procReg.Register(".wav", audioExt)
	procReg.Register(".mp4", audioExt)
	procReg.Register(".mpeg", audioExt)
	procReg.Register(".ogg", audioExt)
	procReg.Register(".oga", audioExt)
	procReg.Register(".opus", audioExt)
	procReg.Register(".m4a", audioExt)
	procReg.Register(".webm", audioExt)

	// Extensions
	githubExt := extensions.NewGitHubExtension()
	gitlabExt := extensions.NewGitLabExtension()
	confluenceExt := extensions.NewConfluenceExtension()
	drupalwikiExt := extensions.NewDrupalWikiExtension()
	obsidianExt := extensions.NewObsidianExtension()
	paperlessExt := extensions.NewPaperlessExtension()
	websiteDepthExt := extensions.NewWebsiteDepthExtension()
	youtubeExt := extensions.NewYouTubeExtension()

	extReg := extensions.NewRegistry()
	extReg.Register("/ext/github-repo", githubExt)
	extReg.Register("/ext/github-repo/branches", githubExt)
	extReg.Register("/ext/gitlab-repo", gitlabExt)
	extReg.Register("/ext/gitlab-repo/branches", gitlabExt)
	extReg.Register("/ext/confluence", confluenceExt)
	extReg.Register("/ext/drupalwiki", drupalwikiExt)
	extReg.Register("/ext/obsidian/vault", obsidianExt)
	extReg.Register("/ext/paperless-ngx", paperlessExt)
	extReg.Register("/ext/website-depth", websiteDepthExt)
	extReg.Register("/ext/youtube-transcript", youtubeExt)

	extMap := map[string]extensions.Extension{
		"github":             githubExt,
		"gitlab":             gitlabExt,
		"confluence":         confluenceExt,
		"drupalwiki":         drupalwikiExt,
		"paperless-ngx":      paperlessExt,
		"youtube-transcript": youtubeExt,
	}

	// Scraper manager
	scrapr := scraper.NewManager(chromedp, tokenizer)

	// Resync extension
	resyncExt := extensions.NewResyncExtension(scrapr, extMap)
	extReg.Register("/ext/resync-source-document", resyncExt)

	// Pipeline
	watchDir := filepath.Join(storageDir, "documents")
	enricher := pipeline.NewEnricher(tokenizer)
	pipe := pipeline.NewPipeline(storageDir, watchDir, enricher)
	pipe.SetRegistry(procReg)
	pipe.RegisterTextExtractor(processors.NewTxtExtractor())

	return &Client{
		pipeline:        pipe,
		registry:        procReg,
		extRegistry:     extReg,
		scraper:         scrapr,
		tokenizer:       tokenizer,
		storageDir:      storageDir,
		chromedpAdapter: chromedp,
		options:         opts,
	}, nil
}

// NewClient is a backward-compatible alias for NewLocalCollector.
func NewClient(storageDir string) (*Client, error) {
	return NewLocalCollector(storageDir)
}

// Online checks if collector is reachable. Always returns true for local collector.
func (c *Client) Online(ctx context.Context) bool {
	return true
}

// AcceptedFileTypes returns list of accepted MIME types.
func (c *Client) AcceptedFileTypes(ctx context.Context) ([]string, error) {
	return utils.AcceptedFileTypes(), nil
}

// ProcessDocument sends a file to collector for processing.
func (c *Client) ProcessDocument(ctx context.Context, filename string, metadata map[string]string) (*ProcessResponse, error) {
	baseName := filepath.Base(filename)
	resp := c.pipeline.ProcessFile(ctx, baseName, pipeline.ProcessFileOptions{
		AbsolutePath: filename,
		Options:      c.options,
	}, metadata)
	return resp, nil
}

// ParseDocument parses a document without full processing.
func (c *Client) ParseDocument(ctx context.Context, filename string, opts ParseOptions) (*ProcessResponse, error) {
	pipeOpts := pipeline.ProcessFileOptions{
		ParseOnly: true,
		Options:   c.options,
	}
	if opts.AbsolutePath != "" {
		pipeOpts.AbsolutePath = opts.AbsolutePath
	}
	resp := c.pipeline.ProcessFile(ctx, filename, pipeOpts, nil)
	return resp, nil
}

// ProcessLink scrapes a URL and saves the content as a document.
// This mirrors the original /process-link endpoint behavior.
func (c *Client) ProcessLink(ctx context.Context, link string, scraperHeaders, metadata map[string]string) (*ProcessResponse, error) {
	return c.scraper.Scrape(ctx, link, "text", scraperHeaders, true, metadata, c.storageDir)
}

// GetLinkContent extracts content from a link as text or html.
func (c *Client) GetLinkContent(ctx context.Context, link string, captureAs string) (*LinkContentResponse, error) {
	return c.scraper.GetLinkText(ctx, link, captureAs)
}

// ProcessRawText sends raw text content to collector.
func (c *Client) ProcessRawText(ctx context.Context, textContent string, metadata map[string]string) (*ProcessResponse, error) {
	if metadata == nil {
		metadata = map[string]string{}
	}
	title := metadata["title"]
	if title == "" {
		title = "Raw Text"
	}

	doc := &Document{
		Name:        title,
		Title:       title,
		PageContent: textContent,
		DocSource:   "raw text uploaded by the user.",
	}

	doc.WordCount = len(strings.Fields(textContent))
	doc.TokenCountEstimate = c.tokenizer.Count(textContent)

	filename := fmt.Sprintf("%s-%s", utils.SlugifyFilename(title), uuid.New().String())
	savedDoc, err := utils.WriteToServerDocuments(c.storageDir, doc, filename, false)
	if err != nil {
		return &ProcessResponse{
			Success: false,
			Reason:  err.Error(),
		}, nil
	}

	return &ProcessResponse{
		Success:   true,
		Documents: []Document{*savedDoc},
	}, nil
}

// ForwardExtensionRequest relays arbitrary requests to collector extensions.
func (c *Client) ForwardExtensionRequest(ctx context.Context, endpoint, method, body string) (*ExtensionResponse, error) {
	return c.extRegistry.Handle(ctx, endpoint, method, []byte(body))
}

// ParseInMemory writes data to a temporary file, parses it, and returns the extracted text.
// The temporary file is cleaned up before returning.
func (c *Client) ParseInMemory(ctx context.Context, filename string, data []byte) (string, error) {
	tmpDir := filepath.Join(c.storageDir, "tmp")
	_ = os.MkdirAll(tmpDir, 0755)
	tmpPath := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return "", fmt.Errorf("write temp file: %w", err)
	}
	defer os.Remove(tmpPath)

	resp, err := c.ParseDocument(ctx, filename, ParseOptions{AbsolutePath: tmpPath})
	if err != nil {
		return "", fmt.Errorf("parse document: %w", err)
	}
	if !resp.Success {
		return "", fmt.Errorf("parse failed: %s", resp.Reason)
	}
	if len(resp.Documents) == 0 {
		return "", fmt.Errorf("no documents parsed")
	}
	return resp.Documents[0].PageContent, nil
}

// Close shuts down the collector client and releases external resources.
func (c *Client) Close() {
	if c.chromedpAdapter != nil {
		c.chromedpAdapter.Close()
	}
}

// attachOptions builds Options from environment variables.
func attachOptions() Options {
	whisperProvider := os.Getenv("WHISPER_PROVIDER")
	if whisperProvider == "" {
		whisperProvider = "local"
	}

	allowAnyIp := os.Getenv("COLLECTOR_ALLOW_ANY_IP")
	if _, ok := os.LookupEnv("COLLECTOR_ALLOW_ANY_IP"); !ok {
		allowAnyIp = "false"
	}

	var browserLaunchArgs []string
	if args := os.Getenv("ANYTHINGLLM_CHROMIUM_ARGS"); args != "" {
		for _, arg := range strings.Split(args, ",") {
			browserLaunchArgs = append(browserLaunchArgs, strings.TrimSpace(arg))
		}
	}

	langList := os.Getenv("TARGET_OCR_LANG")
	if langList == "" {
		langList = "eng"
	}

	return Options{
		WhisperProvider:  whisperProvider,
		WhisperModelPref: os.Getenv("WHISPER_MODEL_PREF"),
		OpenAiKey:        os.Getenv("OPEN_AI_KEY"),
		OCR: OCROptions{
			LangList: langList,
		},
		RuntimeSettings: RuntimeSettings{
			AllowAnyIp:        allowAnyIp,
			BrowserLaunchArgs: browserLaunchArgs,
		},
	}
}
