package frontier

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/knowledge-engine/backend/internal/config"
)

// Priority levels for URL crawling
type Priority int

const (
	PriorityLow Priority = iota
	PriorityMedium
	PriorityHigh
	PriorityCritical
)

// URLInfo represents a URL in the frontier with metadata
type URLInfo struct {
	URL         string    `json:"url"`
	Priority    Priority  `json:"priority"`
	Depth       int       `json:"depth"`
	AddedAt     time.Time `json:"added_at"`
	LastAttempt time.Time `json:"last_attempt,omitempty"`
	RetryCount  int       `json:"retry_count"`
	Domain      string    `json:"domain"`
	Hash        string    `json:"hash"`
}

// URLFrontier manages the queue of URLs to be crawled
type URLFrontier struct {
	config   config.FrontierConfig
	logger   *logrus.Entry
	
	// In-memory storage
	priorityQueues map[Priority][]*URLInfo
	urlMap         map[string]*URLInfo  // URL -> URLInfo for quick lookup
	domainQueues   map[string][]*URLInfo // Domain-based queues for politeness
	
	// Synchronization
	mutex          sync.RWMutex
	domainMutex    sync.RWMutex
	
	// Channels for communication
	addChan        chan *URLInfo
	getChan        chan chan *URLInfo
	stopChan       chan struct{}
	
	// Statistics
	stats          FrontierStats
	statsMutex     sync.RWMutex
}

// FrontierStats holds statistics about the URL frontier
type FrontierStats struct {
	TotalAdded     int64
	TotalProcessed int64
	TotalFailed    int64
	CurrentQueued  int64
	LastUpdated    time.Time
}

// NewURLFrontier creates a new URL frontier instance
func NewURLFrontier(config config.FrontierConfig, logger *logrus.Entry) (*URLFrontier, error) {
	frontier := &URLFrontier{
		config:         config,
		logger:         logger.WithField("component", "url_frontier"),
		priorityQueues: make(map[Priority][]*URLInfo),
		urlMap:         make(map[string]*URLInfo),
		domainQueues:   make(map[string][]*URLInfo),
		addChan:        make(chan *URLInfo, 1000),
		getChan:        make(chan chan *URLInfo),
		stopChan:       make(chan struct{}),
		stats: FrontierStats{
			LastUpdated: time.Now(),
		},
	}

	// Initialize priority queues
	for p := PriorityLow; p <= PriorityCritical; p++ {
		frontier.priorityQueues[p] = make([]*URLInfo, 0)
	}

	return frontier, nil
}

// Start begins the URL frontier processing
func (f *URLFrontier) Start(ctx context.Context) error {
	f.logger.Info("Starting URL frontier")
	
	// Start cleanup routine
	go f.cleanupRoutine(ctx)
	
	// Main processing loop
	for {
		select {
		case <-ctx.Done():
			f.logger.Info("URL frontier stopping due to context cancellation")
			return ctx.Err()
		case <-f.stopChan:
			f.logger.Info("URL frontier stopping")
			return nil
		case urlInfo := <-f.addChan:
			f.handleAddURL(urlInfo)
		case responseChan := <-f.getChan:
			urlInfo := f.handleGetURL()
			responseChan <- urlInfo
		}
	}
}

