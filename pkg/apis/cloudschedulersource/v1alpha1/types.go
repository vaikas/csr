/*
Copyright 2017 The Kubernetes Authors.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudSchedulerSource is a specification for a CloudSchedulerSource resource
type CloudSchedulerSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CloudSchedulerSourceSpec   `json:"spec"`
	Status CloudSchedulerSourceStatus `json:"status"`
}

// CloudSchedulerSourceSpec is the spec for a CloudSchedulerSource resource
type CloudSchedulerSourceSpec struct {
	// ServiceAccountName holds the name of the Kubernetes service account
	// as which the underlying K8s resources should be run. If unspecified
	// this will default to the "default" service account for the namespace
	// in which the GitHubSource exists.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// GoogleCloudProject is the ID of the Google Cloud Project that the PubSub Topic exists in.
	GoogleCloudProject string `json:"googleCloudProject,omitempty"`

	// Location where to create the Job in.
	Location string `json:"location"`

	// Schedule in cron format, for example: "* * * * *" would be run
	// every minute.
	Schedule string `json:"schedule"`
	// Timezone to apply to the schedule. If omitted, uses UTC
	TimeZone string `json:"timezone,omitempty"`

	// Which method to use to call. GET,PUT or POST. If omitted uses POST
	// +optional
	HTTPMethod string `json:"httpMethod,omitempty"`
	// What data to send in the call body (PUT/POST).
	// +optional
	Body string `json:"body,omitempty"`

	// TODO: Add other configuration options here...

	// Sink is a reference to an object that will resolve to a domain name to use
	// as the sink.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`
}

// CloudSchedulerSourceStatus is the status for a CloudSchedulerSource resource
type CloudSchedulerSourceStatus struct {
	// TODO: add conditions and other stuff here...
	// Job is the URI for the created Cloud Scheduler Job
	Job string `json:"job"`

	// SinkURI is the current active sink URI that has been configured
	// for the CloudSchedulerSource
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudSchedulerSourceList is a list of CloudSchedulerSource resources
type CloudSchedulerSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CloudSchedulerSource `json:"items"`
}
