package validator

import (
	"context"
	"time"

	"go.alis.build/alog"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type datetime struct {
	paths       []string
	getValues   func(v *Validator, msg protoreflect.ProtoMessage) []time.Time
	description string
	v           *Validator
}

func (t *datetime) fieldPaths() []string {
	return t.paths
}

func (t *datetime) getDescription() string {
	return t.description
}

func (t *datetime) getValidator() *Validator {
	return t.v
}

func (t *datetime) setValidator(v *Validator) {
	t.getValues(v, v.protoMsg)
	t.v = v
}

// Fixed time value
func Time(value time.Time) *datetime {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []time.Time {
		return []time.Time{value}
	}
	return &datetime{description: value.String(), getValues: getValuesFunc}
}

// time.Now() evaluated at the time of validation
func Now() *datetime {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []time.Time {
		return []time.Time{time.Now()}
	}
	return &datetime{description: "current time", getValues: getValuesFunc}
}

// google.protobuf.Timestamp field
func TimestampField(path string) *datetime {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []time.Time {
		ts, ok := v.getSubMessage(msg, path).(*timestamppb.Timestamp)
		if !ok {
			alog.Fatalf(context.Background(), "%s is not a Timestamp", path)
		}
		if ts == nil {
			return []time.Time{}
		}
		return []time.Time{ts.AsTime()}
	}
	return &datetime{description: path, getValues: getValuesFunc, paths: []string{path}}
}

// google.type.Date field
func DateField(path string) *datetime {
	getValuesFunc := func(v *Validator, msg protoreflect.ProtoMessage) []time.Time {
		date, ok := v.getSubMessage(msg, path).(*date.Date)
		if !ok {
			alog.Fatalf(context.Background(), "%s is not a Date", path)
		}
		if date == nil {
			return []time.Time{{}}
		}
		year := int64(date.GetYear())
		month := time.Month(date.GetMonth())
		day := date.GetDay()
		t := time.Date(int(year), month, int(day), 0, 0, 0, 0, time.UTC)
		return []time.Time{t}
	}
	return &datetime{description: path, getValues: getValuesFunc, paths: []string{path}}
}

// Rule that ensures that timestamp or datefield is populated
func (t *datetime) Populated() *Rule {
	id := "t-populated(" + t.getDescription() + ")"
	descr := &Descriptions{
		rule:         t.getDescription() + " must be populated",
		notRule:      t.getDescription() + " must not be populated",
		condition:    t.getDescription() + " is populated",
		notCondition: t.getDescription() + " is not populated",
	}
	args := []argI{t}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		time1 := t.getValues(t.getValidator(), msg)[0]
		return time1.IsZero(), nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

// Rule that ensures that t is equal to t2
func (t *datetime) Equals(t2 *datetime) *Rule {
	id := "t-eq(" + t.getDescription() + "," + t2.getDescription() + ")"
	descr := &Descriptions{
		rule:         t.getDescription() + " must be equal to " + t2.getDescription(),
		notRule:      t.getDescription() + " must not be equal to " + t2.getDescription(),
		condition:    t.getDescription() + " equals " + t2.getDescription(),
		notCondition: t.getDescription() + " does not equal " + t2.getDescription(),
	}
	args := []argI{t, t2}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		return t.getValues(t.getValidator(), msg)[0].Equal(t2.getValues(t2.getValidator(), msg)[0]), nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

// Rule that ensures that t is before t2
func (t *datetime) Before(t2 *datetime) *Rule {
	id := "t-before(" + t.getDescription() + "," + t2.getDescription() + ")"
	descr := &Descriptions{
		rule:         t.getDescription() + " must be before " + t2.getDescription(),
		notRule:      t.getDescription() + " must not be before " + t2.getDescription(),
		condition:    t.getDescription() + " is before " + t2.getDescription(),
		notCondition: t.getDescription() + " is not before " + t2.getDescription(),
	}
	args := []argI{t, t2}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		time1 := t.getValues(t.getValidator(), msg)[0]
		time2 := t2.getValues(t2.getValidator(), msg)[0]
		return !time1.Before(time2), nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}

// Rule that ensures that t is after t2
func (t *datetime) After(t2 *datetime) *Rule {
	id := "t-after(" + t.getDescription() + "," + t2.getDescription() + ")"
	descr := &Descriptions{
		rule:         t.getDescription() + " must be after " + t2.getDescription(),
		notRule:      t.getDescription() + " must not be after " + t2.getDescription(),
		condition:    t.getDescription() + " is after " + t2.getDescription(),
		notCondition: t.getDescription() + " is not after " + t2.getDescription(),
	}
	args := []argI{t, t2}
	isViolatedFunc := func(msg protoreflect.ProtoMessage) (bool, error) {
		time1 := t.getValues(t.getValidator(), msg)[0]
		time2 := t2.getValues(t2.getValidator(), msg)[0]
		return !time1.After(time2), nil
	}
	return newPrimitiveRule(id, descr, args, isViolatedFunc)
}
