package validator

import (
	"context"
	"time"

	"go.alis.build/alog"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/durationpb"
)

type dur struct {
	paths       []string
	getValues   func(v *Validator, msg protoreflect.ProtoMessage) []time.Duration
	description string
	v           *Validator
}

func (d *dur) fieldPaths() []string {
	return d.paths
}

func (d *dur) getDescription() string {
	return d.description
}

func (d *dur) getValidator() *Validator {
	return d.v
}

func (d *dur) setValidator(v *Validator) {
	d.getValues(v, v.protoMsg)
	d.v = v
}

// Fixed duration value
func Duration(value time.Duration) *dur {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []time.Duration {
		return []time.Duration{value}
	}
	return &dur{description: value.String(), getValues: getValuesFunc}
}

// google.protobuf.Duration field
func DurationField(path string) *dur {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []time.Duration {
		d, ok := v.getSubMessage(msg, path).(*durationpb.Duration)
		if !ok {
			alog.Fatalf(context.Background(), "%s is not a Duration", path)
		}
		if d == nil {
			return []time.Duration{time.Duration(0)}
		}
		return []time.Duration{d.AsDuration()}
	}
	return &dur{paths: []string{path}, description: path, getValues: getValuesFunc}
}

func (d *dur) Populated() *Rule {
	id := "d-populated(" + d.getDescription() + ")"
	descr := &Descriptions{
		rule:         d.getDescription() + " must be populated",
		notRule:      d.getDescription() + " must not be populated",
		condition:    d.getDescription() + " is populated",
		notCondition: d.getDescription() + " is not populated",
	}
	args := []argI{d}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		dur1 := d.getValues(d.getValidator(), msg)[0]
		return dur1.Nanoseconds() == 0, nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}
