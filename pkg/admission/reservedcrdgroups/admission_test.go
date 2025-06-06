/*
Copyright 2022 The KCP Authors.

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

package reservedcrdgroups

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/admission/helpers"
	apisv1alpha2 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha2"
)

func createAttr(obj *apiextensions.CustomResourceDefinition) admission.Attributes {
	return admission.NewAttributesRecord(
		obj,
		nil,
		apiextensionsv1.Kind("CustomResourceDefinition").WithVersion("v1"),
		"",
		"test",
		apiextensionsv1.Resource("customresourcedefinitions").WithVersion("v1"),
		"",
		admission.Create,
		&metav1.CreateOptions{},
		false,
		&user.DefaultInfo{},
	)
}

func createAttrAPIBinding(apiBinding *apisv1alpha2.APIBinding) admission.Attributes {
	return admission.NewAttributesRecord(
		helpers.ToUnstructuredOrDie(apiBinding),
		nil,
		apisv1alpha2.Kind("APIBinding").WithVersion("v1alpha2"),
		"",
		apiBinding.Name,
		apisv1alpha2.Resource("apibindings").WithVersion("v1alpha2"),
		"",
		admission.Create,
		&metav1.CreateOptions{},
		false,
		&user.DefaultInfo{},
	)
}

func updateAttr(obj, old *apiextensions.CustomResourceDefinition) admission.Attributes {
	return admission.NewAttributesRecord(
		obj,
		old,
		apiextensionsv1.Kind("CustomResourceDefinition").WithVersion("v1"),
		"",
		"test",
		apiextensionsv1.Resource("customresourcedefinitions").WithVersion("v1"),
		"",
		admission.Update,
		&metav1.CreateOptions{},
		false,
		&user.DefaultInfo{},
	)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		attr        admission.Attributes
		clusterName logicalcluster.Name

		wantErr bool
	}{
		{
			name: "passes create reserved group in system crd logical cluster",
			attr: createAttr(&apiextensions.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: apiextensions.CustomResourceDefinitionSpec{
					Group: "apis.kcp.io",
				},
			}),
			clusterName: "system:system-crds",
		},
		{
			name: "fails create reserved group outside of system crd logical cluster",
			attr: createAttr(&apiextensions.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: apiextensions.CustomResourceDefinitionSpec{
					Group: "apis.kcp.io",
				},
			}),
			wantErr:     true,
			clusterName: "root:org:ws",
		},
		{
			name: "passes create non-reserved group outside of system crd logical cluster",
			attr: createAttr(&apiextensions.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: apiextensions.CustomResourceDefinitionSpec{
					Group: "foo.dev",
				},
			}),
			clusterName: "root:org:ws",
		},
		{
			name: "passes not a CRD",
			attr: createAttrAPIBinding(&apisv1alpha2.APIBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			}),
			clusterName: "root:org:ws",
		},
		{
			name: "passes update reserved group in system crd logical cluster",
			attr: updateAttr(&apiextensions.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test",
					Labels: map[string]string{"a": "b"},
				},
				Spec: apiextensions.CustomResourceDefinitionSpec{
					Group: "apis.kcp.io",
				},
			},
				&apiextensions.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: apiextensions.CustomResourceDefinitionSpec{
						Group: "foo.apis.kcp.io",
					},
				}),
			clusterName: "system:system-crds",
		},
		{
			name: "fails update reserved group outside of system crd logical cluster",
			attr: updateAttr(&apiextensions.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test",
					Labels: map[string]string{"a": "b"},
				},
				Spec: apiextensions.CustomResourceDefinitionSpec{
					Group: "apis.kcp.io",
				},
			},
				&apiextensions.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: apiextensions.CustomResourceDefinitionSpec{
						Group: "foo.apis.kcp.io",
					},
				}),
			wantErr:     true,
			clusterName: "root:org:ws",
		},
		{
			name: "passes update non-reserved group outside of system crd logical cluster",
			attr: updateAttr(&apiextensions.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test",
					Labels: map[string]string{"a": "b"},
				},
				Spec: apiextensions.CustomResourceDefinitionSpec{
					Group: "bar.dev",
				},
			},
				&apiextensions.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: apiextensions.CustomResourceDefinitionSpec{
						Group: "bar.dev",
					},
				}),
			clusterName: "root:org:ws",
		},
		{
			name: "passes update child of reserved group outside of system crd logical cluster",
			attr: updateAttr(&apiextensions.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: apiextensions.CustomResourceDefinitionSpec{
					Group: "initialization.tenancy.kcp.io",
				},
			},
				&apiextensions.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: apiextensions.CustomResourceDefinitionSpec{
						Group: "initialization.tenancy.kcp.io",
					},
				}),
			clusterName: "root:org:ws",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &reservedCRDGroups{
				Handler: admission.NewHandler(admission.Create, admission.Update),
			}
			var ctx context.Context

			require.NotEmpty(t, tt.clusterName, "clusterName must be set in this test")

			ctx = request.WithCluster(context.Background(), request.Cluster{Name: tt.clusterName})
			if err := o.Validate(ctx, tt.attr, nil); (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
