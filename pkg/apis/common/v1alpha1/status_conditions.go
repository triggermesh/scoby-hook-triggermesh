/*
Copyright 2022 TriggerMesh Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// Status conditions
const (
	// ConditionReady has status True when the component is ready to receive/send events.
	ConditionReady = apis.ConditionReady
	// ConditionSinkProvided has status True when the component has been configured with an event sink.
	ConditionSinkProvided apis.ConditionType = "SinkProvided"
	// ConditionDeployed has status True when the component's adapter is up and running.
	ConditionDeployed apis.ConditionType = "Deployed"
)

// Reasons for status conditions
const (
	// ReasonRBACNotBound is set on a Deployed condition when an adapter's
	// ServiceAccount cannot be bound.
	ReasonRBACNotBound = "RBACNotBound"
	// ReasonUnavailable is set on a Deployed condition when an adapter in unavailable.
	ReasonUnavailable = "AdapterUnavailable"

	// ReasonSinkNotFound is set on a SinkProvided condition when a sink does not exist.
	ReasonSinkNotFound = "SinkNotFound"
	// ReasonSinkEmpty is set on a SinkProvided condition when a sink URI is empty.
	ReasonSinkEmpty = "EmptySinkURI"
)

// DefaultConditionSet is a generic set of status conditions used by default in
// all components.
var DefaultConditionSet = NewConditionSet()

// NewConditionSet returns a set of status conditions for a component type.
// Default conditions can be augmented by passing condition types as function arguments.
func NewConditionSet(cts ...apis.ConditionType) apis.ConditionSet {
	return apis.NewLivingConditionSet(
		append(defaultConditionTypes, cts...)...,
	)
}

// defaultConditionTypes is a list of condition types common to all components.
var defaultConditionTypes = []apis.ConditionType{
	ConditionDeployed,
}

// EventSenderConditionSet is a set of conditions for instances that send
// events to a sink.
var EventSenderConditionSet = NewConditionSet(
	ConditionSinkProvided,
)

// Status defines the observed state of a component instance.
//
// +k8s:deepcopy-gen=true
type Status struct {
	duckv1.SourceStatus  `json:",inline"`
	duckv1.AddressStatus `json:",inline"`

	// Accepted CloudEvent attributes
	CloudEventStatus `json:",inline"`
}

// CloudEventStatus contains attributes that event receivers can embed to
// declare the event types they accept.
//
// +k8s:deepcopy-gen=true
type CloudEventStatus struct {
	// AcceptedEventTypes are the CloudEvent types that a component can process.
	// +optional
	AcceptedEventTypes []string `json:"acceptedEventTypes,omitempty"`
}

// StatusManager manages the status of a TriggerMesh component.
type StatusManager struct {
	apis.ConditionSet
	*Status
}
