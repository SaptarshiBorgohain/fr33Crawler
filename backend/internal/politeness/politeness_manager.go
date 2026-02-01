package politeness

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/temoto/robotstxt"

	"github.com/knowledge-engine/backend/internal/config"
)

// PolitenessManager handles respectful crawling behavior
type PolitenessManager struct {
	config       config.PolitenessConfig
	logger       *logrus.Entry
	domainStates map[string]*DomainState
	robotsCache  map[string]*RobotsEntry
	requestQueue chan *RequestItem
	running      bool
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mu           sync.RWMutex
	
	// Statistics
	stats Statistics
}

// DomainState tracks the state of requests to a specific domain
type DomainState struct {
	domain            string
	lastRequestTime   time.Time
	activeRequests    int
	maxConcurrency    int
	minDelay          time.Duration
	requestQueue      []*RequestItem
	lastAccess        time.Time
	mu                sync.RWMutex
}

// RequestItem represents a queued request
type RequestItem struct {
	url         string
	handler     func(string) error
	callback    func(string, error)
	scheduledAt time.Time
	ctx         context.Context
	cancel      context.CancelFunc
}

// RobotsEntry caches robots.txt data
type RobotsEntry struct {
	robots    *robotstxt.RobotsData
	fetchTime time.Time
	userAgent string
}

// Statistics holds politeness manager statistics
type Statistics struct {
	TotalRequests     int64            `json:"total_requests"`
	CompletedRequests int64            `json:"completed_requests"`
	FailedRequests    int64            `json:"failed_requests"`
	RejectedRequests  int64            `json:"rejected_requests"`
	ActiveRequests    int64            `json:"active_requests"`
	DomainStats       map[string]*DomainStatistics `json:"domain_stats"`
	StartTime         time.Time        `json:"start_time"`
	mu                sync.RWMutex
}

// DomainStatistics holds per-domain statistics
type DomainStatistics struct {
	Domain            string        `json:"domain"`
	TotalRequests     int64         `json:"total_requests"`
	CompletedRequests int64         `json:"completed_requests"`
	FailedRequests    int64         `json:"failed_requests"`
	ActiveRequests    int64         `json:"active_requests"`
	LastRequestTime   time.Time     `json:"last_request_time"`
	AverageDelay      time.Duration `json:"average_delay"`
}

// NewPolitenessManager creates a new politeness manager
func NewPolitenessManager(config config.PolitenessConfig, logger *logrus.Entry) *PolitenessManager {
	if logger == nil {
		logger = logrus.WithField("component", "politeness_manager")
	}

	pm := &PolitenessManager{
		config:       config,
		logger:       logger,
		domainStates: make(map[string]*DomainState),
		robotsCache:  make(map[string]*RobotsEntry),
		requestQueue: make(chan *RequestItem, config.RequestQueueSize),
		stats: Statistics{
			DomainStats: make(map[string]*DomainStatistics),
			StartTime:   time.Now(),
		},
	}

	return pm
}

// Start starts the politeness manager
func (pm *PolitenessManager) Start() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.running {
		return fmt.Errorf("politeness manager is already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	pm.cancel = cancel
	pm.running = true

	// Start worker goroutines
	pm.wg.Add(2)
	go pm.requestProcessor(ctx)
	go pm.cleanupWorker(ctx)

	pm.logger.Info("Politeness manager started")
	return nil
}

// Stop stops the politeness manager
func (pm *PolitenessManager) Stop() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.running {
		return fmt.Errorf("politeness manager is not running")
	}

	pm.running = false
	
	// Cancel context to signal goroutines to stop
	if pm.cancel != nil {
		pm.cancel()
	}
	
	// Wait for goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		pm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		pm.logger.Info("Politeness manager stopped")
		return nil
	case <-time.After(5 * time.Second):
		pm.logger.Warn("Politeness manager stop timed out")
		return fmt.Errorf("stop operation timed out")
	}
}

// CanRequest checks if a request to the given URL is allowed based on politeness rules
func (pm *PolitenessManager) CanRequest(rawURL string) bool {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Host == "" {
		return false
	}

	// Only allow HTTP/HTTPS
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false
	}

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if !pm.running {
		return false
	}

	domain := parsedURL.Host
	domainState := pm.getOrCreateDomainState(domain)

	domainState.mu.RLock()
	defer domainState.mu.RUnlock()

	// Check if we're within concurrency limits
	if domainState.activeRequests >= domainState.maxConcurrency {
		return false
	}

	// Check if enough time has passed since last request
	if time.Since(domainState.lastRequestTime) < domainState.minDelay {
		return false
	}

	return true
}

