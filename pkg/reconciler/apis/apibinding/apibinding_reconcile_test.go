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

package apibinding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/kcp-dev/logicalcluster/v3"

	apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	apisv1alpha2 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha2"
	corev1alpha1 "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
)

func TestReconcileNew(t *testing.T) {
	apiBinding := newBindingBuilder().
		WithClusterName("org:ws").
		WithName("my-binding").
		WithExportReference(logicalcluster.NewPath("org:some-workspace"), "some-export").
		Build()

	c := &controller{}

	t.Logf("Run only newReconciler because no phase is set")
	requeue, err := c.reconcile(context.Background(), apiBinding)
	require.NoError(t, err)
	require.Equal(t, apisv1alpha2.APIBindingPhaseBinding, apiBinding.Status.Phase)
	require.False(t, requeue)
	requireConditionMatches(t, apiBinding, conditions.FalseCondition(conditionsv1alpha1.ReadyCondition, "", "", ""))
}

func TestReconcileBinding(t *testing.T) {
	var (
		unbound = newBindingBuilder().
			WithCondition(&conditionsv1alpha1.Condition{
				Type:   apisv1alpha2.InitialBindingCompleted,
				Status: corev1.ConditionFalse,
			}).
			WithClusterName("org:ws").
			WithName("my-binding").
			WithExportReference(logicalcluster.NewPath("org:some-workspace"), "some-export")

		binding = unbound.DeepCopy().WithPhase(apisv1alpha2.APIBindingPhaseBinding)

		rebinding = binding.DeepCopy().
				WithBoundResources(new(boundAPIResourceBuilder).
					WithGroupResource("kcp.io", "widgets").
					WithSchema("today.widgets.kcp.io", "todaywidgetsuid").
					WithStorageVersions("v0", "v1").
					BoundAPIResource,
			)

		invalidSchema = binding.DeepCopy().WithExportReference(logicalcluster.NewPath("org:some-workspace"), "invalid-schema")

		bound = unbound.DeepCopy().
			WithPhase(apisv1alpha2.APIBindingPhaseBound).
			WithBoundResources(
				new(boundAPIResourceBuilder).
					WithGroupResource("mygroup", "someresources").
					WithSchema("today.someresources.mygroup", "uid1").
					BoundAPIResource,
				new(boundAPIResourceBuilder).
					WithGroupResource("anothergroup", "otherresources").
					WithSchema("today.someresources.anothergroup", "uid2").
					BoundAPIResource,
			)

		conflicting = unbound.DeepCopy().
				WithName("conflicting").
				WithPhase(apisv1alpha2.APIBindingPhaseBound).
				WithExportReference(logicalcluster.NewPath("org:some-workspace"), "conflict").
				WithBoundResources(new(boundAPIResourceBuilder).
					WithGroupResource("kcp.io", "widgets").
					WithSchema("another.widgets.kcp.io", "anotherwidgetsuid").
					BoundAPIResource,
			)

		todayWidgetsAPIResourceSchema = &apisv1alpha1.APIResourceSchema{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					logicalcluster.AnnotationKey: "org-some-workspace",
				},
				Name: "today.widgets.kcp.io",
				UID:  "todaywidgetsuid",
			},
			Spec: apisv1alpha1.APIResourceSchemaSpec{
				Group: "kcp.io",
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Plural:   "widgets",
					Singular: "widget",
					Kind:     "Widget",
					ListKind: "WidgetList",
				},
				Scope: "Namespace",
				Versions: []apisv1alpha1.APIResourceVersion{
					{
						Name:    "v1",
						Served:  true,
						Storage: true,
						Schema: runtime.RawExtension{
							Raw: []byte(`{"description":"foo","type":"object"}`),
						},
					},
				},
			},
		}

		someOtherWidgetsAPIResourceSchema = &apisv1alpha1.APIResourceSchema{
			ObjectMeta: metav1.ObjectMeta{
				Name: "another.widgets.kcp.io",
				UID:  "anotherwidgetsuid",
			},
			Spec: apisv1alpha1.APIResourceSchemaSpec{
				Group: "kcp.io",
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Plural:   "widgets",
					Singular: "widget",
					Kind:     "Widget",
					ListKind: "WidgetList",
				},
				Scope: "Namespace",
				Versions: []apisv1alpha1.APIResourceVersion{
					{
						Name:    "v1",
						Served:  true,
						Storage: true,
						Schema: runtime.RawExtension{
							Raw: []byte(`{"description":"foo","type":"object"}`),
						},
					},
				},
			},
		}
	)

	todaywidgetsuid := withName(newCRD("kcp.io", "widgets"), "todaywidgetsuid")
	anotherwidgetsuid := withName(newCRD("kcp.io", "widgets"), "anotherwidgetsuid")
	uid1 := withName(newCRD("someresources.mygroup", "todays"), "uid1")
	uid2 := withName(newCRD("someresources.anothergroup", "todays"), "uid2")

	type wantAPIExportValid struct {
		invalidReference bool
		notFound         bool
		internalError    bool
		valid            bool
	}
	type wantInitialBindingComplete struct {
		internalError          bool
		schemaInvalid          bool
		waitingForEstablished  bool
		logicalClusterNotFound bool
		resourceConflict       bool
		namingConflict         bool
		completed              bool
	}
	tests := map[string]struct {
		// input objects
		apiBinding                *apisv1alpha2.APIBinding
		getAPIExportError         error
		getAPIResourceSchemaError error
		existingAPIBindings       []*apisv1alpha2.APIBinding
		logicalCluster            *corev1alpha1.LogicalCluster
		updateLogicalClusterError error

		// reconcile result
		wantError   bool
		wantRequeue bool

		// bound CRDs
		crds           map[logicalcluster.Name][]*apiextensionsv1.CustomResourceDefinition
		getCRDError    error
		createCRDError error
		updateCRDError error
		wantCreateCRD  bool

		// Conditions
		wantAPIExportValid         wantAPIExportValid
		wantInitialBindingComplete wantInitialBindingComplete
		wantReady                  bool
		wantNoReady                bool

		// Bound resources
		wantPhaseBound     bool
		wantBoundResources []apisv1alpha2.BoundAPIResource
	}{
		"Update to nil workspace ref reports invalid APIExport": {
			apiBinding: binding.DeepCopy().WithoutWorkspaceReference().Build(),
			wantAPIExportValid: wantAPIExportValid{
				invalidReference: true,
			},
		},
		"APIExport not found": {
			apiBinding:        binding.Build(),
			getAPIExportError: apierrors.NewNotFound(apisv1alpha2.SchemeGroupVersion.WithResource("apiexports").GroupResource(), "some-export"),
			wantAPIExportValid: wantAPIExportValid{
				notFound: true,
			},
		},
		"APIExport get error - random error": {
			apiBinding:        binding.Build(),
			getAPIExportError: errors.New("foo"),
			wantAPIExportValid: wantAPIExportValid{
				internalError: true,
			},
			wantError: true,
		},
		"APIResourceSchema get error - not found": {
			apiBinding:                binding.Build(),
			getAPIResourceSchemaError: apierrors.NewNotFound(schema.GroupResource{}, "foo"),
			wantAPIExportValid: wantAPIExportValid{
				internalError: true,
			},
			wantError: false,
		},
		"APIResourceSchema get error - random error": {
			apiBinding:                binding.Build(),
			getAPIResourceSchemaError: errors.New("foo"),
			wantAPIExportValid: wantAPIExportValid{
				internalError: true,
			},
			wantError: true,
		},
		"APIExport doesn't have identity hash yet": {
			apiBinding: binding.DeepCopy().
				WithExportReference(logicalcluster.NewPath("org:some-workspace"), "no-identity-hash").
				Build(),
			wantAPIExportValid: wantAPIExportValid{
				valid: false,
			},
		},
		"No LogicalCluster": {
			apiBinding: binding.Build(),
			wantInitialBindingComplete: wantInitialBindingComplete{
				logicalClusterNotFound: true,
			},
			wantError: true,
		},
		"LogicalCluster without resource binding annotation": {
			logicalCluster: newLogicalCluster(),
			apiBinding:     binding.Build(),
			wantInitialBindingComplete: wantInitialBindingComplete{
				resourceConflict: true,
			},
			wantError: true,
		},
		"LogicalCluster update error": {
			logicalCluster:            withResourceBindings(newLogicalCluster(), ResourceBindingsAnnotation{}),
			apiBinding:                binding.Build(),
			updateLogicalClusterError: errors.New("foo"),
			wantError:                 true,
		},
		"Resource already bound by other APIBinding": {
			logicalCluster: withResourceBindings(newLogicalCluster(), ResourceBindingsAnnotation{
				"widgets.kcp.io": ExpirableLock{Lock: Lock{Name: "some-other-binding"}},
			}),
			apiBinding: binding.Build(),
			wantInitialBindingComplete: wantInitialBindingComplete{
				resourceConflict: true,
			},
		},
		"Resource already bound by CRD without expiry": {
			logicalCluster: withResourceBindings(newLogicalCluster(), ResourceBindingsAnnotation{
				"widgets.kcp.io": ExpirableLock{Lock: Lock{CRD: true}},
			}),
			apiBinding: binding.Build(),
			wantInitialBindingComplete: wantInitialBindingComplete{
				resourceConflict: true,
			},
		},
		"Resource already bound by unexpired CRD": {
			logicalCluster: withResourceBindings(newLogicalCluster(), ResourceBindingsAnnotation{
				"widgets.kcp.io": ExpirableLock{Lock: Lock{CRD: true}, CRDExpiry: ptr.To(metav1.Time{Time: time.Now().Add(time.Hour)})},
			}),
			apiBinding: binding.Build(),
			wantInitialBindingComplete: wantInitialBindingComplete{
				resourceConflict: true,
			},
		},
		"Resource already bound by expired, but existing CRD": {
			logicalCluster: withResourceBindings(newLogicalCluster(), ResourceBindingsAnnotation{
				"widgets.kcp.io": ExpirableLock{Lock: Lock{CRD: true}, CRDExpiry: ptr.To(metav1.Time{Time: time.Now().Add(-time.Hour)})},
			}),
			crds: map[logicalcluster.Name][]*apiextensionsv1.CustomResourceDefinition{
				logicalcluster.Name("org:ws"): {newCRD("kcp.io", "widgets")},
			},
			apiBinding: binding.Build(),
			wantInitialBindingComplete: wantInitialBindingComplete{
				resourceConflict: true,
			},
		},
		"Resource already bound by expired and non-existing CRD": {
			logicalCluster: withResourceBindings(newLogicalCluster(), ResourceBindingsAnnotation{
				"widgets.kcp.io": ExpirableLock{Lock: Lock{CRD: true}, CRDExpiry: ptr.To(metav1.Time{Time: time.Now().Add(-time.Hour)})},
			}),
			apiBinding:    binding.Build(),
			wantCreateCRD: true,
		},
		"APIResourceSchema invalid": {
			logicalCluster: withResourceBindings(newLogicalCluster(), ResourceBindingsAnnotation{}),
			apiBinding:     invalidSchema.Build(),
			wantAPIExportValid: wantAPIExportValid{
				internalError: true,
			},
		},
		"CRD get error": {
			logicalCluster: withResourceBindings(newLogicalCluster(), ResourceBindingsAnnotation{}),
			apiBinding:     binding.Build(),
			getCRDError:    errors.New("foo"),
			wantAPIExportValid: wantAPIExportValid{
				internalError: true,
			},
			wantError: true,
		},
		"create CRD fails - invalid": {
			logicalCluster: withResourceBindings(newLogicalCluster(), ResourceBindingsAnnotation{}),
			apiBinding:     binding.Build(),
			wantCreateCRD:  true,
			createCRDError: apierrors.NewInvalid(apiextensionsv1.Kind("CustomResourceDefinition"), "todaywidgetsuid", field.ErrorList{field.Forbidden(field.NewPath("foo"), "details")}),
			wantInitialBindingComplete: wantInitialBindingComplete{
				schemaInvalid: true,
			},
			wantError: false,
		},
		"create CRD fails - other error": {
			logicalCluster: withResourceBindings(newLogicalCluster(), ResourceBindingsAnnotation{}),
			apiBinding:     binding.Build(),
			wantCreateCRD:  true,
			createCRDError: errors.New("foo"),
			wantInitialBindingComplete: wantInitialBindingComplete{
				internalError: true,
			},
			wantError: true,
		},
		"create CRD - no other bindings": {
			logicalCluster: withResourceBindings(newLogicalCluster(), ResourceBindingsAnnotation{}),
			apiBinding:     binding.Build(),
			wantCreateCRD:  true,
			wantAPIExportValid: wantAPIExportValid{
				valid: true,
			},
			wantInitialBindingComplete: wantInitialBindingComplete{
				waitingForEstablished: true,
			},
			wantBoundResources: nil, // not yet established
		},
		"create CRD - other bindings - no conflicts": {
			logicalCluster: withResourceBindings(newLogicalCluster(), ResourceBindingsAnnotation{}),
			apiBinding:     binding.Build(),
			existingAPIBindings: []*apisv1alpha2.APIBinding{
				bound.Build(),
			},
			crds: map[logicalcluster.Name][]*apiextensionsv1.CustomResourceDefinition{
				SystemBoundCRDsClusterName: {anotherwidgetsuid, uid1, uid2},
			},
			wantCreateCRD: true,
			wantAPIExportValid: wantAPIExportValid{
				valid: true,
			},
			wantInitialBindingComplete: wantInitialBindingComplete{
				waitingForEstablished: true,
			},
			wantBoundResources: nil, // not yet established
		},
		"create CRD - other bindings - conflicts": {
			logicalCluster: withResourceBindings(newLogicalCluster(), ResourceBindingsAnnotation{}),
			apiBinding:     binding.Build(),
			existingAPIBindings: []*apisv1alpha2.APIBinding{
				conflicting.Build(),
			},
			crds: map[logicalcluster.Name][]*apiextensionsv1.CustomResourceDefinition{
				SystemBoundCRDsClusterName: {todaywidgetsuid, anotherwidgetsuid},
			},
			wantInitialBindingComplete: wantInitialBindingComplete{
				namingConflict: true,
			},
			wantError:   true,
			wantRequeue: true,
			wantNoReady: true,
		},
		"bind existing CRD - other bindings - conflicts": {
			logicalCluster: withResourceBindings(newLogicalCluster(), ResourceBindingsAnnotation{}),
			apiBinding:     binding.Build(),
			crds: map[logicalcluster.Name][]*apiextensionsv1.CustomResourceDefinition{
				SystemBoundCRDsClusterName: {todaywidgetsuid, anotherwidgetsuid},
			},
			existingAPIBindings: []*apisv1alpha2.APIBinding{
				conflicting.Build(),
			},
			wantInitialBindingComplete: wantInitialBindingComplete{
				namingConflict: true,
			},
			wantError:   true,
			wantRequeue: true,
			wantNoReady: true,
		},
		"CRD already exists but isn't established yet": {
			logicalCluster: withResourceBindings(newLogicalCluster(), ResourceBindingsAnnotation{}),
			apiBinding:     binding.Build(),
			crds: map[logicalcluster.Name][]*apiextensionsv1.CustomResourceDefinition{
				SystemBoundCRDsClusterName: {withStoredVersions(todaywidgetsuid, "v0", "v1")},
			},
			wantAPIExportValid: wantAPIExportValid{
				valid: true,
			},
			wantInitialBindingComplete: wantInitialBindingComplete{
				waitingForEstablished: true,
			},
			wantReady:          false,
			wantBoundResources: nil, // not established yet
		},
		"CRD already exists and is established": {
			logicalCluster: withResourceBindings(newLogicalCluster(), ResourceBindingsAnnotation{}),
			apiBinding:     binding.Build(),
			crds: map[logicalcluster.Name][]*apiextensionsv1.CustomResourceDefinition{
				SystemBoundCRDsClusterName: {withEstablished(withStoredVersions(todaywidgetsuid, "v0", "v1"))},
			},
			wantAPIExportValid: wantAPIExportValid{
				valid: true,
			},
			wantReady: true,
			wantBoundResources: []apisv1alpha2.BoundAPIResource{
				{
					Group:    "kcp.io",
					Resource: "widgets",
					Schema: apisv1alpha2.BoundAPIResourceSchema{
						Name:         "today.widgets.kcp.io",
						UID:          "todaywidgetsuid",
						IdentityHash: "hash1",
					},
					StorageVersions: []string{"v0", "v1"},
				},
			},
			wantPhaseBound: true,
			wantInitialBindingComplete: wantInitialBindingComplete{
				completed: true,
			},
		},
		"Ensure merging storage versions works": {
			logicalCluster: withResourceBindings(newLogicalCluster(), ResourceBindingsAnnotation{}),
			apiBinding:     rebinding.Build(),
			crds: map[logicalcluster.Name][]*apiextensionsv1.CustomResourceDefinition{
				SystemBoundCRDsClusterName: {withEstablished(withStoredVersions(todaywidgetsuid, "v2")), anotherwidgetsuid},
			},
			wantAPIExportValid: wantAPIExportValid{
				valid: true,
			},
			wantReady: true,
			wantBoundResources: []apisv1alpha2.BoundAPIResource{
				{
					Group:    "kcp.io",
					Resource: "widgets",
					Schema: apisv1alpha2.BoundAPIResourceSchema{
						Name:         "today.widgets.kcp.io",
						UID:          "todaywidgetsuid",
						IdentityHash: "hash1",
					},
					StorageVersions: []string{"v0", "v1", "v2"},
				},
			},
			wantPhaseBound: true,
			wantInitialBindingComplete: wantInitialBindingComplete{
				completed: true,
			},
		},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			createCRDCalled := false

			apiExports := map[string]*apisv1alpha2.APIExport{
				"some-export": {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							logicalcluster.AnnotationKey: "org-some-workspace",
						},
						Name: "some-export",
					},
					Spec: apisv1alpha2.APIExportSpec{
						Resources: []apisv1alpha2.ResourceSchema{
							{
								Name:   "widgets",
								Group:  "kcp.io",
								Schema: "today.widgets.kcp.io",
								Storage: apisv1alpha2.ResourceSchemaStorage{
									CRD: &apisv1alpha2.ResourceSchemaStorageCRD{},
								},
							},
						},
					},
					Status: apisv1alpha2.APIExportStatus{IdentityHash: "hash1"},
				},
				"conflict": {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							logicalcluster.AnnotationKey: "org-some-workspace",
						},
						Name: "conflict",
					},
					Spec: apisv1alpha2.APIExportSpec{
						Resources: []apisv1alpha2.ResourceSchema{
							{
								Name:   "widgets",
								Group:  "kcp.io",
								Schema: "another.widgets.kcp.io",
								Storage: apisv1alpha2.ResourceSchemaStorage{
									CRD: &apisv1alpha2.ResourceSchemaStorageCRD{},
								},
							},
						},
					},
					Status: apisv1alpha2.APIExportStatus{IdentityHash: "hash2"},
				},
				"invalid-schema": {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							logicalcluster.AnnotationKey: "org-some-workspace",
						},
						Name: "invalid-schema",
					},
					Spec: apisv1alpha2.APIExportSpec{
						Resources: []apisv1alpha2.ResourceSchema{
							{
								Name:   "schema",
								Group:  "io",
								Schema: "invalid.schema.io",
								Storage: apisv1alpha2.ResourceSchemaStorage{
									CRD: &apisv1alpha2.ResourceSchemaStorageCRD{},
								},
							},
						},
					},
					Status: apisv1alpha2.APIExportStatus{IdentityHash: "hash3"},
				},
				"no-identity-hash": {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							logicalcluster.AnnotationKey: "org-some-workspace",
						},
						Name: "some-export",
					},
					Spec: apisv1alpha2.APIExportSpec{
						Resources: []apisv1alpha2.ResourceSchema{
							{
								Name:   "widgets",
								Group:  "kcp.io",
								Schema: "today.widgets.kcp.io",
								Storage: apisv1alpha2.ResourceSchemaStorage{
									CRD: &apisv1alpha2.ResourceSchemaStorageCRD{},
								},
							},
						},
					},
				},
			}

			apiResourceSchemas := map[string]*apisv1alpha1.APIResourceSchema{
				"invalid.schema.io": {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							logicalcluster.AnnotationKey: "org-some-workspace",
						},
						Name: "invalid.schema.io",
					},
					Spec: apisv1alpha1.APIResourceSchemaSpec{
						Versions: []apisv1alpha1.APIResourceVersion{
							{
								Schema: runtime.RawExtension{
									Raw: []byte("invalid schema"),
								},
							},
						},
					},
				},
				"today.widgets.kcp.io":   todayWidgetsAPIResourceSchema,
				"another.widgets.kcp.io": someOtherWidgetsAPIResourceSchema,
			}

			c := &controller{
				listAPIBindings: func(clusterName logicalcluster.Name) ([]*apisv1alpha2.APIBinding, error) {
					return tc.existingAPIBindings, nil
				},
				getAPIExportByPath: func(path logicalcluster.Path, name string) (*apisv1alpha2.APIExport, error) {
					require.Equal(t, "org:some-workspace", path.String())
					return apiExports[name], tc.getAPIExportError
				},
				getAPIResourceSchema: func(clusterName logicalcluster.Name, name string) (*apisv1alpha1.APIResourceSchema, error) {
					if tc.getAPIResourceSchemaError != nil {
						return nil, tc.getAPIResourceSchemaError
					}

					require.Equal(t, "org-some-workspace", clusterName.String())

					s, ok := apiResourceSchemas[name]
					if !ok {
						return nil, apierrors.NewNotFound(apisv1alpha1.Resource("apiresourceschemas"), name)
					}

					return s, nil
				},
				getCRD: func(clusterName logicalcluster.Name, name string) (*apiextensionsv1.CustomResourceDefinition, error) {
					if tc.getCRDError != nil {
						return nil, tc.getCRDError
					}

					crds := tc.crds[clusterName]
					for _, crd := range crds {
						if crd.Name == name {
							return crd, nil
						}
					}
					return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
				},
				listCRDs: func(clusterName logicalcluster.Name) ([]*apiextensionsv1.CustomResourceDefinition, error) {
					return tc.crds[clusterName], nil
				},
				createCRD: func(ctx context.Context, clusterName logicalcluster.Path, crd *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error) {
					createCRDCalled = true
					return crd, tc.createCRDError
				},
				getLogicalCluster: func(clusterName logicalcluster.Name) (*corev1alpha1.LogicalCluster, error) {
					if tc.logicalCluster == nil {
						return nil, apierrors.NewNotFound(schema.GroupResource{}, clusterName.String())
					}
					return tc.logicalCluster, nil
				},
				updateLogicalCluster: func(ctx context.Context, lc *corev1alpha1.LogicalCluster) error {
					if tc.updateLogicalClusterError != nil {
						return tc.updateLogicalClusterError
					}
					return nil
				},
				deletedCRDTracker: &lockedStringSet{},
			}

			requeue, err := c.reconcile(context.Background(), tc.apiBinding)

			// reconcile results
			if tc.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantRequeue, requeue, "mismatched requeue, want: %v, got: %v", tc.wantRequeue, requeue)

			// CRD creation
			require.Equal(t, tc.wantCreateCRD, createCRDCalled, "mismatch on CRD creation expectation")

			// APIExportValid condition
			if tc.wantAPIExportValid.invalidReference {
				requireConditionMatches(t, tc.apiBinding, &conditionsv1alpha1.Condition{
					Type:     apisv1alpha2.APIExportValid,
					Status:   corev1.ConditionFalse,
					Severity: conditionsv1alpha1.ConditionSeverityError,
					Reason:   apisv1alpha2.APIExportInvalidReferenceReason,
				})
			}
			if tc.wantAPIExportValid.notFound {
				requireConditionMatches(t, tc.apiBinding, &conditionsv1alpha1.Condition{
					Type:     apisv1alpha2.APIExportValid,
					Status:   corev1.ConditionFalse,
					Severity: conditionsv1alpha1.ConditionSeverityError,
					Reason:   apisv1alpha2.APIExportNotFoundReason,
				})
			}
			if tc.wantAPIExportValid.internalError {
				requireConditionMatches(t, tc.apiBinding, &conditionsv1alpha1.Condition{
					Type:     apisv1alpha2.APIExportValid,
					Status:   corev1.ConditionFalse,
					Severity: conditionsv1alpha1.ConditionSeverityError,
					Reason:   apisv1alpha2.InternalErrorReason,
				})
			}
			if tc.wantAPIExportValid.valid {
				requireConditionMatches(t, tc.apiBinding, conditions.TrueCondition(apisv1alpha2.APIExportValid))
			}

			// InitialBindingCompleted condition
			if tc.wantInitialBindingComplete.waitingForEstablished {
				requireConditionMatches(t, tc.apiBinding, &conditionsv1alpha1.Condition{
					Type:     apisv1alpha2.InitialBindingCompleted,
					Status:   corev1.ConditionFalse,
					Severity: conditionsv1alpha1.ConditionSeverityInfo,
					Reason:   apisv1alpha2.WaitingForEstablishedReason,
				})
			}
			if tc.wantInitialBindingComplete.internalError {
				requireConditionMatches(t, tc.apiBinding, &conditionsv1alpha1.Condition{
					Type:     apisv1alpha2.InitialBindingCompleted,
					Status:   corev1.ConditionFalse,
					Severity: conditionsv1alpha1.ConditionSeverityError,
					Reason:   apisv1alpha2.InternalErrorReason,
				})
			}
			if tc.wantInitialBindingComplete.schemaInvalid {
				requireConditionMatches(t, tc.apiBinding, &conditionsv1alpha1.Condition{
					Type:     apisv1alpha2.InitialBindingCompleted,
					Status:   corev1.ConditionFalse,
					Severity: conditionsv1alpha1.ConditionSeverityError,
					Reason:   apisv1alpha2.APIResourceSchemaInvalidReason,
				})
			}
			if tc.wantInitialBindingComplete.logicalClusterNotFound {
				requireConditionMatches(t, tc.apiBinding, &conditionsv1alpha1.Condition{
					Type:     apisv1alpha2.InitialBindingCompleted,
					Status:   corev1.ConditionFalse,
					Severity: conditionsv1alpha1.ConditionSeverityError,
					Reason:   apisv1alpha2.LogicalClusterNotFoundReason,
				})
			}
			if tc.wantInitialBindingComplete.resourceConflict {
				requireConditionMatches(t, tc.apiBinding, &conditionsv1alpha1.Condition{
					Type:     apisv1alpha2.InitialBindingCompleted,
					Status:   corev1.ConditionFalse,
					Severity: conditionsv1alpha1.ConditionSeverityError,
					Reason:   apisv1alpha2.NamingConflictsReason,
				})
			}
			if tc.wantInitialBindingComplete.namingConflict {
				requireConditionMatches(t, tc.apiBinding, &conditionsv1alpha1.Condition{
					Type:     apisv1alpha2.InitialBindingCompleted,
					Status:   corev1.ConditionFalse,
					Severity: conditionsv1alpha1.ConditionSeverityError,
					Reason:   apisv1alpha2.NamingConflictsReason,
					Message:  "naming conflict with APIBinding \"conflicting\" bound to APIExport org:some-workspace:conflict: spec.names.plural=widgets is forbidden",
				})
			}
			if tc.wantInitialBindingComplete.completed {
				requireConditionMatches(t, tc.apiBinding, conditions.TrueCondition(apisv1alpha2.InitialBindingCompleted))
			}

			// Ready condition
			if tc.wantNoReady {
				require.False(t, conditions.Has(tc.apiBinding, conditionsv1alpha1.ReadyCondition), "unexpected Ready condition")
			} else if tc.wantReady {
				requireConditionMatches(t, tc.apiBinding, conditions.TrueCondition(conditionsv1alpha1.ReadyCondition))
			} else {
				requireConditionMatches(t, tc.apiBinding, conditions.FalseCondition(conditionsv1alpha1.ReadyCondition, "", "", ""))
			}

			// Phase
			if tc.wantPhaseBound {
				require.Equal(t, apisv1alpha2.APIBindingPhaseBound, tc.apiBinding.Status.Phase)
			} else {
				require.Equal(t, apisv1alpha2.APIBindingPhaseBinding, tc.apiBinding.Status.Phase)
			}

			// Bound resources
			require.Len(t, tc.apiBinding.Status.BoundResources, len(tc.wantBoundResources), "unexpected bound resources")
			for _, want := range tc.wantBoundResources {
				found := false
				for _, got := range tc.apiBinding.Status.BoundResources {
					if got.Group != want.Group || got.Resource != want.Resource {
						continue
					}
					found = true

					require.Equal(t, want, got)
				}

				require.True(t, found, "expected bound resource group=%s resource=%s", want.Group, want.Resource)
			}
		})
	}
}

