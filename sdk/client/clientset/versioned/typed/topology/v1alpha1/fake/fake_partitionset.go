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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	gentype "k8s.io/client-go/gentype"

	v1alpha1 "github.com/kcp-dev/kcp/sdk/apis/topology/v1alpha1"
	topologyv1alpha1 "github.com/kcp-dev/kcp/sdk/client/applyconfiguration/topology/v1alpha1"
	typedtopologyv1alpha1 "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/typed/topology/v1alpha1"
)

// fakePartitionSets implements PartitionSetInterface
type fakePartitionSets struct {
	*gentype.FakeClientWithListAndApply[*v1alpha1.PartitionSet, *v1alpha1.PartitionSetList, *topologyv1alpha1.PartitionSetApplyConfiguration]
	Fake *FakeTopologyV1alpha1
}

func newFakePartitionSets(fake *FakeTopologyV1alpha1) typedtopologyv1alpha1.PartitionSetInterface {
	return &fakePartitionSets{
		gentype.NewFakeClientWithListAndApply[*v1alpha1.PartitionSet, *v1alpha1.PartitionSetList, *topologyv1alpha1.PartitionSetApplyConfiguration](
			fake.Fake,
			"",
			v1alpha1.SchemeGroupVersion.WithResource("partitionsets"),
			v1alpha1.SchemeGroupVersion.WithKind("PartitionSet"),
			func() *v1alpha1.PartitionSet { return &v1alpha1.PartitionSet{} },
			func() *v1alpha1.PartitionSetList { return &v1alpha1.PartitionSetList{} },
			func(dst, src *v1alpha1.PartitionSetList) { dst.ListMeta = src.ListMeta },
			func(list *v1alpha1.PartitionSetList) []*v1alpha1.PartitionSet {
				return gentype.ToPointerSlice(list.Items)
			},
			func(list *v1alpha1.PartitionSetList, items []*v1alpha1.PartitionSet) {
				list.Items = gentype.FromPointerSlice(items)
			},
		),
		fake,
	}
}
