// Package tasks provides the ability to schedule cloud tasks.
package tasks

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"go.alis.build/alog"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	client   *cloudtasks.Client
	project  = os.Getenv("ALIS_OS_PROJECT")
	location = os.Getenv("ALIS_REGION")
)

func init() {
	ctx := context.Background()
	var err error
	client, err = cloudtasks.NewClient(ctx)
	if err != nil {
		alog.Fatalf(ctx, "failed to create cloud tasks client: %v", err)
	}
}

type Task struct {
	URL                 string            // Required https url
	Method              string            // Required: GET/POST/PUT/DELETE/PATCH/OPTIONS
	Headers             map[string]string // Optional http headers
	Body                []byte            // Optional http body
	Time                time.Time         // When the task should run
	ServiceAccountEmail string            // If empty, its derived from the ALIS_OS_PROJECT env (if set).
}

// Schedule the given task.
//
// Queue can be the queue ID, in which case ALIS_REGION and ALIS_OS_PROJECT envs
// are used to determine the full queue name. Otherwise, queue must be in the format
// projects/{project}/locations/{location}/queues/{queue}.
func (t *Task) Schedule(ctx context.Context, queue string) error {
	// validate and prepare values
	if t.URL == "" {
		return fmt.Errorf("missing URL")
	}
	method, err := t.cloudTasksMethod()
	if err != nil {
		return err
	}
	queue, err = t.queueName(queue)
	if err != nil {
		return err
	}

	// create the task
	req := t.request(queue, method)
	if _, err := client.CreateTask(ctx, req); err != nil {
		return err
	}
	return nil
}

// MustSchedule does the same as Schedule, but panics on an error.
func (t *Task) MustSchedule(ctx context.Context, queue string) {
	err := t.Schedule(ctx, queue)
	if err != nil {
		panic(fmt.Sprintf("tasks.MustSchedule: %v", err))
	}
}

func (t *Task) cloudTasksMethod() (cloudtaskspb.HttpMethod, error) {
	switch t.Method {
	case "":
		return 0, fmt.Errorf("missing method")
	case "GET":
		return cloudtaskspb.HttpMethod_GET, nil
	case "POST":
		return cloudtaskspb.HttpMethod_POST, nil
	case "PUT":
		return cloudtaskspb.HttpMethod_PUT, nil
	case "DELETE":
		return cloudtaskspb.HttpMethod_DELETE, nil
	case "PATCH":
		return cloudtaskspb.HttpMethod_PATCH, nil
	case "OPTIONS":
		return cloudtaskspb.HttpMethod_OPTIONS, nil
	default:
		return 0, fmt.Errorf("invalid method %q", t.Method)
	}
}

func (t *Task) queueName(queue string) (string, error) {
	if !strings.HasPrefix(queue, "projects/") {
		if project == "" {
			return "", fmt.Errorf("missing ALIS_OS_PROJECT env")
		}
		if location == "" {
			return "", fmt.Errorf("missing ALIS_REGION env")
		}
		if location == "africa-south1" {
			location = "europe-west1"
		}
		queue = fmt.Sprintf("projects/%s/locations/%s/queues/%s", project, location, queue)
	}
	return queue, nil
}

func (t *Task) request(queue string, method cloudtaskspb.HttpMethod) *cloudtaskspb.CreateTaskRequest {
	req := &cloudtaskspb.CreateTaskRequest{
		Parent: queue,
		Task: &cloudtaskspb.Task{
			MessageType: &cloudtaskspb.Task_HttpRequest{
				HttpRequest: &cloudtaskspb.HttpRequest{
					Url:        t.URL,
					HttpMethod: method,
					Headers:    t.Headers,
					Body:       t.Body,
				},
			},
			ScheduleTime:     timestamppb.New(t.Time),
			DispatchDeadline: durationpb.New(30 * time.Minute),
		},
	}

	// set authorization header if any
	if t.ServiceAccountEmail == "" && project != "" {
		t.ServiceAccountEmail = fmt.Sprintf("alis-build@%s.iam.gserviceaccount.com", project)
	}
	if t.ServiceAccountEmail != "" {
		req.Task.GetHttpRequest().AuthorizationHeader = &cloudtaskspb.HttpRequest_OidcToken{
			OidcToken: &cloudtaskspb.OidcToken{
				ServiceAccountEmail: t.ServiceAccountEmail,
			},
		}
	}
	return req
}