func TestCRDFromAPIResourceSchema(t *testing.T) {
	tests := map[string]struct {
		schema  *apisv1alpha1.APIResourceSchema
		want    *apiextensionsv1.CustomResourceDefinition
		wantErr bool
	}{
		"full schema": {
			schema: &apisv1alpha1.APIResourceSchema{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "my-cluster",
					},
					Name: "my-name",
					UID:  types.UID("my-uuid"),
				},
				Spec: apisv1alpha1.APIResourceSchemaSpec{
					Group: "my-group",
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Plural:     "widgets",
						Singular:   "widget",
						ShortNames: []string{"w"},
						Kind:       "Widget",
						ListKind:   "WidgetList",
						Categories: []string{"things"},
					},
					Scope: apiextensionsv1.NamespaceScoped,
					Versions: []apisv1alpha1.APIResourceVersion{
						{
							Name:               "v1",
							Served:             true,
							Storage:            false,
							Deprecated:         true,
							DeprecationWarning: ptr.To("deprecated!"),
							Schema: runtime.RawExtension{
								Raw: []byte(`
{
	"description": "foo",
	"type": "object"
}
								`),
							},
							Subresources: apiextensionsv1.CustomResourceSubresources{
								Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
								Scale: &apiextensionsv1.CustomResourceSubresourceScale{
									SpecReplicasPath:   ".spec.replicas",
									StatusReplicasPath: ".status.replicas",
									LabelSelectorPath:  ptr.To(".status.selector"),
								},
							},
							AdditionalPrinterColumns: []apiextensionsv1.CustomResourceColumnDefinition{
								{
									Name:        "My Column",
									Type:        "string",
									Format:      "string",
									Description: "This is my column",
									Priority:    1,
									JSONPath:    ".spec.myColumn",
								},
							},
						},
						{
							Name:       "v2",
							Served:     true,
							Storage:    true,
							Deprecated: false,
							Schema: runtime.RawExtension{
								Raw: []byte(`
{
	"description": "foo",
	"type": "object"
}
								`),
							},
							Subresources: apiextensionsv1.CustomResourceSubresources{
								Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
								Scale: &apiextensionsv1.CustomResourceSubresourceScale{
									SpecReplicasPath:   ".spec.replicas",
									StatusReplicasPath: ".status.replicas",
									LabelSelectorPath:  ptr.To(".status.selector"),
								},
							},
							AdditionalPrinterColumns: []apiextensionsv1.CustomResourceColumnDefinition{
								{
									Name:        "My Column",
									Type:        "string",
									Format:      "string",
									Description: "This is my column",
									Priority:    1,
									JSONPath:    ".spec.myColumn",
								},
							},
						},
					},
					Conversion: &apisv1alpha1.CustomResourceConversion{
						Strategy: "None",
					},
				},
			},
			want: &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-uuid",
					Annotations: map[string]string{
						logicalcluster.AnnotationKey:            SystemBoundCRDsClusterName.String(),
						apisv1alpha1.AnnotationBoundCRDKey:      "",
						apisv1alpha1.AnnotationSchemaClusterKey: "my-cluster",
						apisv1alpha1.AnnotationSchemaNameKey:    "my-name",
					},
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Group: "my-group",
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Plural:     "widgets",
						Singular:   "widget",
						ShortNames: []string{"w"},
						Kind:       "Widget",
						ListKind:   "WidgetList",
						Categories: []string{"things"},
					},
					Scope: apiextensionsv1.NamespaceScoped,
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{
							Name:               "v1",
							Served:             true,
							Storage:            false,
							Deprecated:         true,
							DeprecationWarning: ptr.To("deprecated!"),
							Schema: &apiextensionsv1.CustomResourceValidation{
								OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
									Description: "foo",
									Type:        "object",
								},
							},
							Subresources: &apiextensionsv1.CustomResourceSubresources{
								Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
								Scale: &apiextensionsv1.CustomResourceSubresourceScale{
									SpecReplicasPath:   ".spec.replicas",
									StatusReplicasPath: ".status.replicas",
									LabelSelectorPath:  ptr.To(".status.selector"),
								},
							},
							AdditionalPrinterColumns: []apiextensionsv1.CustomResourceColumnDefinition{
								{
									Name:        "My Column",
									Type:        "string",
									Format:      "string",
									Description: "This is my column",
									Priority:    1,
									JSONPath:    ".spec.myColumn",
								},
							},
						},
						{
							Name:       "v2",
							Served:     true,
							Storage:    true,
							Deprecated: false,
							Schema: &apiextensionsv1.CustomResourceValidation{
								OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
									Description: "foo",
									Type:        "object",
								},
							},
							Subresources: &apiextensionsv1.CustomResourceSubresources{
								Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
								Scale: &apiextensionsv1.CustomResourceSubresourceScale{
									SpecReplicasPath:   ".spec.replicas",
									StatusReplicasPath: ".status.replicas",
									LabelSelectorPath:  ptr.To(".status.selector"),
								},
							},
							AdditionalPrinterColumns: []apiextensionsv1.CustomResourceColumnDefinition{
								{
									Name:        "My Column",
									Type:        "string",
									Format:      "string",
									Description: "This is my column",
									Priority:    1,
									JSONPath:    ".spec.myColumn",
								},
							},
						},
					},
					Conversion: &apiextensionsv1.CustomResourceConversion{
						Strategy: "None",
					},
					PreserveUnknownFields: false,
				},
			},
			wantErr: false,
		},
		"error when multiple versions specified but no conversion strategy type": {
			schema: &apisv1alpha1.APIResourceSchema{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "my-cluster",
					},
					Name: "my-name",
					UID:  types.UID("my-uuid"),
				},
				Spec: apisv1alpha1.APIResourceSchemaSpec{
					Group: "my-group",
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Plural:     "widgets",
						Singular:   "widget",
						ShortNames: []string{"w"},
						Kind:       "Widget",
						ListKind:   "WidgetList",
						Categories: []string{"things"},
					},
					Scope: apiextensionsv1.NamespaceScoped,
					Versions: []apisv1alpha1.APIResourceVersion{
						{
							Name:               "v1",
							Served:             true,
							Storage:            false,
							Deprecated:         true,
							DeprecationWarning: ptr.To("deprecated!"),
							Schema: runtime.RawExtension{
								Raw: []byte(`
{
	"description": "foo",
	"type": "object"
}
								`),
							},
							Subresources: apiextensionsv1.CustomResourceSubresources{
								Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
								Scale: &apiextensionsv1.CustomResourceSubresourceScale{
									SpecReplicasPath:   ".spec.replicas",
									StatusReplicasPath: ".status.replicas",
									LabelSelectorPath:  ptr.To(".status.selector"),
								},
							},
							AdditionalPrinterColumns: []apiextensionsv1.CustomResourceColumnDefinition{
								{
									Name:        "My Column",
									Type:        "string",
									Format:      "string",
									Description: "This is my column",
									Priority:    1,
									JSONPath:    ".spec.myColumn",
								},
							},
						},
						{
							Name:       "v2",
							Served:     true,
							Storage:    true,
							Deprecated: false,
							Schema: runtime.RawExtension{
								Raw: []byte(`
{
	"description": "foo",
	"type": "object"
}
								`),
							},
							Subresources: apiextensionsv1.CustomResourceSubresources{
								Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
								Scale: &apiextensionsv1.CustomResourceSubresourceScale{
									SpecReplicasPath:   ".spec.replicas",
									StatusReplicasPath: ".status.replicas",
									LabelSelectorPath:  ptr.To(".status.selector"),
								},
							},
							AdditionalPrinterColumns: []apiextensionsv1.CustomResourceColumnDefinition{
								{
									Name:        "My Column",
									Type:        "string",
									Format:      "string",
									Description: "This is my column",
									Priority:    1,
									JSONPath:    ".spec.myColumn",
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		"error when schema is invalid": {
			schema: &apisv1alpha1.APIResourceSchema{
				Spec: apisv1alpha1.APIResourceSchemaSpec{
					Versions: []apisv1alpha1.APIResourceVersion{
						{
							Schema: runtime.RawExtension{
								Raw: []byte("invalid json"),
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			got, err := generateCRD(tc.schema)

			if tc.wantErr != (err != nil) {
				t.Fatalf("wantErr: %v, got %v", tc.wantErr, err)
			}
			if tc.wantErr {
				return
			}

			require.Equal(t, tc.want, got)
		})
	}
}

// TODO(ncdc): this is a modified copy from apibinding admission. Unify these into a reusable package.
type bindingBuilder struct {
	apisv1alpha2.APIBinding
}

func newBindingBuilder() *bindingBuilder {
	b := new(bindingBuilder)
	return b
}

func (b *bindingBuilder) DeepCopy() *bindingBuilder {
	return &bindingBuilder{
		APIBinding: *b.APIBinding.DeepCopy(),
	}
}

func (b *bindingBuilder) Build() *apisv1alpha2.APIBinding {
	return b.APIBinding.DeepCopy()
}

func (b *bindingBuilder) WithClusterName(clusterName logicalcluster.Name) *bindingBuilder {
	if b.Annotations == nil {
		b.Annotations = make(map[string]string)
	}
	b.Annotations[logicalcluster.AnnotationKey] = string(clusterName)
	return b
}

func (b *bindingBuilder) WithName(name string) *bindingBuilder {
	b.Name = name
	return b
}

func (b *bindingBuilder) WithCondition(c *conditionsv1alpha1.Condition) *bindingBuilder {
	conditions.Set(b, c)
	return b
}

func (b *bindingBuilder) WithoutWorkspaceReference() *bindingBuilder {
	b.Spec.Reference.Export = nil
	return b
}

func (b *bindingBuilder) WithExportReference(path logicalcluster.Path, exportName string) *bindingBuilder {
	b.Spec.Reference.Export = &apisv1alpha2.ExportBindingReference{
		Path: path.String(),
		Name: exportName,
	}
	return b
}

func (b *bindingBuilder) WithPhase(phase apisv1alpha2.APIBindingPhaseType) *bindingBuilder {
	b.Status.Phase = phase
	return b
}

func (b *bindingBuilder) WithBoundResources(boundResources ...apisv1alpha2.BoundAPIResource) *bindingBuilder {
	b.Status.BoundResources = boundResources
	return b
}

type boundAPIResourceBuilder struct {
	apisv1alpha2.BoundAPIResource
}

func (b *boundAPIResourceBuilder) WithGroupResource(group, resource string) *boundAPIResourceBuilder {
	b.Group = group
	b.Resource = resource
	return b
}

func (b *boundAPIResourceBuilder) WithSchema(name, uid string) *boundAPIResourceBuilder {
	b.Schema = apisv1alpha2.BoundAPIResourceSchema{
		Name: name,
		UID:  uid,
	}
	return b
}

func (b *boundAPIResourceBuilder) WithStorageVersions(v ...string) *boundAPIResourceBuilder {
	b.StorageVersions = v
	return b
}

func newCRD(group, resource string) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.%s", resource, group),
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural: resource,
			},
		},
		Status: apiextensionsv1.CustomResourceDefinitionStatus{
			AcceptedNames: apiextensionsv1.CustomResourceDefinitionNames{
				Plural: resource,
			},
		},
	}
}

func withName(crd *apiextensionsv1.CustomResourceDefinition, name string) *apiextensionsv1.CustomResourceDefinition {
	crd = crd.DeepCopy()
	crd.Name = name
	return crd
}

func withStoredVersions(crd *apiextensionsv1.CustomResourceDefinition, versions ...string) *apiextensionsv1.CustomResourceDefinition {
	crd = crd.DeepCopy()
	crd.Status.StoredVersions = versions
	return crd
}

func withEstablished(crd *apiextensionsv1.CustomResourceDefinition) *apiextensionsv1.CustomResourceDefinition {
	crd = crd.DeepCopy()
	crd.Status.Conditions = append(crd.Status.Conditions, apiextensionsv1.CustomResourceDefinitionCondition{
		Type:   apiextensionsv1.Established,
		Status: apiextensionsv1.ConditionTrue,
	})
	return crd
}

func newLogicalCluster() *corev1alpha1.LogicalCluster {
	return &corev1alpha1.LogicalCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: corev1alpha1.LogicalClusterName,
		},
	}
}

