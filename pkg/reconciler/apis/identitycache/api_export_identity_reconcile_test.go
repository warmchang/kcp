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

package identitycache

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kcp-dev/logicalcluster/v3"

	apisv1alpha2 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha2"
)

func TestReconcile(t *testing.T) {
	scenarios := []struct {
		name              string
		initialApiExports []*apisv1alpha2.APIExport
		initialConfigMap  *corev1.ConfigMap
		createConfigMap   func(ctx context.Context, cluster logicalcluster.Path, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error)
		updateConfigMap   func(ctx context.Context, cluster logicalcluster.Path, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error)
		validateCalls     func(t *testing.T, ctx callContext)
	}{
		{
			name: "scenario 1: happy path, cm doesn't exist",
			initialApiExports: []*apisv1alpha2.APIExport{
				newAPIExport("export-1"),
				newAPIExport("export-2"),
			},
			createConfigMap: func(ctx context.Context, cluster logicalcluster.Path, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
				requiredConfigMap := newEmptyRequiredConfigmap()
				requiredConfigMap.Data["export-1"] = "export-1-identity"
				requiredConfigMap.Data["export-2"] = "export-2-identity"

				// copy the annotations since the logicalcluster.AnnotationKey is added on the server side
				configMap.Annotations = requiredConfigMap.Annotations

				if !equality.Semantic.DeepEqual(configMap, requiredConfigMap) {
					return nil, fmt.Errorf("unexpected ConfigMap:\n%s", cmp.Diff(configMap, requiredConfigMap))
				}
				return nil, nil
			},
			validateCalls: func(t *testing.T, ctx callContext) {
				t.Helper()

				if !ctx.createConfigMap.called {
					t.Error("configmap never created")
				}
			},
		},
		{
			name: "scenario 2: no-op cm exist",
			initialApiExports: []*apisv1alpha2.APIExport{
				newAPIExport("export-1"),
				newAPIExport("export-2"),
			},
			initialConfigMap: func() *corev1.ConfigMap {
				requiredConfigMap := newEmptyRequiredConfigmap()
				requiredConfigMap.Data["export-1"] = "export-1-identity"
				requiredConfigMap.Data["export-2"] = "export-2-identity"
				return requiredConfigMap
			}(),
		},
		{
			name: "scenario 3: cm updated",
			initialApiExports: []*apisv1alpha2.APIExport{
				newAPIExport("export-1"),
				newAPIExport("export-2"),
			},
			initialConfigMap: func() *corev1.ConfigMap {
				requiredConfigMap := newEmptyRequiredConfigmap()
				requiredConfigMap.Data["export-1"] = "export-1-identity"
				return requiredConfigMap
			}(),
			updateConfigMap: func(ctx context.Context, cluster logicalcluster.Path, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
				requiredConfigMap := newEmptyRequiredConfigmap()
				requiredConfigMap.Data["export-1"] = "export-1-identity"
				requiredConfigMap.Data["export-2"] = "export-2-identity"

				// copy the annotations since the logicalcluster.AnnotationKey is added on the server side
				configMap.Annotations = requiredConfigMap.Annotations

				if !equality.Semantic.DeepEqual(configMap, requiredConfigMap) {
					return nil, fmt.Errorf("unexpected ConfigMap:\n%s", cmp.Diff(configMap, requiredConfigMap))
				}
				return nil, nil
			},
			validateCalls: func(t *testing.T, ctx callContext) {
				t.Helper()

				if !ctx.updateConfigMap.called {
					t.Error("configmap never updated")
				}
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			calls := callContext{
				createConfigMap: createConfigMapRecord{
					delegate: scenario.createConfigMap,
					defaulted: func(ctx context.Context, cluster logicalcluster.Path, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
						err := fmt.Errorf("unexpected create call for configmap %s|%s/%s", cluster, namespace, configMap.Name)
						t.Error(err)
						return nil, err
					},
				},
				getConfigMap: getConfigMapRecord{
					defaulted: func(cluster logicalcluster.Name, namespace, name string) (*corev1.ConfigMap, error) {
						if scenario.initialConfigMap == nil {
							return nil, errors.NewNotFound(corev1.Resource("configmaps"), name)
						}
						return scenario.initialConfigMap, nil
					},
				},
				updateConfigMap: updateConfigMapRecord{
					delegate: scenario.updateConfigMap,
					defaulted: func(ctx context.Context, cluster logicalcluster.Path, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
						err := fmt.Errorf("unexpected update call for configmap %s|%s/%s", cluster, namespace, configMap.Name)
						t.Error(err)
						return nil, err
					},
				},
				listGlobalAPIExports: listGlobalAPIExportsRecord{
					defaulted: func(name logicalcluster.Name) ([]*apisv1alpha2.APIExport, error) {
						return scenario.initialApiExports, nil
					},
				},
			}
			target := &controller{
				createConfigMap:      calls.createConfigMap.call,
				updateConfigMap:      calls.updateConfigMap.call,
				getConfigMap:         calls.getConfigMap.call,
				listGlobalAPIExports: calls.listGlobalAPIExports.call,
			}
			if err := target.reconcile(context.TODO()); err != nil {
				t.Error(err)
			}
			if scenario.validateCalls != nil {
				scenario.validateCalls(t, calls)
			}
		})
	}
}