// ScheduleRequest schedules a request for processing with politeness controls
func (pm *PolitenessManager) ScheduleRequest(rawURL string, handler func(string) error, callback func(string, error)) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a host")
	}

	// Only allow HTTP/HTTPS
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("only HTTP/HTTPS URLs are supported")
	}

	pm.mu.RLock()
	running := pm.running
	pm.mu.RUnlock()

	if !running {
		return fmt.Errorf("politeness manager is not running")
	}

	// Create request context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), pm.config.DefaultRequestTimeout)
	
	request := &RequestItem{
		url:         rawURL,
		handler:     handler,
		callback:    callback,
		scheduledAt: time.Now(),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Try to enqueue request
	select {
	case pm.requestQueue <- request:
		pm.updateStats(func(stats *Statistics) {
			stats.TotalRequests++
		})
		return nil
	default:
		cancel() // Clean up context
		return fmt.Errorf("request queue is full for URL: %s", rawURL)
	}
}

// IsURLAllowed checks if URL is allowed according to robots.txt
func (pm *PolitenessManager) IsURLAllowed(rawURL string) (bool, error) {
	if !pm.config.EnableRobotsCheck {
		return true, nil
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false, fmt.Errorf("invalid URL: %v", err)
	}

	robotsData, err := pm.getRobotsData(parsedURL.Host)
	if err != nil {
		pm.logger.WithError(err).WithField("domain", parsedURL.Host).Warn("Failed to get robots.txt, allowing request")
		return true, nil // Allow on robots.txt fetch failure
	}

	if robotsData == nil {
		return true, nil // No robots.txt found, allow request
	}

	group := robotsData.FindGroup(pm.config.UserAgent)
	if group == nil {
		return true, nil // No specific rules for our user agent
	}

	return group.Test(parsedURL.Path), nil
}

// GetStatistics returns current statistics
func (pm *PolitenessManager) GetStatistics() Statistics {
	pm.stats.mu.RLock()
	defer pm.stats.mu.RUnlock()

	// Create a deep copy to avoid race conditions
	stats := Statistics{
		TotalRequests:     pm.stats.TotalRequests,
		CompletedRequests: pm.stats.CompletedRequests,
		FailedRequests:    pm.stats.FailedRequests,
		RejectedRequests:  pm.stats.RejectedRequests,
		ActiveRequests:    pm.stats.ActiveRequests,
		StartTime:         pm.stats.StartTime,
		DomainStats:       make(map[string]*DomainStatistics),
	}

	for domain, domainStats := range pm.stats.DomainStats {
		stats.DomainStats[domain] = &DomainStatistics{
			Domain:            domainStats.Domain,
			TotalRequests:     domainStats.TotalRequests,
			CompletedRequests: domainStats.CompletedRequests,
			FailedRequests:    domainStats.FailedRequests,
			ActiveRequests:    domainStats.ActiveRequests,
			LastRequestTime:   domainStats.LastRequestTime,
			AverageDelay:      domainStats.AverageDelay,
		}
	}

	return stats
}

// GetDomainStateCount returns the number of active domain states.
// This is primarily intended for monitoring and testing.
func (pm *PolitenessManager) GetDomainStateCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return len(pm.domainStates)
}

// requestProcessor processes requests from the queue
func (pm *PolitenessManager) requestProcessor(ctx context.Context) {
	defer pm.wg.Done()
	
	pm.logger.Info("Request processor started")
	defer pm.logger.Info("Request processor stopped")

	for {
		select {
		case <-ctx.Done():
			// Cancel any remaining requests in the channel
			go func() {
				for request := range pm.requestQueue {
					request.cancel()
					request.callback(request.url, fmt.Errorf("politeness manager shutting down"))
				}
			}()
			return
		case request, ok := <-pm.requestQueue:
			if !ok {
				return // Channel closed
			}
			
			pm.processRequest(request)
		}
	}
}

