/*
Copyright 2021.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SecretValueFromSource represents the source of a secret value
type SecretValueFromSource struct {
	// The Secret key to select from.
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// GitProvider providers name of git provider
type GitProvider string

type gitEvent string

var (
	// Gitlab gitlab.com compatible
	Gitlab GitProvider = "gitlab"

	// Github github.com compatible
	Github GitProvider = "github"

	// Gogs gogs compatible
	Gogs GitProvider = "gogs"
)

// GitHookSpec defines the desired state of GitHook
type GitHookSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	ProjectURL string `json:"projectUrl"`

	GitProvider GitProvider `json:"gitProvider"`

	AccessToken SecretValueFromSource `json:"accessToken"`

	SecretToken SecretValueFromSource `json:"secretToken"`

	EventTypes []gitEvent `json:"eventTypes"`
}

// GitHookStatus defines the observed state of GitHook
type GitHookStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	ID string `json:"Id,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GitHook is the Schema for the githooks API
type GitHook struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitHookSpec   `json:"spec,omitempty"`
	Status GitHookStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GitHookList contains a list of GitHook
type GitHookList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitHook `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitHook{}, &GitHookList{})
}
