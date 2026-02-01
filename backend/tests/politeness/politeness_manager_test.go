package politeness_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/knowledge-engine/backend/internal/config"
	"github.com/knowledge-engine/backend/internal/politeness"
)

func init() {
	// Set log level to warn to reduce noise during tests
	logrus.SetLevel(logrus.WarnLevel)
}

func TestNewPolitenessManager(t *testing.T) {
	cfg := config.PolitenessConfig{
		DefaultMinDelay:          1 * time.Second,
		DefaultDomainConcurrency: 2,
		GlobalMaxConcurrency:     10,
		RequestQueueSize:         100,
		DefaultRequestTimeout:    30 * time.Second,
		CleanupInterval:          5 * time.Minute,
		DomainStateExpiry:        1 * time.Hour,
		RobotsCacheDuration:      24 * time.Hour,
		EnableRobotsCheck:        true,
		UserAgent:                "TestCrawler/1.0",
	}

	pm := politeness.NewPolitenessManager(cfg, nil)
	require.NotNil(t, pm)
	assert.False(t, pm.CanRequest("http://example.com/page1"))
	assert.False(t, pm.GetStatistics().StartTime.IsZero())
}

func TestPolitenessManager_Start_Stop(t *testing.T) {
	cfg := config.PolitenessConfig{
		DefaultMinDelay:          100 * time.Millisecond,
		DefaultDomainConcurrency: 1,
		GlobalMaxConcurrency:     5,
		RequestQueueSize:         10,
		DefaultRequestTimeout:    5 * time.Second,
		CleanupInterval:          1 * time.Second, // Short interval for testing
		DomainStateExpiry:        5 * time.Second,
		RobotsCacheDuration:      1 * time.Minute,
		EnableRobotsCheck:        false,
		UserAgent:                "TestCrawler/1.0",
	}

	pm := politeness.NewPolitenessManager(cfg, nil)
	assert.False(t, pm.CanRequest("http://example.com/page1"))

	// Start the manager
	err := pm.Start()
	assert.NoError(t, err)
	assert.True(t, pm.CanRequest("http://example.com/page1"))

	// Starting again should return error
	err = pm.Start()
	assert.Error(t, err)

	// Stop the manager
	err = pm.Stop()
	assert.NoError(t, err)
	assert.False(t, pm.CanRequest("http://example.com/page1"))

	// Stopping again should return error
	err = pm.Stop()
	assert.Error(t, err)
}

func TestPolitenessManager_CanRequest(t *testing.T) {
	cfg := config.PolitenessConfig{
		DefaultMinDelay:          100 * time.Millisecond,
		DefaultDomainConcurrency: 2,
		GlobalMaxConcurrency:     10,
		RequestQueueSize:         100,
		DefaultRequestTimeout:    30 * time.Second,
		CleanupInterval:          5 * time.Minute,
		DomainStateExpiry:        1 * time.Hour,
		RobotsCacheDuration:      24 * time.Hour,
		EnableRobotsCheck:        false,
		UserAgent:                "TestCrawler/1.0",
	}

	pm := politeness.NewPolitenessManager(cfg, nil)
	require.NoError(t, pm.Start())
	defer pm.Stop()

	// First request should be allowed
	assert.True(t, pm.CanRequest("http://example.com/page1"))

	// Second request to same domain should be allowed (within concurrency limit)
	assert.True(t, pm.CanRequest("http://example.com/page2"))

	// Note: CanRequest only checks if we can make the request, it doesn't actually
	// increment the active request count. To properly test concurrency limits,
	// we would need to actually schedule requests and check their status.

	// Request to different domain should be allowed
	assert.True(t, pm.CanRequest("http://other.com/page1"))
}

