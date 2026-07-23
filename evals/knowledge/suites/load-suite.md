---
title: load suites
description: Load suite builder and helper usage.
tags: [load, loadgen]
---

# Load suites

Load cases receive `*evals.LoadResult`.

The result builder accepts protobuf-native summaries, SLO checks, tags, and
diagnostic snapshots:

```go
suite := evals.NewLoadSuite("checkout-capacity").
    AddCase("steady-traffic", func(ctx context.Context, r *evals.LoadResult) {
        profile := loadgen.Profile{QPS: 100, Concurrency: 25, Duration: time.Minute}
        metrics, err := loadgen.New().Run(ctx, profile, target)
        if err != nil {
            r.Fail(err)
            return
        }
        r.SetSummary(loadgen.Summary(mode, profile, metrics))
        r.AddSLOCheck(checkProto)
    })
```

Default concurrency is one active case. `WithMaxConcurrency` also applies to
load suites, but parallel load cases combine traffic and can distort
measurements. Use it only when combined traffic is intentional.
