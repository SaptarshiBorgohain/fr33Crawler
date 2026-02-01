package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/knowledge-engine/backend/internal/api"
	"github.com/knowledge-engine/backend/internal/config"
	"github.com/knowledge-engine/backend/internal/engine"
	"github.com/knowledge-engine/backend/internal/fetcher"
	"github.com/knowledge-engine/backend/internal/search"
	"github.com/knowledge-engine/backend/internal/storage"
)

func main() {
	// Setup Logging
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	entry := logger.WithField("service", "crawler-api")

	entry.Info("Starting Knowledge Engine API Service")

	// 1. Config
	cfg := config.Load()
	cfg.Politeness.DefaultDomainConcurrency = 2
	cfg.Politeness.DefaultMinDelay = 1 * time.Second

	// 2. Storage
	dataDir := "./data"
	store, err := storage.NewFileStorage(dataDir)
	if err != nil {
		entry.Fatalf("Failed to initialize storage: %v", err)
	}

	// 3. Search Index (Memory)
	vectorStore := search.NewVectorStore()
	loadExistingData(dataDir, vectorStore, entry)

	// 4. Engine
	eng, err := engine.NewEngine(cfg, entry, store, vectorStore)
	if err != nil {
		entry.Fatalf("Failed to initialize engine: %v", err)
	}

	// 5. API Server
	server := api.NewServer(eng, entry)
	
	entry.Info("Knowledge Engine API ready on port 8080")
	if err := server.Start(":8080"); err != nil {
		entry.Fatal(err)
	}
}

func loadExistingData(dir string, store *search.VectorStore, log *logrus.Entry) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var documents []*search.Document
	count := 0

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}
		
		data, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			continue
		}

		var res fetcher.FetchResult
		if err := json.Unmarshal(data, &res); err == nil {
			documents = append(documents, &search.Document{
				ID:      res.URL,
				Title:   res.Title,
				Content: res.Text,
			})
			count++
		}
	}
	
	if count > 0 {
		store.AddDocuments(documents)
		log.Infof("Pre-loaded %d documents into search index", count)
	}
}
