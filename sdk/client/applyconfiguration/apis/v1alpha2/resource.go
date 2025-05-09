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

// ResourceApplyConfiguration represents a declarative configuration of the Resource type for use
// with apply.
type ResourceApplyConfiguration struct {
	Name    *string                                  `json:"name,omitempty"`
	Group   *string                                  `json:"group,omitempty"`
	Schema  *string                                  `json:"schema,omitempty"`
	Storage *ResourceSchemaStorageApplyConfiguration `json:"storage,omitempty"`
}

// ResourceApplyConfiguration constructs a declarative configuration of the Resource type for use with
// apply.
func Resource() *ResourceApplyConfiguration {
	return &ResourceApplyConfiguration{}
}

// WithName sets the Name field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Name field is set to the value of the last call.
func (b *ResourceApplyConfiguration) WithName(value string) *ResourceApplyConfiguration {
	b.Name = &value
	return b
}

// WithGroup sets the Group field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Group field is set to the value of the last call.
func (b *ResourceApplyConfiguration) WithGroup(value string) *ResourceApplyConfiguration {
	b.Group = &value
	return b
}

// WithSchema sets the Schema field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Schema field is set to the value of the last call.
func (b *ResourceApplyConfiguration) WithSchema(value string) *ResourceApplyConfiguration {
	b.Schema = &value
	return b
}

// WithStorage sets the Storage field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Storage field is set to the value of the last call.
func (b *ResourceApplyConfiguration) WithStorage(value *ResourceSchemaStorageApplyConfiguration) *ResourceApplyConfiguration {
	b.Storage = value
	return b
}
