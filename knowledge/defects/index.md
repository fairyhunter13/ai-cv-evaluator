# Defect

* [make test-e2e runs zero tests and exits 0](make-test-e2e-runs-nothing.md) - Every E2E target filters on -run TestE2E_Core_RateLimitFriendly$, a name no test has, so the README's E2E command passes vacuously.
* [Scores are fabricated from unrelated fields when the model omits them](scores-are-fabricated-when-the-model-omits-them.md) - calculateCVMatchRateFromAnalysis and calculateProjectScoreFromAnalysis derive a score from skill counts and complexity words, and it is returned as if the model produced it.
* [The disk-full outage, and the five safeguards it bought](the-disk-full-outage.md) - Image cleanup used double-quoted awk "{print $2}", so the shell expanded $2 to empty and nothing was ever pruned; 216 images filled the 40 GB disk.
