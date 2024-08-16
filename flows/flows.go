package flows

import (
	"context"
	"fmt"
	"strings"

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
	DefaultTopic        = "flows"
	FlowParentHeaderKey = "x-alis-flow-parent"
)

// Client object to manage Publishing to a Pub/Sub topic.
type Client struct {
	pubsub       *pubsub.Client
	topic        string
	awaitPublish bool
}

// Options for the NewClient method.
type Options struct {
	// The Pub/Sub Topic
	// For example: 'flows'
	//
	// Defaults to 'progress' if not specified.
	Topic string
	// Indicates whether the pubsub client should block until the message is published.
	// If set to true, the client will block until the message is published or the context is done.
	// If set to false, the client will return immediately after the message is published.
	AwaitPublish bool
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

// WithAwaitPublish instructs the client to block until the flow is finished publishing.
// This causes the client to block until the Publish call completes or the context is done.
func WithAwaitPublish() Option {
	return func(opts *Options) {
		opts.AwaitPublish = true
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
		pubsub:       pubsubClient,
		topic:        topic,
		awaitPublish: options.AwaitPublish,
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
	uid, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	// Remove hyphens from the UUID
	id := strings.ReplaceAll(uid.String(), "-", "")

	// Potentially use interceptors to pass method and parent id
	source := "" // retrieve from grpc.Method
	// Retrieve the fully qualified method name
	if invokingMethod, ok := grpc.Method(ctx); ok {
		source = invokingMethod
	}
	var parentId string // retrieve from x-alis-flow-parent
	// We check if the context has a special header x-alis-flow-parent
	// and if it does then we use that as the parent id
	md, ok := metadata.FromIncomingContext(ctx)
	if ok && len(md.Get(FlowParentHeaderKey)) > 0 {
		parentIds := md.Get(FlowParentHeaderKey)
		// We found a special header x-alis-flow-parent, it suggests that the flow has a parent
		parentId = parentIds[len(parentIds)-1]
	}

	return &Flow{
		ctx: ctx,
		data: &flows.Flow{
			Name:       "flows/" + id,
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
		"type":   string((&flows.Flow{}).ProtoReflect().Descriptor().FullName()),
		"source": f.data.Source,
		"parent": f.data.ParentId,
	}

	topic := f.client.pubsub.Topic(f.client.topic)
	defer topic.Stop()
	result := topic.Publish(f.ctx, &pubsub.Message{
		Data:       data,
		Attributes: attributes,
		// OrderingKey: opts.OrderingKey, // TODO: Add ordering key
	})

	if f.client.awaitPublish {
		// Use the Get method to block until the Publish call completes or the context is done
		_, err := result.Get(f.ctx)
		if err != nil {
			return fmt.Errorf("failed to publish message: %w", err)
		}
	}
	return nil
}

// WithSource sets the source of the flow.
//
// This overrides the inferred source from the invoking method.
func (f *Flow) WithSource(source string) *Flow {
	f.data.UpdateTime = timestamppb.Now()
	f.data.Source = source
	return f
}

// WithParentId sets the parent id of the flow.
//
// This overrides the inferred parent id from the x-alis-flow-parent header.
//
// The parent id is of the format: <flow-id>-<step-id>.
// For example: 0af7651916cd43dd8448eb211c80319c-0
func (f *Flow) WithParentId(parentId string) (*Flow, error) {
	if !ParentIdRegex.MatchString(parentId) {
		return nil, fmt.Errorf("invalid parent id (%s). parent id must be in the format <flow-id>-<step-id>", parentId)
	}

	f.data.UpdateTime = timestamppb.Now()
	f.data.ParentId = parentId
	return f, nil
}

// NewStep adds a step to the flow and returns a Step object.
//
// The initial state of the step is Queued.
func (f *Flow) NewStep(id string, title string) (*Step, context.Context, error) {
	// Validate Id
	if !StepIdRegex.MatchString(id) {
		return nil, nil, fmt.Errorf("invalid step id (%s). step id must not contain hyphens", id)
	}

	step := &Step{
		f: f,
		data: &flows.Flow_Step{
			Id:         id,
			Title:      title,
			State:      flows.Flow_Step_QUEUED,
			CreateTime: timestamppb.Now(),
		},
	}
	f.steps.Set(id, step)

	// Get flow id from the flow name
	flowId := strings.TrimPrefix(f.data.Name, "flows/")

	parentId := fmt.Sprintf("%s-%s", flowId, id)

	// Add the parent id to the context
	if err := grpc.SetHeader(f.ctx, metadata.Pairs(FlowParentHeaderKey, parentId)); err != nil {
		return nil, nil, fmt.Errorf("failed to set parent id in context: %w", err)
	}
	// Create new context with the parent id set
	outgoingMd, ok := metadata.FromOutgoingContext(f.ctx)
	// If the metadata is not present, create a new one,
	// otherwise update the existing metadata with the parent id
	if !ok || outgoingMd == nil {
		outgoingMd = metadata.MD{FlowParentHeaderKey: []string{parentId}}
	} else {
		outgoingMd.Set(FlowParentHeaderKey, parentId)
	}
	outgoingCtx := metadata.NewIncomingContext(f.ctx, outgoingMd)

	return step, outgoingCtx, nil
}

// WithTitle sets the title of the step.
func (s *Step) WithTitle(title string) *Step {
	s.data.UpdateTime = timestamppb.Now()
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.Title = title
	return s
}

func (s *Step) WithDescription(description string) *Step {
	s.data.UpdateTime = timestamppb.Now()
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.Description = description
	return s
}

// Queued marks the state of the step as Queued.
func (s *Step) Queued() *Step {
	s.data.UpdateTime = timestamppb.Now()
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.State = flows.Flow_Step_QUEUED
	return s
}

// InProgress marks the state of the step as In Progress.
func (s *Step) InProgress() *Step {
	s.data.UpdateTime = timestamppb.Now()
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.State = flows.Flow_Step_IN_PROGRESS
	return s
}

// Cancelled marks the state of the step as Queued.
func (s *Step) Cancelled() *Step {
	s.data.UpdateTime = timestamppb.Now()
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.State = flows.Flow_Step_CANCELLED
	return s
}

// Done marks the state of the step as done.
func (s *Step) Done() *Step {
	s.data.UpdateTime = timestamppb.Now()
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.State = flows.Flow_Step_COMPLETED
	return s
}

// Failed marks the lasted step as Failed with the specified error.
func (s *Step) Failed(err error) *Step {
	s.data.UpdateTime = timestamppb.Now()
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.State = flows.Flow_Step_FAILED
	return s
}

// AwaitingInput marks the state of the step as Awaiting Input.
func (s *Step) AwaitingInput() *Step {
	s.data.UpdateTime = timestamppb.Now()
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.State = flows.Flow_Step_AWAITING_INPUT
	return s
}

// Publish allows one to publish a particular step.
func (s *Step) Publish() error {
	return s.f.Publish()
}
