package asynqadp

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"
	"time"

	"github.com/hibiken/asynq"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"go.opentelemetry.io/otel"
	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
)

// Worker processes background tasks using asynq.
type Worker struct {
    server  *asynq.Server
    mux     *asynq.ServeMux
    ai      domain.AIClient
    q       *qdrantcli.Client
    twoPass bool
    chain   bool
}

// (duplicates removed)

// buildUserWithContext enriches the user prompt with optional RAG context and/or
// chaining via extraction passes. If any step fails, it gracefully falls back
// to the provided defaultUser prompt.
func buildUserWithContext(ctx context.Context, aicl domain.AIClient, q *qdrantcli.Client, cvText, prText, jobDesc, studyBrief string, chain bool, defaultUser string) string {
    if q == nil || aicl == nil {
        return defaultUser
    }
    vecs, err := aicl.Embed(ctx, []string{jobDesc, studyBrief})
    if err != nil || len(vecs) == 0 {
        return defaultUser
    }
    jobCtx := []string{}
    if rs, err2 := q.Search(ctx, "job_description", vecs[0], 12); err2 == nil {
        jobCtx = topTextsByWeight(rs, 6)
    }
    rubricCtx := []string{}
    if len(vecs) > 1 {
        if rs, err2 := q.Search(ctx, "scoring_rubric", vecs[1], 12); err2 == nil {
            rubricCtx = topTextsByWeight(rs, 6)
        }
    }
    if chain {
        out1, err1 := aicl.ChatJSON(ctx, buildCVExtractSystemPrompt(), buildCVExtractUserPrompt(cvText), 512)
        var cvx cvExtractOut
        if err1 == nil {
            if cvx, err1 = parseCVExtract(out1); err1 != nil { slog.Warn("cv extract parse failed", slog.Any("error", err1)) }
        } else { slog.Warn("cv extract call failed", slog.Any("error", err1)) }
        out2, err2 := aicl.ChatJSON(ctx, buildProjectExtractSystemPrompt(), buildProjectExtractUserPrompt(prText), 512)
        var prx projectExtractOut
        if err2 == nil {
            if prx, err2 = parseProjectExtract(out2); err2 != nil { slog.Warn("project extract parse failed", slog.Any("error", err2)) }
        } else { slog.Warn("project extract call failed", slog.Any("error", err2)) }
        if err1 == nil && err2 == nil {
            return buildEvaluateFromExtractsUserPrompt(cvx, prx, jobDesc, studyBrief, jobCtx, rubricCtx)
        }
    }
    if len(jobCtx) > 0 || len(rubricCtx) > 0 {
        return buildUserPromptRAG(cvText, prText, jobDesc, studyBrief, jobCtx, rubricCtx)
    }
    return defaultUser
}