func withResourceBindings(lc *corev1alpha1.LogicalCluster, rbs ResourceBindingsAnnotation) *corev1alpha1.LogicalCluster {
	bs, err := json.Marshal(rbs)
	if err != nil {
		panic(err)
	}

	lc = lc.DeepCopy()
	lc.Annotations = make(map[string]string)
	lc.Annotations[ResourceBindingsAnnotationKey] = string(bs)
	return lc
}

// requireConditionMatches looks for a condition matching c in g. Only fields that are set in c are compared (Type is
// required, though). If c.Message is set, the test performed is contains rather than an exact match.
func requireConditionMatches(t *testing.T, g conditions.Getter, c *conditionsv1alpha1.Condition) {
	t.Helper()

	actual := conditions.Get(g, c.Type)
	require.NotNil(t, actual, "missing condition %q", c.Type)

	if c.Status != "" {
		require.Equal(t, c.Status, actual.Status, "missing condition %q status %q", c.Type, c.Status)
	}

	if c.Severity != "" {
		require.Equal(t, c.Severity, actual.Severity, "missing condition %q severity %q", c.Type, c.Severity)
	}

	if c.Reason != "" {
		require.Equal(t, c.Reason, actual.Reason, "missing condition %q reason %q", c.Type, c.Reason)
	}

	if c.Message != "" {
		require.Contains(t, actual.Message, c.Message, "missing condition %q containing %q in message", c.Type, c.Message)
	}
}
