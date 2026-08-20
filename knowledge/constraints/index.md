# Constraint

* [A failed step changes the scorer, not just the latency](a-failed-step-changes-the-scorer.md) - Any of the four evaluation steps failing degrades to a single-prompt fast path with a different prompt, and nothing stored says which one ran.
* [Container limits are sized to a 1.9 GB server](container-limits-fit-a-1-9gb-server.md) - Production runs on 1.9 GB RAM; limits summed to ~8.7 GB and caused heavy swapping, and were rewritten to ~2.4 GB against measured actuals.
