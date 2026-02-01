package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/knowledge-engine/backend/internal/config"
	"github.com/knowledge-engine/backend/internal/fetcher"
	"github.com/knowledge-engine/backend/internal/frontier"
	"github.com/knowledge-engine/backend/internal/politeness"
	"github.com/knowledge-engine/backend/internal/provider"
	"github.com/knowledge-engine/backend/internal/search"
	"github.com/knowledge-engine/backend/internal/storage"
)

// Engine orchestrates the crawling components
type Engine struct {
	Config            *config.Config
	Logger            *logrus.Entry
	Frontier          *frontier.URLFrontier
	Politeness        *politeness.PolitenessManager
	Fetcher           *fetcher.Fetcher
	Storage           storage.ContentStorage
	VectorStore       *search.VectorStore
	LLM               provider.LLMProvider
	
	// State
	isRunning    bool
	mu           sync.RWMutex
	cancelCrawl  context.CancelFunc
	activeCtx    context.Context
	
	// Stats
	Stats EngineStats
}

type EngineStats struct {
	PagesCrawled int64
	LastError    string
	StartTime    time.Time
}

func NewEngine(cfg *config.Config, logger *logrus.Entry, store storage.ContentStorage, vStore *search.VectorStore) (*Engine, error) {
	// Initialize core components
	f, err := frontier.NewURLFrontier(cfg.Frontier, logger)
	if err != nil {
		return nil, err
	}

	pm := politeness.NewPolitenessManager(cfg.Politeness, logger)
	ft := fetcher.NewFetcher(cfg.Politeness.DefaultRequestTimeout)

	// Initialize LLM Provider
	var llm provider.LLMProvider
	switch cfg.LLM.Provider {
	case "openai":
		llm = provider.NewOpenAIProvider(cfg.LLM.BaseURL, cfg.LLM.Model, cfg.LLM.APIKey)
	default:
		llm = provider.NewOllamaProvider(cfg.LLM.BaseURL, cfg.LLM.Model)
	}

	return &Engine{
		Config:      cfg,
		Logger:      logger,
		Frontier:    f,
		Politeness:  pm,
		Fetcher:     ft,
		Storage:     store,
		VectorStore: vStore,
		LLM:         llm,
	}, nil
}

// StartCrawl accepts a seed URL and starts the background worker
func (e *Engine) StartCrawl(seedURL string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.isRunning {
		// If running, just add to frontier
		e.Frontier.AddURL(e.activeCtx, seedURL, frontier.PriorityCritical, 0)
		return nil
	}

	// Setup context
	ctx, cancel := context.WithCancel(context.Background())
	e.activeCtx = ctx
	e.cancelCrawl = cancel
	e.isRunning = true
	e.Stats.StartTime = time.Now()

	// Start Politeness Manager
	if err := e.Politeness.Start(); err != nil {
		cancel()
		return err
	}

	// Start Frontier
	go func() {
		e.Frontier.Start(ctx)
	}()

	// Add Head
	e.Frontier.AddURL(ctx, seedURL, frontier.PriorityCritical, 0)

	// Start Worker Loop
go e.runWorker(ctx)

	return nil
}

func (e *Engine) StopCrawl() {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if e.isRunning && e.cancelCrawl != nil {
		e.cancelCrawl()
		e.Politeness.Stop()
		e.isRunning = false
	}
}

func (e *Engine) runWorker(ctx context.Context) {
	defer func() {
		e.mu.Lock()
		e.isRunning = false
		e.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 1. Get URL
			urlInfo, err := e.Frontier.GetNextURL(ctx)
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}
			if urlInfo == nil {
				time.Sleep(1 * time.Second)
				continue
			}

			// 2. Politeness
			if !e.Politeness.CanRequest(urlInfo.URL) {
				time.Sleep(100 * time.Millisecond)
				e.Frontier.MarkCompleted(ctx, urlInfo.URL) // Simplified requeue logic needed
				continue
			}

			// 3. Execution
			var wg sync.WaitGroup
			wg.Add(1)

			handler := func(u string) error {
				res, err := e.Fetcher.Fetch(ctx, u)
				if err != nil {
					return err
				}
				
				// Save to disk
				if err := e.Storage.Save(res); err != nil {
					e.Logger.WithError(err).Error("Failed to save result")
				}

				// Update Index (Live!)
				doc := &search.Document{
					ID:      res.URL,
					Title:   res.Title,
					Content: res.Text,
				}
				// Note: In real system, batch this
				e.VectorStore.AddDocuments([]*search.Document{doc})
				
				e.mu.Lock()
				e.Stats.PagesCrawled++
				e.mu.Unlock()

				// Discovery
				count := 0
				for _, link := range res.Links {
					if count > 20 { break }
					e.Frontier.AddURL(ctx, link, frontier.PriorityMedium, 0)
					count++
				}
				return nil
			}

			callback := func(u string, err error) {
				defer wg.Done()
				if err != nil {
					e.Frontier.MarkFailed(ctx, u, err)
				} else {
					e.Frontier.MarkCompleted(ctx, u)
				}
			}

			if err := e.Politeness.ScheduleRequest(urlInfo.URL, handler, callback); err != nil {
				e.Logger.WithError(err).Error("Failed to schedule")
				wg.Done()
			}

			wg.Wait()
		}
	}
}

func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.isRunning
}

// GenerateAnswer performs the full RAG flow: Search -> Build Prompt -> LLM Generation
func (e *Engine) GenerateAnswer(ctx context.Context, query string) (string, error) {
	// 1. Retrieve Context
	hits := e.VectorStore.Search(query, 5)
	var contextStr string
	for _, hit := range hits {
		contextStr += fmt.Sprintf("- %s\n", hit.Document.Content)
	}

	// 2. Build Prompt
	prompt := provider.BuildPrompt(query, contextStr)

	// 3. Call LLM
	return e.LLM.Generate(ctx, prompt)
}
