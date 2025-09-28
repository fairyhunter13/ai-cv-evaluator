package main

import (
    "context"
    "log"
    "os"

    qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
    realai "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai/real"
    "github.com/fairyhunter13/ai-cv-evaluator/internal/config"
    "github.com/fairyhunter13/ai-cv-evaluator/internal/ragseed"
)

func main() {
    cfg, err := config.Load()
    if err != nil { log.Fatal(err) }
    q := qdrantcli.New(cfg.QdrantURL, cfg.QdrantAPIKey)
    ai := realai.New(cfg)
    ctx := context.Background()
    if err := ragseed.SeedDefault(ctx, q, ai); err != nil {
        log.Fatal(err)
    }
    log.Println("RAG seeds ingested successfully")
}

func getenv(k, def string) string { if v := os.Getenv(k); v != "" { return v }; return def }