// handleEvaluate contains the core processing logic and is unit-testable.
func handleEvaluate(ctx context.Context, jobs domain.JobRepository, uploads domain.UploadRepository, results domain.ResultRepository, aicl domain.AIClient, q *qdrantcli.Client, p domain.EvaluateTaskPayload, twoPass bool, chain bool) error {
    // Mark processing
    if err := jobs.UpdateStatus(ctx, p.JobID, domain.JobProcessing, nil); err != nil { return err }
    observability.StartProcessingJob("evaluate")
    // Load texts
    cv, err := uploads.Get(ctx, p.CVID)
    if err != nil {
        _ = jobs.UpdateStatus(ctx, p.JobID, domain.JobFailed, strPtr(err.Error()))
        observability.FailJob("evaluate")
        return err
    }
    pr, err := uploads.Get(ctx, p.ProjectID)
    if err != nil {
        _ = jobs.UpdateStatus(ctx, p.JobID, domain.JobFailed, strPtr(err.Error()))
        observability.FailJob("evaluate")
        return err
    }
    if aicl == nil {
        _ = jobs.UpdateStatus(ctx, p.JobID, domain.JobFailed, strPtr("ai client not configured"))
        observability.FailJob("evaluate")
        return nil
    }
    sys := buildSystemPrompt()
    usr := buildUserPrompt(cv.Text, pr.Text, p.JobDescription, p.StudyCaseBrief)
    usr = buildUserWithContext(ctx, aicl, q, cv.Text, pr.Text, p.JobDescription, p.StudyCaseBrief, chain, usr)
    var res llmEvalOut
    var outJSON string
    var lastErr error
    for attempt := 0; attempt < 3; attempt++ {
        outJSON, err = aicl.ChatJSON(ctx, sys, usr, 512)
        if err != nil { lastErr = err; break }
        res, err = parseAndNormalize(outJSON)
        if err == nil { lastErr = nil; break }
        if !isSchemaOrJSONErr(err) { lastErr = err; break }
        sys = buildSystemPrompt() + "\nReminder: Return ONLY valid compact JSON, no markdown, no code fences."
        time.Sleep(time.Duration(200*(attempt+1)) * time.Millisecond)
        lastErr = err
    }
    if lastErr != nil {
        _ = jobs.UpdateStatus(ctx, p.JobID, domain.JobFailed, strPtr(lastErr.Error()))
        observability.FailJob("evaluate")
        return lastErr
    }
    if twoPass {
        sys2 := buildNormalizationSystemPrompt()
        usr2 := buildNormalizationUserPrompt(res)
        out2, err2 := aicl.ChatJSON(ctx, sys2, usr2, 512)
        if err2 == nil {
            if res2, err3 := parseAndNormalize(out2); err3 == nil { res = res2 } else { slog.Warn("normalization pass parse failed; using first pass", slog.Any("error", err3)) }
        } else { slog.Warn("normalization pass failed; using first pass", slog.Any("error", err2)) }
    }
    if err := results.Upsert(ctx, domain.Result{
        JobID: p.JobID,
        CVMatchRate: res.CVMatchRate,
        CVFeedback: res.CVFeedback,
        ProjectScore: res.ProjectScore,
        ProjectFeedback: res.ProjectFeedback,
        OverallSummary: res.OverallSummary,
        CreatedAt: time.Now().UTC(),
    }); err != nil {
        _ = jobs.UpdateStatus(ctx, p.JobID, domain.JobFailed, strPtr(err.Error()))
        observability.FailJob("evaluate")
        return err
    }
    observability.ObserveEvaluation(res.CVMatchRate, res.ProjectScore)
    if err := jobs.UpdateStatus(ctx, p.JobID, domain.JobCompleted, nil); err != nil { return err }
    observability.CompleteJob("evaluate")
    slog.Info("job completed", slog.String("job_id", p.JobID))
    return nil
}

// NewWorker constructs a Worker with the provided repositories, AI client and Qdrant client.
func NewWorker(redisURL string, jobs domain.JobRepository, uploads domain.UploadRepository, results domain.ResultRepository, aicl domain.AIClient, qcli *qdrantcli.Client, twoPass bool, chain bool) (*Worker, error) {
	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil { return nil, err }
	srv := asynq.NewServer(opt, asynq.Config{Concurrency: 5})
	mux := asynq.NewServeMux()
	worker := &Worker{server: srv, mux: mux, ai: aicl, q: qcli, twoPass: twoPass, chain: chain}

	mux.HandleFunc(TaskEvaluate, func(ctx context.Context, t *asynq.Task) error {
        tracer := otel.Tracer("queue.worker")
        ctx, span := tracer.Start(ctx, "EvaluateJob")
        defer span.End()
        var p domain.EvaluateTaskPayload
        if err := json.Unmarshal(t.Payload(), &p); err != nil { return err }
        return handleEvaluate(ctx, jobs, uploads, results, worker.ai, worker.q, p, worker.twoPass, worker.chain)
    })

	return worker, nil
}

// Start begins processing tasks until shutdown.
func (w *Worker) Start(_ context.Context) error { return w.server.Start(w.mux) }
// Stop gracefully shuts down the worker server.
func (w *Worker) Stop() { w.server.Shutdown() }

func strPtr(s string) *string { return &s }

// topTextsByWeight sorts results by payload.weight (desc) if present, otherwise keeps original order.
// It returns up to top texts with duplicates removed.
func topTextsByWeight(rs []map[string]any, top int) []string {
    type item struct{ t string; w float64; hasW bool }
    items := make([]item, 0, len(rs))
    for _, r := range rs {
        pl, ok := r["payload"].(map[string]any); if !ok { continue }
        t, ok2 := pl["text"].(string); if !ok2 { continue }
        it := item{t: t}
        if v, ok := pl["weight"]; ok {
            switch vv := v.(type) {
            case float64:
                it.w = vv; it.hasW = true
            case int:
                it.w = float64(vv); it.hasW = true
            }
        }
        items = append(items, it)
    }
    // stable sort: items with weight come first by weight desc, then keep input order
    sort.SliceStable(items, func(i, j int) bool {
        if items[i].hasW && items[j].hasW {
            return items[i].w > items[j].w
        }
        if items[i].hasW != items[j].hasW { return items[i].hasW }
        return false
    })
    out := make([]string, 0, top)
    seen := map[string]struct{}{}
    for _, it := range items {
        if _, ok := seen[it.t]; ok { continue }
        out = append(out, it.t)
        seen[it.t] = struct{}{}
        if len(out) >= top { break }
    }
    return out
}