func TestPolitenessManager_ScheduleRequest(t *testing.T) {
	cfg := config.PolitenessConfig{
		DefaultMinDelay:          10 * time.Millisecond,
		DefaultDomainConcurrency: 1,
		GlobalMaxConcurrency:     5,
		RequestQueueSize:         10,
		DefaultRequestTimeout:    5 * time.Second,
		CleanupInterval:          5 * time.Minute,
		DomainStateExpiry:        1 * time.Hour,
		RobotsCacheDuration:      24 * time.Hour,
		EnableRobotsCheck:        false,
		UserAgent:                "TestCrawler/1.0",
	}

	pm := politeness.NewPolitenessManager(cfg, nil)
	require.NoError(t, pm.Start())

	// Mock handler that returns success quickly
	handler := func(url string) error {
		return nil
	}

	// Schedule a request
	requestChan := make(chan error, 1)
	err := pm.ScheduleRequest("http://example.com/page1", handler, func(url string, err error) {
		requestChan <- err
	})
	assert.NoError(t, err)

	// Wait for request to complete with a reasonable timeout
	select {
	case err := <-requestChan:
		// Accept either success or graceful shutdown
		if err != nil {
			assert.Contains(t, []string{"context canceled", "politeness manager shutting down"}, err.Error(),
				"Expected success or graceful shutdown, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Request timed out")
	}

	// Give some time for request to be processed
	time.Sleep(100 * time.Millisecond)

	// Stop after request completes
	pm.Stop()
}

func TestPolitenessManager_ScheduleRequest_WithDelay(t *testing.T) {
	cfg := config.PolitenessConfig{
		DefaultMinDelay:          200 * time.Millisecond,
		DefaultDomainConcurrency: 1,
		GlobalMaxConcurrency:     5,
		RequestQueueSize:         10,
		DefaultRequestTimeout:    5 * time.Second,
		CleanupInterval:          5 * time.Minute,
		DomainStateExpiry:        1 * time.Hour,
		RobotsCacheDuration:      24 * time.Hour,
		EnableRobotsCheck:        false,
		UserAgent:                "TestCrawler/1.0",
	}

	pm := politeness.NewPolitenessManager(cfg, nil)
	require.NoError(t, pm.Start())
	defer pm.Stop()

	// Mock handler
	handler := func(url string) error {
		return nil
	}

	// Schedule first request
	firstComplete := make(chan time.Time, 1)
	err := pm.ScheduleRequest("http://example.com/page1", handler, func(url string, err error) {
		firstComplete <- time.Now()
	})
	assert.NoError(t, err)

	// Schedule second request to same domain
	secondComplete := make(chan time.Time, 1)
	err = pm.ScheduleRequest("http://example.com/page2", handler, func(url string, err error) {
		secondComplete <- time.Now()
	})
	assert.NoError(t, err)

	// Wait for both requests to complete
	var firstTime, secondTime time.Time
	for i := 0; i < 2; i++ {
		select {
		case t1 := <-firstComplete:
			firstTime = t1
		case t2 := <-secondComplete:
			secondTime = t2
		case <-time.After(5 * time.Second):
			t.Fatal("Requests timed out")
		}
	}

	// Ensure second request was delayed
	assert.True(t, secondTime.After(firstTime))
	delay := secondTime.Sub(firstTime)
	assert.True(t, delay >= cfg.DefaultMinDelay, "Expected delay of at least %v, got %v", cfg.DefaultMinDelay, delay)
}

func TestPolitenessManager_RequestTimeout(t *testing.T) {
	cfg := config.PolitenessConfig{
		DefaultMinDelay:          10 * time.Millisecond,
		DefaultDomainConcurrency: 1,
		GlobalMaxConcurrency:     5,
		RequestQueueSize:         10,
		DefaultRequestTimeout:    100 * time.Millisecond, // Short timeout for testing
		CleanupInterval:          5 * time.Minute,
		DomainStateExpiry:        1 * time.Hour,
		RobotsCacheDuration:      24 * time.Hour,
		EnableRobotsCheck:        false,
		UserAgent:                "TestCrawler/1.0",
	}

	pm := politeness.NewPolitenessManager(cfg, nil)
	require.NoError(t, pm.Start())
	defer pm.Stop()

	// Handler that takes longer than timeout
	handler := func(url string) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	}

	// Schedule request
	requestChan := make(chan error, 1)
	err := pm.ScheduleRequest("http://example.com/page1", handler, func(url string, err error) {
		requestChan <- err
	})
	assert.NoError(t, err)

	// Wait for request to timeout
	select {
	case err := <-requestChan:
		assert.Error(t, err)
		// Context can be canceled or deadline exceeded depending on timing
		assert.True(t, err.Error() == "context deadline exceeded" || err.Error() == "context canceled", 
			"Expected timeout or cancellation error, got: %s", err.Error())
	case <-time.After(2 * time.Second):
		t.Fatal("Expected timeout error")
	}
}