// AddURL adds a URL to the frontier
func (f *URLFrontier) AddURL(ctx context.Context, rawURL string, priority Priority, depth int) error {
	// Normalize and validate URL
	normalizedURL, err := f.normalizeURL(rawURL)
	if err != nil {
		return fmt.Errorf("failed to normalize URL %s: %w", rawURL, err)
	}

	// Check if URL already exists
	f.mutex.RLock()
	if existing, exists := f.urlMap[normalizedURL]; exists {
		f.mutex.RUnlock()
		// Update priority if new one is higher
		if priority > existing.Priority {
			existing.Priority = priority
			f.logger.WithFields(logrus.Fields{
				"url": normalizedURL,
				"old_priority": existing.Priority,
				"new_priority": priority,
			}).Debug("Updated URL priority")
		}
		return nil
	}
	f.mutex.RUnlock()

	// Create URL info
	urlInfo := &URLInfo{
		URL:      normalizedURL,
		Priority: priority,
		Depth:    depth,
		AddedAt:  time.Now(),
		Domain:   f.extractDomain(normalizedURL),
		Hash:     f.hashURL(normalizedURL),
	}

	// Add to queue
	select {
	case f.addChan <- urlInfo:
		f.updateStats(func(s *FrontierStats) { s.TotalAdded++ })
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetNextURL retrieves the next URL to crawl
func (f *URLFrontier) GetNextURL(ctx context.Context) (*URLInfo, error) {
	responseChan := make(chan *URLInfo)
	
	select {
	case f.getChan <- responseChan:
		select {
		case urlInfo := <-responseChan:
			if urlInfo != nil {
				f.updateStats(func(s *FrontierStats) { s.TotalProcessed++ })
			}
			return urlInfo, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// MarkCompleted marks a URL as successfully processed
func (f *URLFrontier) MarkCompleted(ctx context.Context, url string) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	
	if urlInfo, exists := f.urlMap[url]; exists {
		delete(f.urlMap, url)
		f.logger.WithField("url", url).Debug("Marked URL as completed")
		f.updateStats(func(s *FrontierStats) { s.CurrentQueued-- })
		_ = urlInfo // Use urlInfo to avoid unused variable warning
	}
}

// MarkFailed marks a URL as failed and potentially retries it
func (f *URLFrontier) MarkFailed(ctx context.Context, url string, err error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	
	if urlInfo, exists := f.urlMap[url]; exists {
		urlInfo.RetryCount++
		urlInfo.LastAttempt = time.Now()
		
		if urlInfo.RetryCount >= f.config.MaxRetries {
			// Max retries reached, remove from queue
			delete(f.urlMap, url)
			f.logger.WithFields(logrus.Fields{
				"url": url,
				"retry_count": urlInfo.RetryCount,
				"error": err,
			}).Warn("URL failed permanently after max retries")
			f.updateStats(func(s *FrontierStats) { 
				s.TotalFailed++
				s.CurrentQueued-- 
			})
		} else {
			// Reduce priority for retry
			if urlInfo.Priority > PriorityLow {
				urlInfo.Priority--
			}
			f.logger.WithFields(logrus.Fields{
				"url": url,
				"retry_count": urlInfo.RetryCount,
				"error": err,
			}).Debug("URL marked for retry")
		}
	}
}

// GetStats returns current frontier statistics
func (f *URLFrontier) GetStats() FrontierStats {
	f.statsMutex.RLock()
	defer f.statsMutex.RUnlock()
	return f.stats
}

// Close gracefully shuts down the URL frontier
func (f *URLFrontier) Close() error {
	f.logger.Info("Closing URL frontier")
	close(f.stopChan)
	return nil
}

// handleAddURL processes the addition of a new URL
func (f *URLFrontier) handleAddURL(urlInfo *URLInfo) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	// Check memory limits
	if len(f.urlMap) >= f.config.MaxURLsInMemory {
		f.logger.Warn("URL frontier at memory limit, dropping URL")
		return
	}

	// Add to URL map
	f.urlMap[urlInfo.URL] = urlInfo

	// Add to priority queue
	f.priorityQueues[urlInfo.Priority] = append(f.priorityQueues[urlInfo.Priority], urlInfo)

	// Add to domain queue for politeness
	f.domainMutex.Lock()
	f.domainQueues[urlInfo.Domain] = append(f.domainQueues[urlInfo.Domain], urlInfo)
	f.domainMutex.Unlock()

	f.updateStats(func(s *FrontierStats) { s.CurrentQueued++ })

	f.logger.WithFields(logrus.Fields{
		"url":      urlInfo.URL,
		"priority": urlInfo.Priority,
		"domain":   urlInfo.Domain,
		"depth":    urlInfo.Depth,
	}).Debug("Added URL to frontier")
}

// handleGetURL processes the retrieval of the next URL
func (f *URLFrontier) handleGetURL() *URLInfo {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	// Check each priority level from highest to lowest
	for priority := PriorityCritical; priority >= PriorityLow; priority-- {
		queue := f.priorityQueues[priority]
		
		for i, urlInfo := range queue {
			// Check politeness constraints
			if f.canCrawlDomain(urlInfo.Domain) {
				// Remove from priority queue
				f.priorityQueues[priority] = append(queue[:i], queue[i+1:]...)
				
				// Update last attempt time
				urlInfo.LastAttempt = time.Now()
				
				return urlInfo
			}
		}
	}

	return nil // No URLs available for crawling right now
}

// canCrawlDomain checks if we can crawl from this domain based on politeness rules
func (f *URLFrontier) canCrawlDomain(domain string) bool {
	f.domainMutex.RLock()
	defer f.domainMutex.RUnlock()

	domainQueue, exists := f.domainQueues[domain]
	if !exists || len(domainQueue) == 0 {
		return true
	}

	// Find the most recent crawl from this domain
	var mostRecent time.Time
	for _, urlInfo := range domainQueue {
		if !urlInfo.LastAttempt.IsZero() && urlInfo.LastAttempt.After(mostRecent) {
			mostRecent = urlInfo.LastAttempt
		}
	}

	// Check if enough time has passed
	return time.Since(mostRecent) >= f.config.PolitenessDelay
}

// normalizeURL normalizes a URL for consistent handling
func (f *URLFrontier) normalizeURL(rawURL string) (string, error) {
	// Basic validation
	if rawURL == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}
	
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %w", err)
	}
	
	// Check for valid scheme
	if parsed.Scheme == "" {
		return "", fmt.Errorf("URL must have a scheme (http, https, etc.)")
	}
	
	// Check for valid host
	if parsed.Host == "" {
		return "", fmt.Errorf("URL must have a host")
	}

	// Basic normalizations
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	
	// Remove fragment
	parsed.Fragment = ""
	
	// Remove trailing slash for paths (except root)
	if len(parsed.Path) > 1 && strings.HasSuffix(parsed.Path, "/") {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	}

	return parsed.String(), nil
}

// extractDomain extracts the domain from a URL
func (f *URLFrontier) extractDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Host
}

