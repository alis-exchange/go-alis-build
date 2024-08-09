package flows

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	alUtils "go.alis.build/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	flows "open.alis.services/protobuf/alis/open/flows/v1"
)

const (
	DefaultTopic    = "flows"
	FlowIdHeaderKey = "x-alis-flows-id"
)

// Client object to manage Publishing to a Pub/Sub topic.
type Client struct {
	pubsub *pubsub.Client
	topic  string
}

// Options for the NewClient method.
type Options struct {
	// The Pub/Sub Topic
	// For example: 'flows'
	//
	// Defaults to 'progress' if not specified.
	Topic string
}

// Option is a functional option for the NewClient method.
type Option func(*Options)

// WithTopic sets the topic for the client.
//
// If provided multiple times, the last value will take precedence.
func WithTopic(topic string) Option {
	return func(opts *Options) {
		opts.Topic = topic
	}
}

// NewClient creates a new instance of the Client object.
//
// Multiple Option functions can be provided to customize the client.
// For example: WithTopic("flows")
func NewClient(project string, opts ...Option) (*Client, error) {
	// Validate arguments
	if project == "" {
		return nil, fmt.Errorf("project is required but not provided")
	}

	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	// Default topic is 'flows'
	topic := DefaultTopic
	if options.Topic != "" {
		topic = options.Topic
	}

	pubsubClient, err := pubsub.NewClient(context.Background(), project)
	if err != nil {
		return nil, err
	}
	return &Client{
		pubsub: pubsubClient,
		topic:  topic,
	}, nil
}

type Flow struct {
	ctx    context.Context
	client *Client
	data   *flows.Flow
	steps  *alUtils.OrderedMap[string, *Step]
}

// Step represents a single step within the Flow object.
type Step struct {
	f    *Flow
	data *flows.Flow_Step
}

// NewFlow creates a new Flow object
//
// The source is inferred from the invoking method.
// This can be overridden by calling WithSource.
//
// The parent id is inferred from the x-alis-flows-id header.
// This can be overridden by calling WithParentId.
func (c *Client) NewFlow(ctx context.Context) (*Flow, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	// Potentially use interceptors to pass method and parent id
	source := "" // retrieve from grpc.Method
	// Retrieve the fully qualified method name
	if invokingMethod, ok := grpc.Method(ctx); ok {
		source = invokingMethod
	}
	var parentId string // retrieve from x-alis-flows-id
	// We check if the context has a special header x-alis-flows-id
	// and if it does then we use that as the parent id
	if md, ok := metadata.FromIncomingContext(ctx); ok && len(md.Get(FlowIdHeaderKey)) > 0 {
		// We found a special header x-alis-flows-id, it suggests that the flow has a parent
		parentId = md.Get(FlowIdHeaderKey)[0]
	}

	return &Flow{
		ctx: ctx,
		data: &flows.Flow{
			Id:         id.String(),
			Source:     source,
			ParentId:   parentId,
			Steps:      []*flows.Flow_Step{},
			CreateTime: timestamppb.Now(),
		},
		client: c,
		steps:  alUtils.NewOrderedMap[string, *Step](),
	}, nil
}

// Publish the Flow as an event.
func (f *Flow) Publish() error {
	// Convert the event message to a []byte format, as required by Pub/Sub's data attribute

	// Using the data object add all the steps
	steps := make([]*flows.Flow_Step, f.steps.Len())
	f.steps.Range(func(idx int, key string, value *Step) bool {
		steps[idx] = value.data
		return true
	})
	f.data.Steps = steps
	f.data.PublishTime = timestamppb.Now()

	data, err := proto.Marshal(f.data)
	if err != nil {
		return fmt.Errorf("marshal the message to bytes: %w", err)
	}

	// Set the Type of event
	attributes := map[string]string{
		"type":   "alis.open.flows.v1.Flow",
		"source": f.data.Source,
		"parent": f.data.ParentId,
	}

	topic := f.client.pubsub.Topic(f.client.topic)
	defer topic.Stop()
	topic.Publish(f.ctx, &pubsub.Message{
		Data:       data,
		Attributes: attributes,
		// OrderingKey: opts.OrderingKey,
	})

	// Use the Get method to block until the Publish call completes or the context is done
	// _, err = result.Get(ctx)
	return nil
}

// WithSource sets the source of the flow.
//
// This overrides the inferred source from the invoking method.
func (f *Flow) WithSource(source string) *Flow {
	f.data.Source = source
	return f
}

// WithParentId sets the parent id of the flow.
//
// This overrides the inferred parent id from the x-alis-flows-id header.
func (f *Flow) WithParentId(parentId string) *Flow {
	f.data.ParentId = parentId
	return f
}

// NewStep adds a step to the flow and returns a Step object.
//
// The initial state of the step is Queued.
func (f *Flow) NewStep(id string, displayName string) *Step {
	step := &Step{
		f: f,
		data: &flows.Flow_Step{
			Id:          id,
			DisplayName: displayName,
			State:       flows.Flow_Step_QUEUED,
		},
	}
	f.steps.Set(id, step)
	return step
}

// WithDisplayName sets the display name of the step.
func (s *Step) WithDisplayName(displayName string) *Step {
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.DisplayName = displayName
	return s
}

// Queued marks the state of the step as Queued.
func (s *Step) Queued() *Step {
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.State = flows.Flow_Step_QUEUED
	return s
}

// InProgress marks the state of the step as In Progress.
func (s *Step) InProgress() *Step {
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.State = flows.Flow_Step_IN_PROGRESS
	return s
}

// Cancelled marks the state of the step as Queued.
func (s *Step) Cancelled() *Step {
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.State = flows.Flow_Step_CANCELLED
	return s
}

// Done marks the state of the step as done.
func (s *Step) Done() *Step {
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.State = flows.Flow_Step_COMPLETED
	return s
}

// Failed marks the lasted step as Failed with the specified error.
func (s *Step) Failed(err error) *Step {
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.State = flows.Flow_Step_FAILED
	return s
}

// AwaitingInput marks the state of the step as Awaiting Input.
func (s *Step) AwaitingInput() *Step {
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.State = flows.Flow_Step_AWAITING_INPUT
	return s
}

// Publish allows one to publish a particular step.
func (s *Step) Publish() error {
	return s.f.Publish()
}
