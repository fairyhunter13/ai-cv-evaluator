package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"gopkg.in/yaml.v3"
)

type ragYAML struct {
	Items []string        `yaml:"items"`
	Texts []string        `yaml:"texts"`
	Data  []ragYAMLItem   `yaml:"data"`
}

type ragYAMLItem struct {
	Text    string   `yaml:"text"`
	Type    string   `yaml:"type"`
	Section string   `yaml:"section"`
	Weight  float64  `yaml:"weight"`
}

func seedQdrantFromYAML(ctx domain.Context, q *qdrantcli.Client, ai domain.AIClient, path string, collection string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) { return fmt.Errorf("seed file not found: %s", path) }
		return err
	}
	// Try multiple shapes
	var doc ragYAML
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("yaml parse: %w", err)
	}
	texts := make([]string, 0, len(doc.Items)+len(doc.Texts)+len(doc.Data))
	if len(doc.Items) > 0 { texts = append(texts, doc.Items...) }
	if len(doc.Texts) > 0 { texts = append(texts, doc.Texts...) }
	for _, it := range doc.Data { if s := strings.TrimSpace(it.Text); s != "" { texts = append(texts, s) } }
	if len(texts) == 0 {
		// Try raw YAML as list of strings
		var ls []string
		if err := yaml.Unmarshal(b, &ls); err == nil { texts = append(texts, ls...) }
	}
	if len(texts) == 0 { return fmt.Errorf("no texts to seed in %s", path) }

	// Batch embed and upsert
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
			// try to find if this chunk came from Data to attach metadata
			// naive match by exact text
			for _, it := range doc.Data {
				if strings.TrimSpace(it.Text) == strings.TrimSpace(chunk[j]) {
					if it.Type != "" { p["type"] = it.Type }
					if it.Section != "" { p["section"] = it.Section }
					if it.Weight > 0 { p["weight"] = it.Weight }
					break
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
