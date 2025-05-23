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

// Code generated by cluster-client-gen. DO NOT EDIT.

package fake

import (
	kcpgentype "github.com/kcp-dev/client-go/third_party/k8s.io/client-go/gentype"
	kcptesting "github.com/kcp-dev/client-go/third_party/k8s.io/client-go/testing"
	"github.com/kcp-dev/logicalcluster/v3"

	apisv1alpha2 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha2"
	kcpv1alpha2 "github.com/kcp-dev/kcp/sdk/client/applyconfiguration/apis/v1alpha2"
	typedkcpapisv1alpha2 "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster/typed/apis/v1alpha2"
	typedapisv1alpha2 "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/typed/apis/v1alpha2"
)

// aPIBindingClusterClient implements APIBindingClusterInterface
type aPIBindingClusterClient struct {
	*kcpgentype.FakeClusterClientWithList[*apisv1alpha2.APIBinding, *apisv1alpha2.APIBindingList]
	Fake *kcptesting.Fake
}

func newFakeAPIBindingClusterClient(fake *ApisV1alpha2ClusterClient) typedkcpapisv1alpha2.APIBindingClusterInterface {
	return &aPIBindingClusterClient{
		kcpgentype.NewFakeClusterClientWithList[*apisv1alpha2.APIBinding, *apisv1alpha2.APIBindingList](
			fake.Fake,
			apisv1alpha2.SchemeGroupVersion.WithResource("apibindings"),
			apisv1alpha2.SchemeGroupVersion.WithKind("APIBinding"),
			func() *apisv1alpha2.APIBinding { return &apisv1alpha2.APIBinding{} },
			func() *apisv1alpha2.APIBindingList { return &apisv1alpha2.APIBindingList{} },
			func(dst, src *apisv1alpha2.APIBindingList) { dst.ListMeta = src.ListMeta },
			func(list *apisv1alpha2.APIBindingList) []*apisv1alpha2.APIBinding {
				return kcpgentype.ToPointerSlice(list.Items)
			},
			func(list *apisv1alpha2.APIBindingList, items []*apisv1alpha2.APIBinding) {
				list.Items = kcpgentype.FromPointerSlice(items)
			},
		),
		fake.Fake,
	}
}

func (c *aPIBindingClusterClient) Cluster(cluster logicalcluster.Path) typedapisv1alpha2.APIBindingInterface {
	return newFakeAPIBindingClient(c.Fake, cluster)
}

// aPIBindingScopedClient implements APIBindingInterface
type aPIBindingScopedClient struct {
	*kcpgentype.FakeClientWithListAndApply[*apisv1alpha2.APIBinding, *apisv1alpha2.APIBindingList, *kcpv1alpha2.APIBindingApplyConfiguration]
	Fake        *kcptesting.Fake
	ClusterPath logicalcluster.Path
}

func newFakeAPIBindingClient(fake *kcptesting.Fake, clusterPath logicalcluster.Path) typedapisv1alpha2.APIBindingInterface {
	return &aPIBindingScopedClient{
		kcpgentype.NewFakeClientWithListAndApply[*apisv1alpha2.APIBinding, *apisv1alpha2.APIBindingList, *kcpv1alpha2.APIBindingApplyConfiguration](
			fake,
			clusterPath,
			"",
			apisv1alpha2.SchemeGroupVersion.WithResource("apibindings"),
			apisv1alpha2.SchemeGroupVersion.WithKind("APIBinding"),
			func() *apisv1alpha2.APIBinding { return &apisv1alpha2.APIBinding{} },
			func() *apisv1alpha2.APIBindingList { return &apisv1alpha2.APIBindingList{} },
			func(dst, src *apisv1alpha2.APIBindingList) { dst.ListMeta = src.ListMeta },
			func(list *apisv1alpha2.APIBindingList) []*apisv1alpha2.APIBinding {
				return kcpgentype.ToPointerSlice(list.Items)
			},
			func(list *apisv1alpha2.APIBindingList, items []*apisv1alpha2.APIBinding) {
				list.Items = kcpgentype.FromPointerSlice(items)
			},
		),
		fake,
		clusterPath,
	}
}
