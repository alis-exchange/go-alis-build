# tasks

The `tasks` package provides a simple and convenient way to schedule HTTP tasks using Google Cloud Tasks. It abstracts away the boilerplate of setting up the Cloud Tasks client and constructing the protobuf requests.

## Features
- **Automatic Client Initialization**: Automatically creates a Google Cloud Tasks client in the background on initialization.
- **Simplified Scheduling**: Provides `Task.Schedule` and `Task.MustSchedule` for easy scheduling.
- **Environment Variable Integration**: Uses `ALIS_OS_PROJECT` and `ALIS_REGION` to automatically resolve queue names and default service accounts.
- **Built-in Authentication**: Automatically attaches an OIDC token to requests for secure communication, defaulting to `alis-build@{ALIS_OS_PROJECT}.iam.gserviceaccount.com` if no custom service account is provided.

## Environment Variables

This package relies on the following environment variables (especially when using short queue names):

- `ALIS_OS_PROJECT`: The Google Cloud project ID. Used to construct the default full queue name and the default service account email.
- `ALIS_REGION`: The Google Cloud region. Used to construct the default full queue name. *Note: If this is set to `africa-south1`, it automatically defaults to `europe-west1`.*

## Usage

### Basic Example

```go
package main

import (
	"context"
	"time"

	"go.alis.build/tasks"
)

func main() {
	ctx := context.Background()

	t := tasks.Task{
		URL:    "https://api.example.com/v1/process",
		Method: "POST",
		Body:   []byte(`{"key": "value"}`),
		Time:   time.Now().Add(5 * time.Minute), // Run 5 minutes from now
	}

	// Schedule to a queue named "my-queue". 
	// The package will construct the full queue path: 
	// projects/{ALIS_OS_PROJECT}/locations/{ALIS_REGION}/queues/my-queue
	err := t.Schedule(ctx, "my-queue")
	if err != nil {
		panic(err)
	}
}
```

### Advanced Example

```go
package main

import (
	"context"
	"time"

	"go.alis.build/tasks"
)

func main() {
	ctx := context.Background()

	t := tasks.Task{
		URL:    "https://api.example.com/v1/update",
		Method: "PATCH",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body:                []byte(`{"status": "completed"}`),
		Time:                time.Now().Add(1 * time.Hour),
		ServiceAccountEmail: "my-custom-sa@my-project.iam.gserviceaccount.com", // Override default service account
	}

	// You can also provide the full queue name directly, bypassing ALIS_REGION and ALIS_OS_PROJECT overrides for the queue path.
	queuePath := "projects/my-custom-project/locations/us-central1/queues/my-custom-queue"
	t.MustSchedule(ctx, queuePath) // Will panic if scheduling fails
}
```
