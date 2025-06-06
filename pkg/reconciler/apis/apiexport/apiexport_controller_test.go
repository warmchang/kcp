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

package apiexport

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kcp-dev/logicalcluster/v3"

	kcpfeatures "github.com/kcp-dev/kcp/pkg/features"
	apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	apisv1alpha2 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha2"
	corev1alpha1 "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
)

func TestReconcile(t *testing.T) {
	// Save original feature gate value
	originalFeatureGate := kcpfeatures.DefaultFeatureGate.Enabled(kcpfeatures.EnableDeprecatedAPIExportVirtualWorkspacesUrls)

	tests := map[string]struct {
		secretRefSet                         bool
		secretExists                         bool
		createSecretError                    error
		keyMissing                           bool
		secretHashDoesntMatchAPIExportStatus bool
		apiExportHasExpectedHash             bool
		apiExportHasSomeOtherHash            bool
		hasPreexistingVerifyFailure          bool
		listShardsError                      error
		apiExportEndpointSliceNotFound       bool
		skipEndpointSliceAnnotation          bool

		apiBindings []interface{}

		wantGenerationFailed             bool
		wantError                        bool
		wantCreateSecretCalled           bool
		wantUnsetIdentity                bool
		wantDefaultSecretRef             bool
		wantStatusHashSet                bool
		wantVerifyFailure                bool
		wantIdentityValid                bool
		wantCreateAPIExportEndpointSlice bool
		wantVirtualWorkspaceURLs         bool
	}{
		"create secret when ref is nil and secret doesn't exist": {
			secretExists: false,

			wantCreateSecretCalled: true,
			wantDefaultSecretRef:   true,
		},
		"error creating secret - identity not valid": {
			secretExists:      false,
			createSecretError: errors.New("foo"),

			wantCreateSecretCalled: true,
			wantUnsetIdentity:      true,
			wantGenerationFailed:   true,
			wantError:              true,
		},
		"set ref if default secret exists": {
			secretExists: true,

			wantDefaultSecretRef: true,
		},
		"status hash updated when unset": {
			secretRefSet: true,
			secretExists: true,

			wantStatusHashSet: true,
			wantIdentityValid: true,
		},
		"identity verification fails when reference secret doesn't exist": {
			secretRefSet: true,
			secretExists: false,

			wantVerifyFailure: true,
		},
		"identity verification fails when hash from secret's key differs with APIExport's hash": {
			secretRefSet:                         true,
			secretExists:                         true,
			apiExportHasExpectedHash:             true,
			secretHashDoesntMatchAPIExportStatus: true,

			wantVerifyFailure: true,
		},
		"able to fix identity verification by returning to secret with correct key/hash": {
			secretRefSet:                true,
			secretExists:                true,
			apiExportHasExpectedHash:    true,
			hasPreexistingVerifyFailure: true,

			wantIdentityValid: true,
		},
		"error listing shards": {
			secretRefSet: true,
			secretExists: true,

			wantStatusHashSet: true,
			wantIdentityValid: true,

			apiBindings: []interface{}{
				"something",
			},
			listShardsError: errors.New("foo"),
		},
		"create APIExportEndpointSlice when APIBindings present": {
			secretRefSet: true,
			secretExists: true,

			wantStatusHashSet:                true,
			apiExportEndpointSliceNotFound:   true,
			wantCreateAPIExportEndpointSlice: true,
			wantIdentityValid:                true,
		},
		"skip APIExportEndpointSlice creation when skip annotation is present": {
			secretRefSet: true,
			secretExists: true,

			wantStatusHashSet:                true,
			apiExportEndpointSliceNotFound:   true,
			wantCreateAPIExportEndpointSlice: false,
			wantIdentityValid:                true,
			skipEndpointSliceAnnotation:      true,
		},
		"virtual workspace URLs when feature gate enabled": {
			secretRefSet: true,
			secretExists: true,

			wantStatusHashSet:        true,
			wantIdentityValid:        true,
			wantVirtualWorkspaceURLs: true,
		},
		"error listing shards with feature gate enabled": {
			secretRefSet: true,
			secretExists: true,

			wantStatusHashSet:        true,
			wantIdentityValid:        true,
			wantVirtualWorkspaceURLs: false,
			listShardsError:          errors.New("foo"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Skip virtual workspace URL tests if feature gate is not enabled
			if tc.wantVirtualWorkspaceURLs && !originalFeatureGate {
				t.Skip("Skipping test that requires EnableDeprecatedAPIExportVirtualWorkspacesUrls feature gate")
			}

			createSecretCalled := false
			createEndpointSliceCalled := false

			expectedKey := "abc"
			expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(expectedKey)))
			someOtherKey := "def"

			c := &controller{
				getNamespace: func(clusterName logicalcluster.Name, name string) (*corev1.Namespace, error) {
					return &corev1.Namespace{}, nil
				},
				createNamespace: func(ctx context.Context, clusterName logicalcluster.Path, ns *corev1.Namespace) error {
					return nil
				},
				secretNamespace: "default-ns",
				getAPIExportEndpointSlice: func(clusterName logicalcluster.Name, name string) (*apisv1alpha1.APIExportEndpointSlice, error) {
					if tc.apiExportEndpointSliceNotFound {
						return nil, apierrors.NewNotFound(corev1.Resource("apiexportendpointslices"), name)
					}
					return &apisv1alpha1.APIExportEndpointSlice{}, nil
				},
				createAPIExportEndpointSlice: func(ctx context.Context, clusterName logicalcluster.Path, apiExportEndpointSlice *apisv1alpha1.APIExportEndpointSlice) error {
					createEndpointSliceCalled = true
					return nil
				},
				getSecret: func(ctx context.Context, clusterName logicalcluster.Name, ns, name string) (*corev1.Secret, error) {
					if tc.secretExists {
						secret := &corev1.Secret{
							Data: map[string][]byte{},
						}
						if !tc.keyMissing {
							if tc.secretHashDoesntMatchAPIExportStatus {
								secret.Data[apisv1alpha1.SecretKeyAPIExportIdentity] = []byte(someOtherKey)
							} else {
								secret.Data[apisv1alpha1.SecretKeyAPIExportIdentity] = []byte(expectedKey)
							}
						}
						return secret, nil
					}

					return nil, apierrors.NewNotFound(corev1.Resource("secrets"), name)
				},
				createSecret: func(ctx context.Context, clusterName logicalcluster.Path, secret *corev1.Secret) error {
					createSecretCalled = true
					return tc.createSecretError
				},
				listShards: func() ([]*corev1alpha1.Shard, error) {
					if tc.listShardsError != nil {
						return nil, tc.listShardsError
					}

					return []*corev1alpha1.Shard{
						{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									logicalcluster.AnnotationKey: "root:org:ws",
								},
								Name: "shard1",
							},
							Spec: corev1alpha1.ShardSpec{
								ExternalURL: "https://server-1.kcp.io/",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									logicalcluster.AnnotationKey: "root:org:ws",
								},
								Name: "shard2",
							},
							Spec: corev1alpha1.ShardSpec{
								ExternalURL: "https://server-2.kcp.io/",
							},
						},
					}, nil
				},
			}

			apiExport := &apisv1alpha2.APIExport{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:org:ws",
					},
					Name: "my-export",
				},
			}

			if tc.secretRefSet {
				apiExport.Spec.Identity = &apisv1alpha2.Identity{
					SecretRef: &corev1.SecretReference{
						Namespace: "somens",
						Name:      "somename",
					},
				}
			}

			if tc.apiExportHasSomeOtherHash {
				apiExport.Status.IdentityHash = "asdfasdfasdfasdf"
			}
			if tc.apiExportHasExpectedHash {
				apiExport.Status.IdentityHash = expectedHash
			}

			if tc.hasPreexistingVerifyFailure {
				conditions.MarkFalse(apiExport, apisv1alpha2.APIExportIdentityValid, apisv1alpha2.IdentityVerificationFailedReason, conditionsv1alpha1.ConditionSeverityError, "")
			}

			if tc.skipEndpointSliceAnnotation {
				apiExport.Annotations[apisv1alpha2.APIExportEndpointSliceSkipAnnotation] = "true"
			}

			err := c.reconcile(context.Background(), apiExport)
			if tc.wantError {
				require.Error(t, err, "expected an error")
			} else {
				require.NoError(t, err, "expected no error")
			}

			require.Equal(t, tc.wantCreateSecretCalled, createSecretCalled, "expected to try to create secret")

			if !tc.wantUnsetIdentity {
				if tc.wantDefaultSecretRef {
					require.Equal(t, "default-ns", apiExport.Spec.Identity.SecretRef.Namespace)
					require.Equal(t, apiExport.Name, apiExport.Spec.Identity.SecretRef.Name)
				} else {
					require.Equal(t, "somens", apiExport.Spec.Identity.SecretRef.Namespace)
					require.Equal(t, "somename", apiExport.Spec.Identity.SecretRef.Name)
				}
			}

			if tc.wantStatusHashSet {
				hashBytes := sha256.Sum256([]byte("abc"))
				hash := fmt.Sprintf("%x", hashBytes)
				require.Equal(t, hash, apiExport.Status.IdentityHash)
			}

			if tc.wantGenerationFailed {
				requireConditionMatches(t, apiExport,
					conditions.FalseCondition(
						apisv1alpha2.APIExportIdentityValid,
						apisv1alpha2.IdentityGenerationFailedReason,
						conditionsv1alpha1.ConditionSeverityError,
						"",
					),
				)
			}

			if tc.wantVerifyFailure {
				requireConditionMatches(t, apiExport,
					conditions.FalseCondition(
						apisv1alpha2.APIExportIdentityValid,
						apisv1alpha2.IdentityVerificationFailedReason,
						conditionsv1alpha1.ConditionSeverityError,
						"",
					),
				)
			}

			if tc.wantIdentityValid {
				requireConditionMatches(t, apiExport, conditions.TrueCondition(apisv1alpha2.APIExportIdentityValid))
			}

			require.Equal(t, tc.wantCreateAPIExportEndpointSlice, createEndpointSliceCalled, "expected createEndpointSliceCalled to be %v", tc.wantCreateAPIExportEndpointSlice)

			if tc.wantVirtualWorkspaceURLs {
				//nolint:staticcheck
				require.NotNil(t, apiExport.Status.VirtualWorkspaces, "expected virtual workspace URLs to be set")
				//nolint:staticcheck
				require.Len(t, apiExport.Status.VirtualWorkspaces, 2, "expected 2 virtual workspace URLs")
				require.True(t, conditions.IsTrue(apiExport, apisv1alpha2.APIExportVirtualWorkspaceURLsReady), "expected virtual workspace URLs to be ready")
			} else {
				//nolint:staticcheck
				require.Nil(t, apiExport.Status.VirtualWorkspaces, "expected virtual workspace URLs to be nil")
				require.False(t, conditions.Has(apiExport, apisv1alpha2.APIExportVirtualWorkspaceURLsReady), "expected virtual workspace URLs condition to not exist")
			}
		})
	}
}

// requireConditionMatches looks for a condition matching c in g. Only fields that are set in c are compared (Type is
// required, though). If c.Message is set, the test performed is contains rather than an exact match.
func requireConditionMatches(t *testing.T, g conditions.Getter, c *conditionsv1alpha1.Condition) {
	t.Helper()

	actual := conditions.Get(g, c.Type)

	require.NotNil(t, actual, "missing condition %q", c.Type)

	if c.Status != "" {
		require.Equal(t, c.Status, actual.Status)
	}

	if c.Severity != "" {
		require.Equal(t, c.Severity, actual.Severity)
	}

	if c.Reason != "" {
		require.Equal(t, c.Reason, actual.Reason)
	}

	if c.Message != "" {
		require.Contains(t, actual.Message, c.Message)
	}
}
