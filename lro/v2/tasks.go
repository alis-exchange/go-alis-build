package lro

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"github.com/google/uuid"
	"github.com/googleapis/gax-go/v2"
	"go.alis.build/alog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type cloudTasksClient interface {
	CreateTask(context.Context, *cloudtaskspb.CreateTaskRequest, ...gax.CallOption) (*cloudtaskspb.Task, error)
	Close() error
}

type queue struct {
	name                string
	serviceAccountEmail string
	client              cloudTasksClient
	taskDeadline        time.Duration
	retryBackoff        gax.Backoff
	// createTaskTimeout bounds one CreateTask attempt. Cloud Tasks rejects a
	// request whose deadline is more than 30s out ("The deadline cannot be more
	// than 30s in the future"), so every attempt carries its own short deadline
	// instead of whatever the caller's context happened to hold.
	createTaskTimeout time.Duration
	// scheduleBudget bounds the whole set of attempts for one schedule.
	scheduleBudget time.Duration
}

const (
	defaultCreateTaskTimeout = 20 * time.Second
	defaultScheduleBudget    = 30 * time.Second
)

var supportedCloudTasksLocations = map[string]struct{}{
	"northamerica-northeast1": {},
	"southamerica-east1":      {},
	"us-central1":             {},
	"us-east1":                {},
	"us-east4":                {},
	"us-west1":                {},
	"us-west2":                {},
	"us-west3":                {},
	"us-west4":                {},
	"europe-central2":         {},
	"europe-west1":            {},
	"europe-west2":            {},
	"europe-west3":            {},
	"europe-west6":            {},
	"asia-east1":              {},
	"asia-east2":              {},
	"asia-northeast1":         {},
	"asia-northeast2":         {},
	"asia-northeast3":         {},
	"asia-south1":             {},
	"asia-southeast1":         {},
	"asia-southeast2":         {},
	"australia-southeast1":    {},
}

var cloudTasksLocationFallbacks = map[string]string{
	// Cloud Tasks is not available in africa-south1. Route via the closest supported European region.
	"africa-south1": "europe-west1",
}

// newQueue constructs the Cloud Tasks queue client used to resume operations.
func newQueue(ctx context.Context, cfg Config) (*queue, error) {
	tasksClient, err := cloudtasks.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create cloud tasks client: %w", err)
	}

	return &queue{
		name:                fmt.Sprintf("projects/%s/locations/%s/queues/%s", cfg.CloudTasksProject, cfg.CloudTasksLocation, cfg.CloudTasksQueue),
		serviceAccountEmail: cfg.CloudTasksServiceAccount,
		client:              tasksClient,
		taskDeadline:        30 * time.Minute,
		retryBackoff: gax.Backoff{
			Initial:    100 * time.Millisecond,
			Max:        5 * time.Second,
			Multiplier: 2,
		},
		createTaskTimeout: defaultCreateTaskTimeout,
		scheduleBudget:    defaultScheduleBudget,
	}, nil
}

// scheduleCloudTask creates a Cloud Tasks HTTP task for the supplied callback URL.
//
// The caller's context is used for attribution only. Scheduling runs detached
// from it: an RPC caller's deadline or disconnect decides nothing about whether
// the operation continues, and Cloud Tasks would reject a forwarded deadline
// longer than 30s anyway. Every attempt gets its own short deadline, every
// error is retried until the schedule budget runs out, and each failed attempt
// is logged with the status details Cloud Tasks returned.
func (q *queue) scheduleCloudTask(ctx context.Context, url string, scheduleTime time.Time) error {
	// Supplying a task name makes CreateTask idempotent. If Cloud Tasks accepts a
	// request but its response is lost, the retry receives AlreadyExists instead
	// of creating a duplicate callback.
	taskName := q.name + "/tasks/" + uuid.NewString()
	req := &cloudtaskspb.CreateTaskRequest{
		Parent: q.name,
		Task: &cloudtaskspb.Task{
			Name: taskName,
			MessageType: &cloudtaskspb.Task_HttpRequest{
				HttpRequest: &cloudtaskspb.HttpRequest{
					Url:        url,
					HttpMethod: cloudtaskspb.HttpMethod_PUT,
					AuthorizationHeader: &cloudtaskspb.HttpRequest_OidcToken{
						OidcToken: &cloudtaskspb.OidcToken{
							ServiceAccountEmail: q.serviceAccountEmail,
						},
					},
				},
			},
			ScheduleTime:     timestamppb.New(scheduleTime),
			DispatchDeadline: durationpb.New(q.taskDeadline),
		},
	}

	ctx = context.WithoutCancel(ctx)
	budgetCtx, cancel := context.WithTimeout(ctx, q.scheduleBudget)
	defer cancel()

	backoff := q.retryBackoff
	var lastErr error
	for attempt := 1; ; attempt++ {
		attemptCtx, cancelAttempt := context.WithTimeout(budgetCtx, q.createTaskTimeout)
		_, err := q.client.CreateTask(attemptCtx, req)
		cancelAttempt()
		if err == nil || status.Code(err) == codes.AlreadyExists {
			return nil
		}
		alog.Warnf(ctx, "lro: creating cloud task %s (attempt %d) failed: %v%s", taskName, attempt, err, statusDetails(err))
		// Keep the last answer Cloud Tasks actually gave; a budget expiry
		// mid-call only says the attempts ran out.
		if budgetCtx.Err() == nil || lastErr == nil {
			lastErr = err
		}

		select {
		case <-budgetCtx.Done():
			return fmt.Errorf("after %d attempt(s) over %s: %w", attempt, q.scheduleBudget, lastErr)
		case <-time.After(backoff.Pause()):
		}
	}
}

// statusDetails renders the rich error details of a gRPC status, if any, so a
// rejection such as a bad field or an over-long deadline is named in the log.
func statusDetails(err error) string {
	details := status.Convert(err).Details()
	if len(details) == 0 {
		return ""
	}
	return fmt.Sprintf(" details=%v", details)
}

// resolveCloudTasksLocation returns a supported Cloud Tasks region for the deployment region.
func resolveCloudTasksLocation(location string) (string, error) {
	if _, ok := supportedCloudTasksLocations[location]; ok {
		return location, nil
	}
	if fallback, ok := cloudTasksLocationFallbacks[location]; ok {
		return fallback, nil
	}

	supported := make([]string, 0, len(supportedCloudTasksLocations))
	for region := range supportedCloudTasksLocations {
		supported = append(supported, region)
	}
	sort.Strings(supported)

	return "", fmt.Errorf(
		"ALIS_REGION %q is not a supported Cloud Tasks location and no fallback is configured; supported locations: %s",
		location,
		strings.Join(supported, ", "),
	)
}