// processRequest handles individual request processing
func (pm *PolitenessManager) processRequest(request *RequestItem) {
	parsedURL, err := url.Parse(request.url)
	if err != nil {
		request.cancel() // Clean up context on error
		request.callback(request.url, fmt.Errorf("invalid URL: %v", err))
		pm.updateStats(func(stats *Statistics) {
			stats.FailedRequests++
		})
		return
	}

	domain := parsedURL.Host

	// Get domain state with proper locking
	pm.mu.RLock()
	domainState := pm.getOrCreateDomainState(domain)
	pm.mu.RUnlock()

	// Check robots.txt if enabled
	if pm.config.EnableRobotsCheck {
		allowed, err := pm.IsURLAllowed(request.url)
		if err != nil {
			pm.logger.WithError(err).WithField("url", request.url).Warn("Robots check failed")
		} else if !allowed {
			pm.logger.WithField("url", request.url).Debug("URL blocked by robots.txt")
			request.cancel() // Clean up context
			request.callback(request.url, fmt.Errorf("URL blocked by robots.txt"))
			pm.updateStats(func(stats *Statistics) {
				stats.RejectedRequests++
			})
			return
		}
	}

	// Wait for politeness delay
	pm.waitForPolitenessDelay(domainState)

	// Increment active requests
	domainState.mu.Lock()
	if domainState.activeRequests >= domainState.maxConcurrency {
		domainState.mu.Unlock()
		request.cancel() // Clean up context
		request.callback(request.url, fmt.Errorf("domain concurrency limit exceeded"))
		pm.updateStats(func(stats *Statistics) {
			stats.RejectedRequests++
		})
		return
	}
	domainState.activeRequests++
	domainState.lastRequestTime = time.Now()
	domainState.lastAccess = time.Now()
	domainState.mu.Unlock()

	// Update statistics
	pm.updateStats(func(stats *Statistics) {
		stats.ActiveRequests++
		if stats.DomainStats[domain] == nil {
			stats.DomainStats[domain] = &DomainStatistics{Domain: domain}
		}
		stats.DomainStats[domain].ActiveRequests++
	})

	// Execute request handler
	go func() {
		defer func() {
			// Clean up request context
			request.cancel()
			
			// Decrement active requests
			domainState.mu.Lock()
			domainState.activeRequests--
			domainState.mu.Unlock()

			pm.updateStats(func(stats *Statistics) {
				stats.ActiveRequests--
				stats.DomainStats[domain].ActiveRequests--
			})
		}()

		// Execute with timeout
		var handlerErr error
		done := make(chan struct{})
		
		go func() {
			defer close(done)
			handlerErr = request.handler(request.url)
		}()

		select {
		case <-done:
			// Handler completed
			if handlerErr != nil {
				pm.updateStats(func(stats *Statistics) {
					stats.FailedRequests++
					stats.DomainStats[domain].FailedRequests++
				})
			} else {
				pm.updateStats(func(stats *Statistics) {
					stats.CompletedRequests++
					stats.DomainStats[domain].CompletedRequests++
					stats.DomainStats[domain].LastRequestTime = time.Now()
				})
			}
			request.callback(request.url, handlerErr)
			
		case <-request.ctx.Done():
			// Request timed out
			pm.updateStats(func(stats *Statistics) {
				stats.FailedRequests++
				stats.DomainStats[domain].FailedRequests++
			})
			request.callback(request.url, request.ctx.Err())
		}
	}()
}

// waitForPolitenessDelay waits for the appropriate delay before making a request
func (pm *PolitenessManager) waitForPolitenessDelay(domainState *DomainState) {
	domainState.mu.RLock()
	lastRequestTime := domainState.lastRequestTime
	minDelay := domainState.minDelay
	domainState.mu.RUnlock()

	if lastRequestTime.IsZero() {
		return // First request to this domain
	}

	elapsed := time.Since(lastRequestTime)
	if elapsed < minDelay {
		waitTime := minDelay - elapsed
		pm.logger.WithFields(logrus.Fields{
			"domain":    domainState.domain,
			"wait_time": waitTime,
		}).Debug("Waiting for politeness delay")
		time.Sleep(waitTime)
	}
}

