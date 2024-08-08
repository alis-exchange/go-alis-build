package flows

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	flows "open.alis.services/protobuf/alis/open/flows/v1"
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

// New creates a new instance of the Client object.
func NewClient(project string, opts *Options) (*Client, error) {
	// Validate arguments
	if project == "" {
		return nil, fmt.Errorf("project is required but not provided")
	}

	// Default topic is 'flows'
	topic := "flows"
	if opts != nil {
		if opts.Topic != "" {
			topic = opts.Topic
		}
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
	steps  []*Step
}

// Step represents a single step within the Flow object.
type Step struct {
	f    *Flow
	data *flows.Flow_Step
}

// Flow creats a new Flow object
func (c *Client) NewFlow(ctx context.Context) (*Flow, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	// Potentially use interceptors to pass method and parent id
	source := "some header value" // potentially from interceptor method info?
	parentId := "some parent id"  // retrieve from x-alis-flows-id

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
	}, nil
}

// Publish the Flow as an event.
func (f *Flow) Publish() error {
	// Convert the event message to a []byte format, as required by Pub/Sub's data attribute

	// Using the data object add all the steps
	steps := make([]*flows.Flow_Step, len(f.steps))
	for i, step := range f.steps {
		steps[i] = step.data
	}
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

// Step adds a step to the flow and returns a Step object.
func (f *Flow) NewStep(id string, displayName string) *Step {
	step := &Step{
		f: f,
		data: &flows.Flow_Step{
			Id:          id,
			DisplayName: displayName,
			State:       flows.Flow_Step_IN_PROGRESS,
		},
	}
	f.steps = append(f.steps, step)
	return step
}

// Done marks the state of the step as done.
func (s *Step) WithDisplayName(displayName string) *Step {
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.DisplayName = displayName
	return s
}

// Done marks the state of the step as done.
func (s *Step) Done() *Step {
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.State = flows.Flow_Step_COMPLETED
	return s
}

// AwaitingInput marks the state of the step as Awaiting Input.
func (s *Step) AwaitingInput() *Step {
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.State = flows.Flow_Step_AWAITING_INPUT
	return s
}

// Queued marks the state of the step as Queued.
func (s *Step) Queued() *Step {
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.State = flows.Flow_Step_QUEUED
	return s
}

// Cancelled marks the state of the step as Queued.
func (s *Step) Cancelled() *Step {
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.State = flows.Flow_Step_CANCELLED
	return s
}

// Failed marks the lasted step as Failed with the specified error.
func (s *Step) Failed(err error) *Step {
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.State = flows.Flow_Step_FAILED
	return s
}
