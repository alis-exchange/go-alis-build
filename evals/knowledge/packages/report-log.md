---
type: Go Package
title: package evals/report/log
description: Default log reporter — one-line alog summary per completed Run.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/report/log
tags: [package, report, log, reporter]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/report/log` implements the default reporter: a one-line summary of
each completed `Run` via `alog`. Passing runs log at Info; failing runs
at Warn.

# Usage

```go
import logreport "go.alis.build/evals/report/log"

services.TestServiceServer.Reporter = logreport.Reporter{}
```

# Related

* [`report` package](/packages/report.md)
* [Reporter concept](/concepts/reporter.md)