// getOrCreateDomainState gets or creates domain state (must be called with read lock)
func (pm *PolitenessManager) getOrCreateDomainState(domain string) *DomainState {
	// Check if state already exists
	if state, exists := pm.domainStates[domain]; exists {
		state.mu.Lock()
		state.lastAccess = time.Now()
		state.mu.Unlock()
		return state
	}

	// Need to create new domain state - upgrade to write lock
	pm.mu.RUnlock()
	pm.mu.Lock()
	
	// Check again in case another goroutine created it while we were waiting
	if state, exists := pm.domainStates[domain]; exists {
		state.mu.Lock()
		state.lastAccess = time.Now()
		state.mu.Unlock()
		pm.mu.Unlock()
		pm.mu.RLock() // Downgrade back to read lock
		return state
	}

	// Create new domain state
	state := &DomainState{
		domain:         domain,
		maxConcurrency: pm.config.DefaultDomainConcurrency,
		minDelay:       pm.config.DefaultMinDelay,
		lastAccess:     time.Now(),
	}

	pm.domainStates[domain] = state
	pm.logger.WithField("domain", domain).Debug("Created new domain state")
	
	pm.mu.Unlock()
	pm.mu.RLock() // Downgrade back to read lock
	return state
}

// getRobotsData fetches and caches robots.txt data
func (pm *PolitenessManager) getRobotsData(domain string) (*robotstxt.RobotsData, error) {
	pm.mu.RLock()
	entry, exists := pm.robotsCache[domain]
	pm.mu.RUnlock()

	if exists && time.Since(entry.fetchTime) < pm.config.RobotsCacheDuration {
		return entry.robots, nil
	}

	// Need to fetch robots.txt
	robotsURL := fmt.Sprintf("https://%s/robots.txt", domain)
	
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	req, err := http.NewRequest("GET", robotsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create robots.txt request: %v", err)
	}
	
	req.Header.Set("User-Agent", pm.config.UserAgent)
	
	resp, err := client.Do(req)
	if err != nil {
		// Try HTTP if HTTPS fails
		robotsURL = fmt.Sprintf("http://%s/robots.txt", domain)
		req.URL, _ = url.Parse(robotsURL)
		resp, err = client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch robots.txt: %v", err)
		}
	}
	defer resp.Body.Close()

	var robotsData *robotstxt.RobotsData
	if resp.StatusCode == http.StatusOK {
		robotsData, err = robotstxt.FromResponse(resp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse robots.txt: %v", err)
		}
	}

	// Cache the result (even if nil for 404s)
	pm.mu.Lock()
	pm.robotsCache[domain] = &RobotsEntry{
		robots:    robotsData,
		fetchTime: time.Now(),
		userAgent: pm.config.UserAgent,
	}
	pm.mu.Unlock()

	return robotsData, nil
}

// cleanupWorker periodically cleans up expired domain states and robots cache
func (pm *PolitenessManager) cleanupWorker(ctx context.Context) {
	defer pm.wg.Done()
	
	ticker := time.NewTicker(pm.config.CleanupInterval)
	defer ticker.Stop()
	
	pm.logger.Info("Cleanup worker started")
	defer pm.logger.Info("Cleanup worker stopped")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pm.cleanup()
		}
	}
}

// cleanup removes expired domain states and robots cache entries
func (pm *PolitenessManager) cleanup() {
	now := time.Now()
	
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Clean up expired domain states
	var expiredDomains []string
	for domain, state := range pm.domainStates {
		state.mu.RLock()
		expired := now.Sub(state.lastAccess) > pm.config.DomainStateExpiry
		activeRequests := state.activeRequests
		state.mu.RUnlock()

		if expired && activeRequests == 0 {
			expiredDomains = append(expiredDomains, domain)
		}
	}

	for _, domain := range expiredDomains {
		delete(pm.domainStates, domain)
		pm.logger.WithField("domain", domain).Debug("Cleaned up expired domain state")
	}

	// Clean up expired robots cache entries
	var expiredRobots []string
	for domain, entry := range pm.robotsCache {
		if now.Sub(entry.fetchTime) > pm.config.RobotsCacheDuration {
			expiredRobots = append(expiredRobots, domain)
		}
	}

	for _, domain := range expiredRobots {
		delete(pm.robotsCache, domain)
		pm.logger.WithField("domain", domain).Debug("Cleaned up expired robots cache entry")
	}

	if len(expiredDomains) > 0 || len(expiredRobots) > 0 {
		pm.logger.WithFields(logrus.Fields{
			"expired_domains": len(expiredDomains),
			"expired_robots":  len(expiredRobots),
		}).Debug("Cleanup completed")
	}
}

// updateStats safely updates statistics
func (pm *PolitenessManager) updateStats(updateFn func(*Statistics)) {
	pm.stats.mu.Lock()
	defer pm.stats.mu.Unlock()
	updateFn(&pm.stats)
}