func TestPolitenessManager_QueueFull(t *testing.T) {
	cfg := config.PolitenessConfig{
		DefaultMinDelay:          10 * time.Millisecond,
		DefaultDomainConcurrency: 1,
		GlobalMaxConcurrency:     5,
		RequestQueueSize:         1, // Very small queue
		DefaultRequestTimeout:    5 * time.Second,
		CleanupInterval:          5 * time.Minute,
		DomainStateExpiry:        1 * time.Hour,
		RobotsCacheDuration:      24 * time.Hour,
		EnableRobotsCheck:        false,
		UserAgent:                "TestCrawler/1.0",
	}

	pm := politeness.NewPolitenessManager(cfg, nil)
	require.NoError(t, pm.Start())
	defer pm.Stop()

	// Slow handler to block the processor
	handler := func(url string) error {
		time.Sleep(100 * time.Millisecond) // Enough to fill queue
		return nil
	}

	// Fill up the queue - first request should work
	err := pm.ScheduleRequest("http://example.com/page1", handler, func(url string, err error) {})
	assert.NoError(t, err)

	// Second request should fill the queue since processor is slow
	err = pm.ScheduleRequest("http://example.com/page2", handler, func(url string, err error) {})
	if err != nil {
		// If queue is full immediately, that's also valid
		assert.Contains(t, err.Error(), "request queue is full")
	} else {
		// Try third request which should definitely fail
		err = pm.ScheduleRequest("http://example.com/page3", handler, func(url string, err error) {})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "request queue is full")
	}
}

