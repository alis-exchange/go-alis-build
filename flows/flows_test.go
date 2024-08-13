package flows

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	flows "open.alis.services/protobuf/alis/open/flows/v1"
)

var (
	project    string
	topic      string
	parentFlow string
)

func init() {
	project = "macquarie-mr-dev-19e"
	parentFlow = ""
}

// mockServerTransportStream implements the gRPC ServerTransportStream interface
type mockServerTransportStream struct {
	// You can store metadata here if needed
	headers  metadata.MD
	trailers metadata.MD
}

// Method returns the name of the method being invoked
func (m *mockServerTransportStream) Method() string {
	return "foo"
}

// SetHeader sets the headers to be sent to the client
func (m *mockServerTransportStream) SetHeader(md metadata.MD) error {
	m.headers = md
	fmt.Println("SetHeader called with:", md)
	return nil
}

// SendHeader sends the headers to the client
func (m *mockServerTransportStream) SendHeader(md metadata.MD) error {
	// Combine existing headers with new ones if necessary
	for k, v := range md {
		m.headers[k] = v
	}
	fmt.Println("SendHeader called with:", md)
	return nil
}

// SetTrailer sets the trailers (trailing metadata) to be sent to the client
func (m *mockServerTransportStream) SetTrailer(md metadata.MD) error {
	m.trailers = md
	fmt.Println("SetTrailer called with:", md)
	return nil
}

