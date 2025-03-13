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


// Code generated by kcp code-generator. DO NOT EDIT.

package v1alpha1

import (
	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"	
	"github.com/kcp-dev/logicalcluster/v3"
	
	"k8s.io/client-go/tools/cache"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/api/errors"

	tenancyv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
	)

// WorkspaceClusterLister can list Workspaces across all workspaces, or scope down to a WorkspaceLister for one workspace.
// All objects returned here must be treated as read-only.
type WorkspaceClusterLister interface {
	// List lists all Workspaces in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*tenancyv1alpha1.Workspace, err error)
	// Cluster returns a lister that can list and get Workspaces in one workspace.
Cluster(clusterName logicalcluster.Name) WorkspaceLister
WorkspaceClusterListerExpansion
}

type workspaceClusterLister struct {
	indexer cache.Indexer
}

// NewWorkspaceClusterLister returns a new WorkspaceClusterLister.
// We assume that the indexer:
// - is fed by a cross-workspace LIST+WATCH
// - uses kcpcache.MetaClusterNamespaceKeyFunc as the key function
// - has the kcpcache.ClusterIndex as an index
func NewWorkspaceClusterLister(indexer cache.Indexer) *workspaceClusterLister {
	return &workspaceClusterLister{indexer: indexer}
}

// List lists all Workspaces in the indexer across all workspaces.
func (s *workspaceClusterLister) List(selector labels.Selector) (ret []*tenancyv1alpha1.Workspace, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*tenancyv1alpha1.Workspace))
	})
	return ret, err
}

// Cluster scopes the lister to one workspace, allowing users to list and get Workspaces.
func (s *workspaceClusterLister) Cluster(clusterName logicalcluster.Name) WorkspaceLister {
return &workspaceLister{indexer: s.indexer, clusterName: clusterName}
}

// WorkspaceLister can list all Workspaces, or get one in particular.
// All objects returned here must be treated as read-only.
type WorkspaceLister interface {
	// List lists all Workspaces in the workspace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*tenancyv1alpha1.Workspace, err error)
// Get retrieves the Workspace from the indexer for a given workspace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*tenancyv1alpha1.Workspace, error)
WorkspaceListerExpansion
}
// workspaceLister can list all Workspaces inside a workspace.
type workspaceLister struct {
	indexer cache.Indexer
	clusterName logicalcluster.Name
}

// List lists all Workspaces in the indexer for a workspace.
func (s *workspaceLister) List(selector labels.Selector) (ret []*tenancyv1alpha1.Workspace, err error) {
	err = kcpcache.ListAllByCluster(s.indexer, s.clusterName, selector, func(i interface{}) {
		ret = append(ret, i.(*tenancyv1alpha1.Workspace))
	})
	return ret, err
}

// Get retrieves the Workspace from the indexer for a given workspace and name.
func (s *workspaceLister) Get(name string) (*tenancyv1alpha1.Workspace, error) {
	key := kcpcache.ToClusterAwareKey(s.clusterName.String(), "", name)
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(tenancyv1alpha1.Resource("workspaces"), name)
	}
	return obj.(*tenancyv1alpha1.Workspace), nil
}
// NewWorkspaceLister returns a new WorkspaceLister.
// We assume that the indexer:
// - is fed by a workspace-scoped LIST+WATCH
// - uses cache.MetaNamespaceKeyFunc as the key function
func NewWorkspaceLister(indexer cache.Indexer) *workspaceScopedLister {
	return &workspaceScopedLister{indexer: indexer}
}

// workspaceScopedLister can list all Workspaces inside a workspace.
type workspaceScopedLister struct {
	indexer cache.Indexer
}

// List lists all Workspaces in the indexer for a workspace.
func (s *workspaceScopedLister) List(selector labels.Selector) (ret []*tenancyv1alpha1.Workspace, err error) {
	err = cache.ListAll(s.indexer, selector, func(i interface{}) {
		ret = append(ret, i.(*tenancyv1alpha1.Workspace))
	})
	return ret, err
}

// Get retrieves the Workspace from the indexer for a given workspace and name.
func (s *workspaceScopedLister) Get(name string) (*tenancyv1alpha1.Workspace, error) {
	key := name
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(tenancyv1alpha1.Resource("workspaces"), name)
	}
	return obj.(*tenancyv1alpha1.Workspace), nil
}
