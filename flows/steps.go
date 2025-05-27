package flows

import (
	"google.golang.org/protobuf/types/known/timestamppb"
	flows "open.alis.services/protobuf/alis/open/flows/v1"
)

// Step represents a single step within the Flow object.
type Step struct {
	f    *Flow
	data *flows.Flow_Step
}

// StepOptions for the NewStep method.
type StepOptions struct {
	existingId  bool
	title       string
	description string
	state       flows.Flow_Step_State
}

// StepOption is a functional option for the NewStep method.
type StepOption func(*StepOptions)

// WithTitle sets the title of the step.
func WithTitle(title string) StepOption {
	return func(opts *StepOptions) {
		opts.title = title
	}
}

// WithDescription sets the description of the step.
func WithDescription(description string) StepOption {
	return func(opts *StepOptions) {
		opts.description = description
	}
}

// WithExistingId gets the step with the specified id.
// If the step does not exist, it assumes normal behaviour and creates a new step with the specified id.
func WithExistingId() StepOption {
	return func(opts *StepOptions) {
		opts.existingId = true
	}
}

// WithState sets the initial state of the step.
func WithState(state flows.Flow_Step_State) StepOption {
	return func(opts *StepOptions) {
		opts.state = state
	}
}

// WithTitle sets the title of the step.
func (s *Step) WithTitle(title string) *Step {
	s.data.UpdateTime = timestamppb.Now()
	s.f.data.UpdateTime = timestamppb.Now()
	s.data.Title = title
	return s
}

// WithDescription sets the description of the step.
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
