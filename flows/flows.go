package flows

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"go.alis.build/utils/maps"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	pbFlows "open.alis.services/protobuf/alis/open/flows/v1"
)

type FlowOptions struct {
	existingFlow *pbFlows.Flow
}

// FlowOption is a functional option for the NewFlow method.
type FlowOption func(*FlowOptions)

// WithExistingFlow is used when one would like to create a new
// Flow object from an existing Flow data object.  This would
// typically be used with resumable Long-running Operations where the
// Flow data object is stored withing the state of the particular LRO
func WithExistingFlow(existingFlow *pbFlows.Flow) FlowOption {
	return func(opts *FlowOptions) {
		opts.existingFlow = existingFlow
	}
}

type Flow struct {
	ctx    context.Context
	client *Client
	data   *pbFlows.Flow
	steps  *maps.OrderedMap[string, *Step]
}

// NewFlow creates a new Flow object
//
// The source is inferred from the invoking method.
// This can be overridden by calling WithSource.
//
// The parent id is inferred from the x-alis-flow-parent header.
// This can be overridden by calling WithParentId.
func (c *Client) NewFlow(ctx context.Context, opts ...FlowOption) (*Flow, error) {
	// We'll create a new flowId
	uid, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	id := strings.ReplaceAll(uid.String(), "-", "") // Remove hyphens from the UUID

	options := &FlowOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// This is a new flow
	if options.existingFlow == nil {
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
		if ok && len(md.Get(FlowHeaderKey)) > 0 {
			flowIds := md.Get(FlowHeaderKey)
			// Hmmm... why did we find a x-alis-flow-id value in the header, are we not
			// supposed to create a new Flow?
			return nil, fmt.Errorf("an exising flow was already found in the current context (x-alis-flow-id:%s)", flowIds[len(flowIds)-1])
		}

		// Add the parent id to the context
		if err := grpc.SetHeader(ctx, metadata.Pairs(FlowHeaderKey, id)); err != nil {
			return nil, fmt.Errorf("failed to set flow id (%s) in context: %w", parentId, err)
		}

		return &Flow{
			ctx: ctx,
			data: &pbFlows.Flow{
				Name:       "flows/" + id,
				Source:     source,
				ParentId:   parentId,
				Steps:      []*pbFlows.Flow_Step{},
				CreateTime: timestamppb.Now(),
			},
			client: c,
			steps:  maps.NewOrderedMap[string, *Step](),
		}, nil
	} else {
		// Instantiate a new Flow object from the Flow proto data.
		flow := &Flow{
			ctx:    ctx,
			data:   options.existingFlow,
			client: c,
		}

		// Iterate through the steps and prepare the steps attribute
		var steps *maps.OrderedMap[string, *Step]
		for _, s := range options.existingFlow.GetSteps() {
			// TODO: how to
			step, _, err := flow.NewStep(s.GetId(), WithDescription(s.GetDescription()), WithTitle(s.GetTitle()))
			if err != nil {
				return nil, err
			}
			steps.Set(s.GetId(), step)
		}

		// Use the provided Flow data object to instantiate a new Flow object
		return flow, nil
	}
}

// Publish the Flow as an event.
func (f *Flow) Publish() error {
	// Using the data object add all the steps
	steps := make([]*pbFlows.Flow_Step, f.steps.Len())
	f.steps.Range(func(idx int, key string, value *Step) bool {
		steps[idx] = value.data
		return true
	})
	f.data.Steps = steps
	f.data.PublishTime = timestamppb.Now()

	// Convert the event message to a []byte format, as required by Pub/Sub's data attribute
	data, err := proto.Marshal(f.data)
	if err != nil {
		return fmt.Errorf("marshal the message to bytes: %w", err)
	}

	// Set the Type of event
	attributes := map[string]string{
		"type":   string((&pbFlows.Flow{}).ProtoReflect().Descriptor().FullName()),
		"source": f.data.Source,
		"parent": f.data.ParentId,
	}

	topic := f.client.pubsub.Topic(f.client.topic)
	topic.EnableMessageOrdering = true

	defer topic.Stop()
	result := topic.Publish(f.ctx, &pubsub.Message{
		Data:        data,
		Attributes:  attributes,
		OrderingKey: f.data.Name, // This ensures that messages published at the step level are delivered in order
	})

	// If the awaitPublish option is set, block until the message is published
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
// The initial state of the step is Queued. This can be overridden by passing the WithState option.
//
// If the WithExistingId option is provided, the step with the specified id is returned.
// If the step does not exist, a new step is created with the specified id.
func (f *Flow) NewStep(id string, opts ...StepOption) (*Step, context.Context, error) {
	// Validate Id
	if !StepIdRegex.MatchString(id) {
		return nil, nil, fmt.Errorf("invalid step id (%s). step id must not contain hyphens", id)
	}

	options := &StepOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Create a new step
	var step *Step
	// If the step already exists, get the step
	if options.existingId {
		if existingStep, ok := f.steps.Get(id); ok {
			step = existingStep
		}
	}
	// Create a new step if it does not exist
	if step == nil {
		// Get initial state
		state := pbFlows.Flow_Step_QUEUED
		if options.state != pbFlows.Flow_Step_STATE_UNSPECIFIED {
			state = options.state
		}

		step = &Step{
			f: f,
			data: &pbFlows.Flow_Step{
				Id:          id,
				Title:       options.title,
				Description: options.description,
				State:       state,
				CreateTime:  timestamppb.Now(),
			},
		}
		f.steps.Set(id, step)
	}

	// Get flow id from the flow name
	flowId := strings.TrimPrefix(f.data.Name, "flows/")
	parentId := fmt.Sprintf("%s-%s", flowId, id)

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

/*
Steps returns the steps of the flow.
The steps are returned in an [OrderedMap](https://pkg.go.dev/go.alis.build/utils#OrderedMap)

Example Usage:

	stepsMap := make(map[string]*Step, flow.Steps().Len())
	flow.Steps().Range(func(idx int, key string, value *Step) bool {
		stepsMap[key] = value
		return true
	})
	stepsMap["step1"].Done()
*/
func (f *Flow) Steps() *maps.OrderedMap[string, *Step] {
	return f.steps
}
