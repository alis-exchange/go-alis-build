# Long-Running Operations (LROs)

## Background

Long-running operations (LROs) are a pattern in resource-driven design where an operation takes an extended period of
time to complete. This can be due to a number of factors, such as the size of the operation, the amount of data
involved, or the availability of resources.

LROs are often used for tasks such as:

- Copying large amounts of data
- Processing large amounts of data
- Running complex queries
- Executing long-running tasks

LROs can be implemented in a number of ways, but the most common approach is to use a client-server model.
In this model, the client initiates the operation and the server performs the operation in the background.
The client can then poll the server to check the status of the operation until it is complete.

More details on LROs are available at: https://google.aip.dev/151

This package makes managing LROs on your server super simple. 😎

## Using Terraform for setting up Spanner?

Here is an example terraform you could use when setting up your underlying Spanner database

```tf

# Create a Spanner Table
resource "alis_google_spanner_table" "operations" {
  project  = var.ALIS_MANAGED_SPANNER_PROJECT
  instance = var.ALIS_MANAGED_SPANNER_INSTANCE
  database = var.ALIS_MANAGED_SPANNER_DB
  name     = "${replace(var.ALIS_OS_PROJECT, "-", "_")}_Operations"
  schema = {
    columns = [
      {
        name            = "key",
        is_computed     = true,
        computation_ddl = "Operation.name",
        type            = "STRING",
        is_primary_key  = true,
        required        = true,
        unique          = true,
      },
      {
        name          = "Operation"
        type          = "PROTO"
        proto_package = "google.longrunning.Operation"
        required      = true
      },
      {
        name          = "Checkpoint"
        type          = "BYTES"
        required      = false
      },
    ]
  }
}

```

## Cloud Workflows

Here is an example yaml you could use to provision and manage your workflow responsible for resuming LROs (if used)

```yaml
main:
  params: [args]
  steps:
    - init:
        assign:
          - operationId: ${args.operationId}
          - operations: ${args.operations}
          - method: ${args.method}
          - timeout: ${args.timeout}
          - pollFrequency: ${args.pollFrequency}
    - wait:
        parallel:
          for:
            value: op
            in: ${args.operations}
            steps:
              - log:
                  call: sys.log
                  args:
                    data: ${"polling " + op}
              - startCounter:
                  assign:
                    - counter: 0
              - poll:
                  try:
                    call: http.post
                    args:
                      url: https://internal-gateway-...-ew.a.run.app/.../GetOperation
                      auth:
                        type: OIDC
                        audience: https://internal-gateway-...-ew.a.run.app
                      body:
                        name: ${op}
                    result: operation
                  retry:
                    predicate: ${custom_predicate}
                    max_retries: 7
                    backoff:
                      initial_delay: 1
                      max_delay: 60
                      multiplier: 2
                  except:
                    as: e
                    steps:
                      - return_run_step_error:
                          raise: ${e}
              - checkStatus:
                  switch:
                    - condition: ${not("done" in operation.body)}
                      next: sleep
                    - condition: true
                      next: done
              - sleep:
                  call: sys.sleep
                  args:
                    seconds: ${pollFrequency}
              - increment:
                  assign:
                    - counter: ${counter+pollFrequency}
              - timeoutCheck:
                  switch:
                    - condition: ${counter > timeout}
                      next: timedOut
                  next: poll
              - timedOut:
                  call: sys.log
                  args:
                    data: ${op + " timed out"}
                  next: continue
              - done:
                  call: sys.log
                  args:
                    data: ${op + " completed"}
    - resumeMethod:
        call: http.post
        args:
          url: ${"https://internal-gateway-...-ew.a.run.app/" + method}
          headers:
            x-alis-operation-id: ${operationId}
          body:
            data: {}
          auth:
            type: OIDC
            audience: https://internal-gateway-...-ew.a.run.app

# A fix to avoid connection related issues with hitting the LRO service
custom_predicate:
  params: [e]
  steps:
    - handle:
        switch:
          - condition: ${e.message == "connection broken"}
            return: true
    - dont_handle:
        return: false
```
