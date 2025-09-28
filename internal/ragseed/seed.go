// Package ragseed provides helpers to seed Qdrant collections from YAML files.
package ragseed

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"gopkg.in/yaml.v3"
)

type ragYAML struct {
	Items []string      `yaml:"items"`
	Texts []string      `yaml:"texts"`
	Data  []ragYAMLItem `yaml:"data"`
}

type ragYAMLItem struct {
	Text    string  `yaml:"text"`
	Type    string  `yaml:"type"`
	Section string  `yaml:"section"`
	Weight  float64 `yaml:"weight"`
}

// SeedFile ingests a single YAML seed file into the given collection.
func SeedFile(ctx domain.Context, q *qdrantcli.Client, ai domain.AIClient, path string, collection string) error {
	// Mitigate file inclusion issues by constraining to current working directory.
	abs, err := filepath.Abs(path)
	if err != nil { return err }
	wd, err := os.Getwd()
	if err != nil { return err }
	abs = filepath.Clean(abs)
	wd = filepath.Clean(wd)
	if os.Getenv("RAGSEED_ALLOW_ABSPATHS") != "1" {
		if !strings.HasPrefix(abs, wd+string(os.PathSeparator)) && abs != wd {
			return fmt.Errorf("disallowed path: %s", abs)
		}
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) { return fmt.Errorf("seed file not found: %s", path) }
		return err
	}
	var doc ragYAML
	if err := yaml.Unmarshal(b, &doc); err != nil {
		// Fallback: try simple list of strings
		var ls []string
		if err2 := yaml.Unmarshal(b, &ls); err2 != nil {
			return fmt.Errorf("yaml parse: %w", err)
		}
		if len(ls) == 0 { return fmt.Errorf("no texts to seed in %s", path) }
		return upsertAll(ctx, q, ai, collection, ls, nil)
	}
	texts := make([]string, 0, len(doc.Items)+len(doc.Texts)+len(doc.Data))
	if len(doc.Items) > 0 { texts = append(texts, doc.Items...) }
	if len(doc.Texts) > 0 { texts = append(texts, doc.Texts...) }
	meta := make(map[string]ragYAMLItem)
	for _, it := range doc.Data { if s := strings.TrimSpace(it.Text); s != "" { texts = append(texts, s); meta[s] = it } }
	if len(texts) == 0 {
		var ls []string
		if err := yaml.Unmarshal(b, &ls); err == nil { texts = append(texts, ls...) }
	}
	if len(texts) == 0 { return fmt.Errorf("no texts to seed in %s", path) }

	return upsertAll(ctx, q, ai, collection, texts, meta)
}

// SeedDefault seeds both job_description and scoring_rubric from default file paths.
func SeedDefault(ctx domain.Context, q *qdrantcli.Client, ai domain.AIClient) error {
	if err := SeedFile(ctx, q, ai, "configs/rag/job_description.yaml", "job_description"); err != nil {
		return err
	}
	if err := SeedFile(ctx, q, ai, "configs/rag/scoring_rubric.yaml", "scoring_rubric"); err != nil {
		return err
	}
	return nil
}

// upsertAll embeds and upserts texts with optional metadata mapping
func upsertAll(ctx domain.Context, q *qdrantcli.Client, ai domain.AIClient, collection string, texts []string, meta map[string]ragYAMLItem) error {
	const batch = 16
	for i := 0; i < len(texts); i += batch {
		end := i + batch
		if end > len(texts) { end = len(texts) }
		chunk := texts[i:end]
		vecs, err := ai.Embed(ctx, chunk)
		if err != nil { return fmt.Errorf("embed: %w", err) }
		payloads := make([]map[string]any, len(chunk))
		for j := range chunk {
			p := map[string]any{"text": chunk[j], "source": collection}
			if meta != nil {
				if it, ok := meta[strings.TrimSpace(chunk[j])]; ok {
					if it.Type != "" { p["type"] = it.Type }
					if it.Section != "" { p["section"] = it.Section }
					if it.Weight > 0 { p["weight"] = it.Weight }
				}
			}
			payloads[j] = p
		}
		if err := q.UpsertPoints(ctx, collection, vecs, payloads, nil); err != nil {
			return fmt.Errorf("qdrant upsert: %w", err)
		}
	}
	return nil
}