func Test_Client(t *testing.T) {
	ctx := grpc.NewContextWithServerTransportStream(context.Background(), &mockServerTransportStream{})
	if parentFlow != "" {
		ctx = metadata.NewIncomingContext(ctx, metadata.MD{
			FlowParentHeaderKey: []string{parentFlow},
		})
	}

	// Create new client
	client, err := NewClient(project, WithTopic(topic), WithAwaitPublish())
	if err != nil {
		t.Errorf("NewClient() error = %v", err)
		return
	}

	// Ensure client is initialized
	if !assert.NotEmptyf(t, client, "Expected client to be initialized") {
		t.Errorf("NewClient() = %v, want not nil", client)
		return
	}

	// Ensure topic is set correctly
	setTopic := DefaultTopic
	if topic != "" {
		setTopic = topic
	}
	if !assert.Equal(t, setTopic, client.topic, fmt.Sprintf("Expected(%s), Got(%s)", setTopic, client.topic)) {
		t.Errorf("NewClient().topic = %v, want %v", client.topic, setTopic)
	}

	// Ensure pubsub is initialized
	if !assert.NotEmptyf(t, client.pubsub, "Expected pubsub client to be initialized") {
		t.Errorf("NewClient().pubsub = %v, want not nil", client.pubsub)
	}

	// Create new flow
	flow, err := client.NewFlow(ctx)
	if err != nil {
		t.Errorf("NewFlow() error = %v", err)
		return
	}

	// Ensure flow is initialized
	if !assert.NotEmptyf(t, flow, "Expected flow to be initialized") {
		t.Errorf("NewFlow() = %v, want not nil", flow)
	}

	// Ensure flow steps are initialized and empty
	if !assert.NotEmptyf(t, flow.steps, "Expected flow steps to be initialized") {
		t.Errorf("NewFlow().steps = %v, want empty", flow.steps)
	}

	// Ensure data is initialized
	if !assert.NotEmptyf(t, flow.data, "Expected flow data to be initialized") {
		t.Errorf("NewFlow().data = %v, want not nil", flow.data)
	}

	// Ensure data has the name populated
	if !assert.NotEmptyf(t, flow.data.GetName(), "Expected flow.data.name to be populated") {
		t.Errorf("NewFlow().data.id = %v, want not nil", flow.data.GetName())
	}

	// If parent flow is provided, ensure data has the parent id populated
	var setParentId string
	if parentFlow != "" {
		setParentId = parentFlow
	}
	if !assert.Equal(t, setParentId, flow.data.GetParentId(), fmt.Sprintf("Expected(%s), Got(%s)", setParentId, flow.data.GetParentId())) {
		t.Errorf("NewFlow().data.parent_id = %v, want %v", flow.data.GetParentId(), setParentId)
	}

	// Create a new steps
	step1Id := "1.0"
	step1Title := "Step 1"
	step1, ctx, err := flow.NewStep(step1Id, step1Title)
	if err != nil {
		t.Errorf("NewStep() error = %v", err)
		return
	}

	// Ensure length of steps is 1
	if !assert.Len(t, flow.steps.Keys(), 1, "Expected flow steps to have a length of 1") {
		t.Errorf("NewFlow().steps = %v, want length of 1", flow.steps)
	}

	// Ensure step is initialized
	if !assert.NotEmptyf(t, step1, "Expected step to be initialized") {
		t.Errorf("NewStep() = %v, want not nil", step1)
	}

	// Ensure step id is set correctly
	if !assert.Equalf(t, step1Id, step1.data.GetId(), fmt.Sprintf("Expected(%s), Got(%s)", step1Id, step1.data.GetId())) {
		t.Errorf("NewStep().data.id = %v, want %v", step1.data.GetId(), step1Id)
	}

	// Ensure step title is set correctly
	if !assert.Equalf(t, step1Title, step1.data.GetTitle(), fmt.Sprintf("Expected(%s), Got(%s)", step1Title, step1.data.GetTitle())) {
		t.Errorf("NewStep().data.title = %v, want %v", step1.data.GetTitle(), step1Title)
	}

	// Change state to Queued
	step1 = step1.Queued()
	if !assert.Equalf(t, flows.Flow_Step_QUEUED, step1.data.GetState(), fmt.Sprintf("Expected(%s), Got(%s)", flows.Flow_Step_QUEUED, step1.data.GetState())) {
		t.Errorf("NewStep().data.state = %v, want %v", step1.data.GetState(), flows.Flow_Step_QUEUED)
	}

	// Change state to In Progress
	step1 = step1.InProgress()
	if !assert.Equalf(t, flows.Flow_Step_IN_PROGRESS, step1.data.GetState(), fmt.Sprintf("Expected(%s), Got(%s)", flows.Flow_Step_IN_PROGRESS, step1.data.GetState())) {
		t.Errorf("NewStep().data.state = %v, want %v", step1.data.GetState(), flows.Flow_Step_IN_PROGRESS)
	}

	// Change state to Cancelled
	step1 = step1.Cancelled()
	if !assert.Equalf(t, flows.Flow_Step_CANCELLED, step1.data.GetState(), fmt.Sprintf("Expected(%s), Got(%s)", flows.Flow_Step_CANCELLED, step1.data.GetState())) {
		t.Errorf("NewStep().data.state = %v, want %v", step1.data.GetState(), flows.Flow_Step_CANCELLED)
	}

	// Change state to Done
	step1 = step1.Done()
	if !assert.Equalf(t, flows.Flow_Step_COMPLETED, step1.data.GetState(), fmt.Sprintf("Expected(%s), Got(%s)", flows.Flow_Step_COMPLETED, step1.data.GetState())) {
		t.Errorf("NewStep().data.state = %v, want %v", step1.data.GetState(), flows.Flow_Step_COMPLETED)
	}

	// Change state to Failed
	step1 = step1.Failed(fmt.Errorf("error"))
	if !assert.Equalf(t, flows.Flow_Step_FAILED, step1.data.GetState(), fmt.Sprintf("Expected(%s), Got(%s)", flows.Flow_Step_FAILED, step1.data.GetState())) {
		t.Errorf("NewStep().data.state = %v, want %v", step1.data.GetState(), flows.Flow_Step_FAILED)
	}

	// Change state to Awaiting Input
	step1 = step1.AwaitingInput()
	if !assert.Equalf(t, flows.Flow_Step_AWAITING_INPUT, step1.data.GetState(), fmt.Sprintf("Expected(%s), Got(%s)", flows.Flow_Step_AWAITING_INPUT, step1.data.GetState())) {
		t.Errorf("NewStep().data.state = %v, want %v", step1.data.GetState(), flows.Flow_Step_AWAITING_INPUT)
	}

	step2Id := "2.0"
	step2Title := "Step 2"
	step2, ctx, err := flow.NewStep(step2Id, step2Title)
	if err != nil {
		t.Errorf("NewStep() error = %v", err)
		return
	}

	// Ensure length of steps is 2
	if !assert.Len(t, flow.steps.Keys(), 2, "Expected flow steps to have a length of 2") {
		t.Errorf("NewFlow().steps = %v, want length of 2", flow.steps)
	}

	// Ensure step is initialized
	if !assert.NotEmptyf(t, step2, "Expected step to be initialized") {
		t.Errorf("NewStep() = %v, want not nil", step2)
	}

	flow2, err := client.NewFlow(ctx)
	if err != nil {
		t.Errorf("NewFlow() error = %v", err)
		return
	}

	flow2Step1, ctx, err := flow2.NewStep("1.0", "Step 1")
	if err != nil {
		t.Errorf("NewStep() error = %v", err)
		return
	}

	flow2Step1 = flow2Step1.Done()
	if !assert.Equalf(t, flows.Flow_Step_COMPLETED, flow2Step1.data.GetState(), fmt.Sprintf("Expected(%s), Got(%s)", flows.Flow_Step_COMPLETED, flow2Step1.data.GetState())) {
		t.Errorf("NewStep().data.state = %v, want %v", flow2Step1.data.GetState(), flows.Flow_Step_COMPLETED)
	}

	// Test concurrent steps
	// Create a new flow
	concurrentFlow, err := client.NewFlow(ctx)
	if err != nil {
		t.Errorf("NewFlow() error = %v", err)
		return
	}

	eg, _ := errgroup.WithContext(ctx)
	eg.SetLimit(10)
	for i := range 100 {
		eg.Go(func() error {
			step, _, err := concurrentFlow.NewStep(fmt.Sprintf("%d", i), fmt.Sprintf("Step %d", i))
			if err != nil {
				return err
			}
			step.Done()

			if _, ok := step.f.steps.Get(step.data.GetId()); !ok {
				return fmt.Errorf("step not found")
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		t.Errorf("Error creating concurrent steps: %v", err)
		return
	}
	// Ensure length of steps is 100
	if !assert.Len(t, concurrentFlow.steps.Keys(), 100, "Expected flow steps to have a length of 100") {
		t.Errorf("NewFlow().steps = %v, want length of 100", concurrentFlow.steps)
	}
}
