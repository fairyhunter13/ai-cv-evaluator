"""Attest one candidate evaluation.

Decides whether a stored score is supported by the receipt of the run that
produced it. Never uses an LLM. Never makes network calls. Safe to run
consumer-side.

Nothing writes this receipt yet — that is the point of the concept this
attester belongs to. It is written first so the gap is a failing check rather
than a paragraph.
"""

from __future__ import annotations

RECEIPT_FIELDS = (
    "job_id",
    "git_sha",
    "model_id",
    "provider",
    "prompt_version",
    "path_taken",
    "temperature",
    "max_tokens",
    "attempts",
    "raw_scores",
)

PATHS = ("three-call", "fast-path", "derived")
PLACEHOLDERS = ("No feedback provided", "No summary provided")
RANGES = {"cv_match_rate": (0.0, 1.0), "project_score": (1.0, 10.0)}


def attest(*, sanctioned_models, sanctioned_prompt_version, sanctioned_temperature,
           max_attempts, receipt, stored_result):
    """Return {"ok", "reason", "details"}; reason is None when ok."""

    def no(reason, **details):
        return {"ok": False, "reason": reason, "details": details}

    missing = [f for f in RECEIPT_FIELDS if receipt.get(f) in (None, "")]
    if missing:
        return no("receipt is incomplete", missing=missing)

    if receipt["job_id"] != stored_result.get("job_id"):
        return no("receipt is for a different job", receipt=receipt["job_id"],
                  stored=stored_result.get("job_id"))

    if receipt["model_id"] not in sanctioned_models:
        return no("the model that scored this is not sanctioned",
                  model=receipt["model_id"], provider=receipt["provider"])

    if receipt["prompt_version"] != sanctioned_prompt_version:
        return no("scored against a different prompt", used=receipt["prompt_version"],
                  sanctioned=sanctioned_prompt_version)

    if receipt["temperature"] != sanctioned_temperature:
        return no("scored at a different temperature", used=receipt["temperature"],
                  sanctioned=sanctioned_temperature)

    if receipt["attempts"] > max_attempts:
        return no("more attempts than the retry policy allows",
                  attempts=receipt["attempts"], allowed=max_attempts)

    if receipt["path_taken"] not in PATHS:
        return no("unknown evaluation path", path=receipt["path_taken"])

    # A derived score was never assessed: the model omitted it and the handler
    # computed one. Reporting it as a failed attestation is the point.
    if receipt["path_taken"] == "derived":
        return no("the score was derived, not assessed", path=receipt["path_taken"])

    for field, (lo, hi) in RANGES.items():
        raw = receipt["raw_scores"].get(field)
        if raw is None:
            return no("receipt carries no pre-clamp score", field=field)
        if not lo <= raw <= hi:
            return no("the model returned a score outside its range, and clamping hid it",
                      field=field, raw=raw, allowed=[lo, hi])
        if stored_result.get(field) != raw:
            return no("the stored score is not the one the model returned",
                      field=field, raw=raw, stored=stored_result.get(field))

    for field in ("cv_feedback", "project_feedback", "overall_summary"):
        text = (stored_result.get(field) or "").strip()
        if not text or text in PLACEHOLDERS:
            return no("a text field is empty or a placeholder", field=field)

    return {"ok": True, "reason": None,
            "details": {"model_id": receipt["model_id"], "git_sha": receipt["git_sha"],
                        "max_tokens": receipt["max_tokens"]}}