// hashURL creates a hash of the URL for deduplication
func (f *URLFrontier) hashURL(url string) string {
	hash := md5.Sum([]byte(url))
	return fmt.Sprintf("%x", hash)
}

// updateStats safely updates frontier statistics
func (f *URLFrontier) updateStats(updateFn func(*FrontierStats)) {
	f.statsMutex.Lock()
	defer f.statsMutex.Unlock()
	updateFn(&f.stats)
	f.stats.LastUpdated = time.Now()
}

// cleanupRoutine periodically cleans up expired URLs and manages memory
func (f *URLFrontier) cleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(f.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.performCleanup()
		}
	}
}

// performCleanup removes stale URLs and manages memory
func (f *URLFrontier) performCleanup() {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	now := time.Now()
	cleanupThreshold := 24 * time.Hour // Remove URLs older than 24 hours if not processed

	var removedCount int
	for url, urlInfo := range f.urlMap {
		if urlInfo.LastAttempt.IsZero() && now.Sub(urlInfo.AddedAt) > cleanupThreshold {
			delete(f.urlMap, url)
			removedCount++
		}
	}

	if removedCount > 0 {
		f.logger.WithField("removed_count", removedCount).Info("Cleaned up stale URLs")
		f.updateStats(func(s *FrontierStats) { s.CurrentQueued -= int64(removedCount) })
		
		// Rebuild priority queues to remove dangling pointers
		f.rebuildPriorityQueues()
	}
}

// rebuildPriorityQueues rebuilds the priority queues after cleanup
func (f *URLFrontier) rebuildPriorityQueues() {
	// Clear existing queues
	for p := PriorityLow; p <= PriorityCritical; p++ {
		f.priorityQueues[p] = f.priorityQueues[p][:0]
	}

	// Rebuild from URL map
	for _, urlInfo := range f.urlMap {
		f.priorityQueues[urlInfo.Priority] = append(f.priorityQueues[urlInfo.Priority], urlInfo)
	}
}