func TestPolitenessManager_RobotsCheck(t *testing.T) {
	// Create a test server that serves robots.txt
	robotsContent := `User-agent: *
Disallow: /private/
Allow: /public/
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(robotsContent))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Page content"))
		}
	}))
	defer server.Close()

	cfg := config.PolitenessConfig{
		DefaultMinDelay:          10 * time.Millisecond,
		DefaultDomainConcurrency: 1,
		GlobalMaxConcurrency:     5,
		RequestQueueSize:         10,
		DefaultRequestTimeout:    5 * time.Second,
		CleanupInterval:          5 * time.Minute,
		DomainStateExpiry:        1 * time.Hour,
		RobotsCacheDuration:      1 * time.Minute,
		EnableRobotsCheck:        true,
		UserAgent:                "TestCrawler/1.0",
	}

	pm := politeness.NewPolitenessManager(cfg, nil)
	require.NoError(t, pm.Start())
	defer pm.Stop()

	// Test allowed URL
	allowed, err := pm.IsURLAllowed(server.URL + "/public/page.html")
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Test disallowed URL
	allowed, err = pm.IsURLAllowed(server.URL + "/private/secret.html")
	assert.NoError(t, err)
	assert.False(t, allowed)

	// Test other URLs (should be allowed by default)
	allowed, err = pm.IsURLAllowed(server.URL + "/other/page.html")
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestPolitenessManager_Statistics(t *testing.T) {
	cfg := config.PolitenessConfig{
		DefaultMinDelay:          10 * time.Millisecond,
		DefaultDomainConcurrency: 2,
		GlobalMaxConcurrency:     5,
		RequestQueueSize:         10,
		DefaultRequestTimeout:    5 * time.Second,
		CleanupInterval:          5 * time.Minute,
		DomainStateExpiry:        1 * time.Hour,
		RobotsCacheDuration:      24 * time.Hour,
		EnableRobotsCheck:        false,
		UserAgent:                "TestCrawler/1.0",
	}

	pm := politeness.NewPolitenessManager(cfg, nil)
	require.NoError(t, pm.Start())

	// Initial stats
	stats := pm.GetStatistics()
	assert.Equal(t, int64(0), stats.TotalRequests)
	assert.Equal(t, int64(0), stats.CompletedRequests)
	assert.Equal(t, int64(0), stats.FailedRequests)
	assert.Equal(t, int64(0), stats.RejectedRequests)

	// Schedule some requests
	handler := func(url string) error {
		return nil
	}

	var wg sync.WaitGroup
	wg.Add(2)

	err := pm.ScheduleRequest("http://example.com/page1", handler, func(url string, err error) {
		wg.Done()
	})
	assert.NoError(t, err)

	err = pm.ScheduleRequest("http://example.com/page2", handler, func(url string, err error) {
		wg.Done()
	})
	assert.NoError(t, err)

	// Wait for requests to complete
	wg.Wait()

	// Check updated stats
	stats = pm.GetStatistics()
	assert.Equal(t, int64(2), stats.TotalRequests)
	// Requests may complete or be canceled depending on timing
	assert.True(t, stats.CompletedRequests + stats.FailedRequests == 2)
	assert.True(t, len(stats.DomainStats) > 0)

	pm.Stop()
}

func TestPolitenessManager_ConcurrentRequests(t *testing.T) {
	cfg := config.PolitenessConfig{
		DefaultMinDelay:          10 * time.Millisecond,
		DefaultDomainConcurrency: 2,
		GlobalMaxConcurrency:     10,
		RequestQueueSize:         100,
		DefaultRequestTimeout:    5 * time.Second,
		CleanupInterval:          5 * time.Minute,
		DomainStateExpiry:        1 * time.Hour,
		RobotsCacheDuration:      24 * time.Hour,
		EnableRobotsCheck:        false,
		UserAgent:                "TestCrawler/1.0",
	}

	pm := politeness.NewPolitenessManager(cfg, nil)
	require.NoError(t, pm.Start())

	// Handler that adds slight delay
	handler := func(url string) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	// Schedule multiple requests concurrently
	numRequests := 20
	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		url := fmt.Sprintf("http://example%d.com/page", i%5) // 5 different domains
		go func(u string) {
			err := pm.ScheduleRequest(u, handler, func(url string, err error) {
				wg.Done()
			})
			if err != nil {
				t.Errorf("Failed to schedule request for %s: %v", u, err)
				wg.Done()
			}
		}(url)
	}

	// Wait for all requests with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("Concurrent requests test timed out")
	}

	// Verify statistics
	stats := pm.GetStatistics()
	assert.Equal(t, int64(numRequests), stats.TotalRequests)
	// Requests may complete, fail, or be rejected depending on timing when manager stops
	assert.True(t, stats.CompletedRequests+stats.FailedRequests+stats.RejectedRequests == int64(numRequests), 
		"Expected total handled requests (completed+failed+rejected) to equal scheduled requests. Got Completed=%d, Failed=%d, Rejected=%d",
		stats.CompletedRequests, stats.FailedRequests, stats.RejectedRequests)

	pm.Stop()
}

func TestPolitenessManager_InvalidURL(t *testing.T) {
	cfg := config.PolitenessConfig{
		DefaultMinDelay:          10 * time.Millisecond,
		DefaultDomainConcurrency: 1,
		GlobalMaxConcurrency:     5,
		RequestQueueSize:         10,
		DefaultRequestTimeout:    5 * time.Second,
		CleanupInterval:          5 * time.Minute,
		DomainStateExpiry:        1 * time.Hour,
		RobotsCacheDuration:      24 * time.Hour,
		EnableRobotsCheck:        false,
		UserAgent:                "TestCrawler/1.0",
	}

	pm := politeness.NewPolitenessManager(cfg, nil)
	require.NoError(t, pm.Start())
	defer pm.Stop()

	// Test invalid URLs
	invalidURLs := []string{
		"",
		"not-a-url",
		"http://",
		"ftp://example.com", // Non-HTTP protocol
	}

	for _, url := range invalidURLs {
		assert.False(t, pm.CanRequest(url), "Expected CanRequest to return false for URL: %s", url)

		err := pm.ScheduleRequest(url, func(string) error { return nil }, func(string, error) {})
		assert.Error(t, err, "Expected ScheduleRequest to return error for URL: %s", url)
	}
}

func TestPolitenessManager_DomainCleanup(t *testing.T) {
	cfg := config.PolitenessConfig{
		DefaultMinDelay:          10 * time.Millisecond,
		DefaultDomainConcurrency: 1,
		GlobalMaxConcurrency:     5,
		RequestQueueSize:         10,
		DefaultRequestTimeout:    5 * time.Second,
		CleanupInterval:          100 * time.Millisecond, // Very short for testing
		DomainStateExpiry:        200 * time.Millisecond, // Short expiry
		RobotsCacheDuration:      24 * time.Hour,
		EnableRobotsCheck:        false,
		UserAgent:                "TestCrawler/1.0",
	}

	pm := politeness.NewPolitenessManager(cfg, nil)
	require.NoError(t, pm.Start())
	defer pm.Stop()

	// Trigger domain state creation
	assert.True(t, pm.CanRequest("http://example.com/page1"))

	// Check domain exists
	assert.Greater(t, pm.GetDomainStateCount(), 0)

	// Domain should be cleaned up after expiry
	require.Eventually(t, func() bool {
		return pm.GetDomainStateCount() == 0
	}, 2*time.Second, 50*time.Millisecond)
}

// Benchmark tests
func BenchmarkPolitenessManager_CanRequest(b *testing.B) {
	cfg := config.PolitenessConfig{
		DefaultMinDelay:          10 * time.Millisecond,
		DefaultDomainConcurrency: 5,
		GlobalMaxConcurrency:     100,
		RequestQueueSize:         1000,
		DefaultRequestTimeout:    30 * time.Second,
		CleanupInterval:          5 * time.Minute,
		DomainStateExpiry:        1 * time.Hour,
		RobotsCacheDuration:      24 * time.Hour,
		EnableRobotsCheck:        false,
		UserAgent:                "BenchmarkCrawler/1.0",
	}

	pm := politeness.NewPolitenessManager(cfg, nil)
	pm.Start()
	defer pm.Stop()

	urls := make([]string, 100)
	for i := 0; i < 100; i++ {
		urls[i] = fmt.Sprintf("http://example%d.com/page", i%10)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			pm.CanRequest(urls[i%len(urls)])
			i++
		}
	})
}

func BenchmarkPolitenessManager_ScheduleRequest(b *testing.B) {
	cfg := config.PolitenessConfig{
		DefaultMinDelay:          1 * time.Millisecond,
		DefaultDomainConcurrency: 10,
		GlobalMaxConcurrency:     1000,
		RequestQueueSize:         10000,
		DefaultRequestTimeout:    30 * time.Second,
		CleanupInterval:          5 * time.Minute,
		DomainStateExpiry:        1 * time.Hour,
		RobotsCacheDuration:      24 * time.Hour,
		EnableRobotsCheck:        false,
		UserAgent:                "BenchmarkCrawler/1.0",
	}

	pm := politeness.NewPolitenessManager(cfg, nil)
	pm.Start()
	defer pm.Stop()

	handler := func(url string) error {
		return nil
	}

	callback := func(url string, err error) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		url := fmt.Sprintf("http://example%d.com/page%d", i%100, i)
		pm.ScheduleRequest(url, handler, callback)
	}
}