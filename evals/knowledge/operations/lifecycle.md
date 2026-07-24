---
title: lifecycle
description: Runtime lifecycle for typed eval suites.
tags: [lifecycle, run, publish]
---

# Lifecycle

1. Construct a named typed suite.
2. Add named cases with fluent `AddCase`.
3. Run the suite with `Run` or `RunAndPublish`.
4. Receive a materialized `*evalspb.Run`.
5. If using `RunAndPublish`, the selected reporter receives that run.

There is no startup registration or freeze step.

Cases execute synchronously under a bounded scheduler. The scheduler preserves
result order, recovers panics at the case boundary, stops scheduling new cases
when context is cancelled, waits for active cases to return, and returns partial
cancelled runs.

For setup and cleanup, write normal Go around the suite call:

```go
cleanup, err := seed(ctx)
if err != nil {
    return nil, err
}
defer cleanup(context.WithoutCancel(ctx))

return suite.RunAndPublish(ctx)
```
