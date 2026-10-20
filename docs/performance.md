# Performance

Deterministic benchmarks (`make bench`); the numbers below are from an Apple M-series
laptop (arm64, Go 1.27) and exist to compare runs on the same machine, not as promises.

| Benchmark | Result |
| --- | --- |
| lay out a 300-turn timeline from scratch | ~7 ms |
| the same 600-turn timeline per frame with the per-turn layout cache | ~1 ms |

Measures taken: persisted turns are laid out once and reused until the width, a display
setting or the turn changes; image decoding, `@` search, task logs run off
the UI goroutine; renderers never query the database. Targets (p95 ≤ 50 ms for input and
scroll, ≤ 16.7 ms frame work) are workload-specific goals; no real-terminal p95 was
measured for this release, and CI should compare benchmarks relatively, not by wall clock.
