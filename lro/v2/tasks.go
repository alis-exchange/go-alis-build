package lro

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"go.alis.build/alog"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type queue struct {
	name                string
	serviceAccountEmail string
	client              *cloudtasks.Client
	taskDeadline        time.Duration
}

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
	}, nil
}

// schedulePutRequest schedules a PUT callback, simulating it locally when not running on Cloud Run.
func (q *queue) schedulePutRequest(ctx context.Context, mux *http.ServeMux, url string, scheduleTime time.Time) error {
	if os.Getenv("K_SERVICE") == "" {
		return q.simulatePutRequest(mux, url, scheduleTime)
	}
	return q.scheduleCloudTask(ctx, url, scheduleTime)
}

// scheduleCloudTask creates a Cloud Tasks HTTP task for the supplied callback URL.
func (q *queue) scheduleCloudTask(ctx context.Context, url string, scheduleTime time.Time) error {
	req := &cloudtaskspb.CreateTaskRequest{
		Parent: q.name,
		Task: &cloudtaskspb.Task{
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

	_, err := q.client.CreateTask(ctx, req)
	return err
}

// simulatePutRequest invokes the callback handler locally after the requested delay.
func (q *queue) simulatePutRequest(mux *http.ServeMux, url string, scheduleTime time.Time) error {
	go func() {
		time.Sleep(time.Until(scheduleTime))

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(q.taskDeadline))
		defer cancel()

		r, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
		if err != nil {
			alog.Errorf(ctx, "creating http request for local task simulation: %v", err)
			return
		}

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			alog.Errorf(ctx, "unexpected local task simulation response: %d %s", w.Code, w.Body.String())
		}
	}()
	return nil
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
