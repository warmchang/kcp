/*
Copyright The KCP Authors.

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

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1alpha2

// PermissionClaimApplyConfiguration represents a declarative configuration of the PermissionClaim type for use
// with apply.
type PermissionClaimApplyConfiguration struct {
	GroupResourceApplyConfiguration `json:",inline"`
	Verbs                           []string `json:"verbs,omitempty"`
	IdentityHash                    *string  `json:"identityHash,omitempty"`
}

// PermissionClaimApplyConfiguration constructs a declarative configuration of the PermissionClaim type for use with
// apply.
func PermissionClaim() *PermissionClaimApplyConfiguration {
	return &PermissionClaimApplyConfiguration{}
}

// WithGroup sets the Group field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Group field is set to the value of the last call.
func (b *PermissionClaimApplyConfiguration) WithGroup(value string) *PermissionClaimApplyConfiguration {
	b.GroupResourceApplyConfiguration.Group = &value
	return b
}

// WithResource sets the Resource field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Resource field is set to the value of the last call.
func (b *PermissionClaimApplyConfiguration) WithResource(value string) *PermissionClaimApplyConfiguration {
	b.GroupResourceApplyConfiguration.Resource = &value
	return b
}

// WithVerbs adds the given value to the Verbs field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Verbs field.
func (b *PermissionClaimApplyConfiguration) WithVerbs(values ...string) *PermissionClaimApplyConfiguration {
	for i := range values {
		b.Verbs = append(b.Verbs, values[i])
	}
	return b
}

// WithIdentityHash sets the IdentityHash field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the IdentityHash field is set to the value of the last call.
func (b *PermissionClaimApplyConfiguration) WithIdentityHash(value string) *PermissionClaimApplyConfiguration {
	b.IdentityHash = &value
	return b
}