type callContext struct {
	createConfigMap      createConfigMapRecord
	getConfigMap         getConfigMapRecord
	updateConfigMap      updateConfigMapRecord
	listGlobalAPIExports listGlobalAPIExportsRecord
}

type createConfigMapRecord struct {
	called              bool
	delegate, defaulted func(ctx context.Context, cluster logicalcluster.Path, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error)
}

func (r *createConfigMapRecord) call(ctx context.Context, cluster logicalcluster.Path, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	r.called = true
	delegate := r.delegate
	if delegate == nil {
		delegate = r.defaulted
	}
	return delegate(ctx, cluster, namespace, configMap)
}

type getConfigMapRecord struct {
	called              bool
	delegate, defaulted func(cluster logicalcluster.Name, namespace, name string) (*corev1.ConfigMap, error)
}

func (r *getConfigMapRecord) call(cluster logicalcluster.Name, namespace, name string) (*corev1.ConfigMap, error) {
	r.called = true
	delegate := r.delegate
	if delegate == nil {
		delegate = r.defaulted
	}
	return delegate(cluster, namespace, name)
}

type updateConfigMapRecord struct {
	called              bool
	delegate, defaulted func(ctx context.Context, cluster logicalcluster.Path, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error)
}

func (r *updateConfigMapRecord) call(ctx context.Context, cluster logicalcluster.Path, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	r.called = true
	delegate := r.delegate
	if delegate == nil {
		delegate = r.defaulted
	}
	return delegate(ctx, cluster, namespace, configMap)
}

type listGlobalAPIExportsRecord struct {
	called              bool
	delegate, defaulted func(cluster logicalcluster.Name) ([]*apisv1alpha2.APIExport, error)
}

func (r *listGlobalAPIExportsRecord) call(cluster logicalcluster.Name) ([]*apisv1alpha2.APIExport, error) {
	r.called = true
	delegate := r.delegate
	if delegate == nil {
		delegate = r.defaulted
	}
	return delegate(cluster)
}

func newAPIExport(name string) *apisv1alpha2.APIExport {
	return &apisv1alpha2.APIExport{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				logicalcluster.AnnotationKey: "root",
			},
			Name: name,
		},
		Status: apisv1alpha2.APIExportStatus{
			IdentityHash: fmt.Sprintf("%s-identity", name),
		},
	}
}

func newEmptyRequiredConfigmap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				logicalcluster.AnnotationKey: "system:shard",
			},
			Namespace: "default",
			Name:      ConfigMapName,
		},
		Data: map[string]string{},
	}
}
