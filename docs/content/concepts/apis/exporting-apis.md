# Exporting and Binding APIs

If you're looking to provide APIs that can be consumed by multiple workspaces, this section is for you!

kcp adds new APIs that enable this new model of publishing and consuming APIs. The following diagram shows a workspace
that an API provider would use to export their `widgets` API. The provider uses kcp's new `APIResourceSchema` type to
define the schema for `widgets`, and the new `APIExport` type to export `widgets` to other workspaces.

2 separate consumer workspaces use kcp's new `APIBinding` type to add the `widgets` API to their workspaces. They can
then proceed to CRUD `widgets` in their workspaces, just like any other API type (e.g. `namespaces`, `configmaps`,
etc.).

```mermaid
flowchart TD
    subgraph provider["API Provider Workspace"]
        export["Widgets APIExport"]
        schema["Widgets APIResourceSchema"]
        export --> schema
    end

    subgraph consumer1["Consumer Workspace"]
        binding1["Widgets APIBinding"]
        widgetsA1["Widget A"]
        widgetsB1["Widget B"]
        widgetsC1["Widget C"]
        binding1 --> widgetsA1
        binding1 --> widgetsB1
        binding1 --> widgetsC1
    end

    subgraph consumer2["Consumer Workspace"]
        binding2["Widgets APIBinding"]
        widgetsA2["Widget A"]
        widgetsB2["Widget B"]
        widgetsC2["Widget C"]
        binding2 --> widgetsA2
        binding2 --> widgetsB2
        binding2 --> widgetsC2
    end

    export --> binding1
    export --> binding2

    classDef state color:#F77
    classDef or fill:none,stroke:none
    class export,schema,binding1,binding2 resource;
```

**Diagram: 1 APIExport consumed by 2 different workspaces** ([source][diagram1])

Above we talked about needing 1 controller instance per workspace, as workspace is roughly synonymous with cluster, and
most/all controllers out there currently only support a single cluster. But because multiple workspaces coexist in kcp,
we can be much more efficient and have 1 controller handle `widgets` in multiple workspaces!

We achieve this using a specific URL just for your controller for your `APIExport`.

You'll need to do a few things:

1. Define 1 or more `APIResourceSchema` objects, 1 per API resource (just like you'd do with CRDs).
2. Define an `APIExport` object that references all the `APIResourceSchema` instances you want to export.
3. Write controllers that are multi-workspace aware.

Let's look at each of these in more detail.

## Define APIResourceSchemas

An `APIResourceSchema` defines a single custom API type. It is almost identical to a CRD, but creating
an `APIResourceSchema` instance does not add a usable API to the server. By intentionally decoupling the schema
definition from serving, API owners can be more explicit about API evolution. In the future, we could envision the
possibility of a new `APIDeployment` resource that coordinates rolling out API updates.

Here is an example for a `widgets` resource:

```yaml
apiVersion: apis.kcp.io/v1alpha1
kind: APIResourceSchema
metadata:
  name: v220801.widgets.example.kcp.io # (1)
spec:
  group: example.kcp.io
  names:
    categories:
    - kcp
    kind: Widget
    listKind: WidgetList
    plural: widgets
    singular: widget
  scope: Cluster
  versions:
  - additionalPrinterColumns:
    - description: The current phase
      jsonPath: .status.phase
      name: Phase
      type: string
    - jsonPath: .metadata.creationTimestamp
      name: Age
      type: date
    name: v1alpha1
    schema:
      description: 'NOTE: full schema omitted for brevity'
      type: object
    served: true
    storage: true
    subresources:
      status: {}
```

1. `name` must be of the format `<some prefix>.<plural resource name>.<group name>`. For this example:
       - `<some prefix>` is `v220801`
       - `<plural resource name>` is `widgets`
       - `<group name>` is `example.kcp.io`

An `APIResourceSchema`'s `spec` is immutable; if you need to make changes to your API schema, you create a new instance.

Once you've created at least one `APIResourceSchema`, you can proceed with creating your `APIExport`.

## Define Your APIExport

An `APIExport` is the way an API provider makes one or more APIs (coming from `APIResourceSchemas`) available to
workspaces.

Here is an example `APIExport` called `example.kcp.io` that exports 1 resource: `widgets`.

```yaml
apiVersion: apis.kcp.io/v1alpha2
kind: APIExport
metadata:
  name: example.kcp.io
spec:
  resources:
    - group: example.kcp.io
      name: widgets
      schema: v220801.widgets.example.kcp.io
      storage:
        crd: {}
```

At a minimum, you specify the resource group, resource name, name of the `APIResourceSchema` and storage type in `spec.resources`. `APIResourceSchemas` must be in the same workspace as the `APIExport` (and therefore no workspace name or path is required here).

You can optionally configure the following additional aspects of an `APIExport`:

- its identity
- what permissions consumers of your exported APIs are granted
- API resources from other sources (built-in types and/or from other `APIExport`s) that your controllers need to access
  for your service to function correctly

We'll talk about each of these next.

### APIExport Identity

Each API resource type is defined by an API group name and a resource name. Each API resource type can further be
distinguished by an API version. For example, the `roles.rbac.authorization.k8s.io` resource that we frequently interact
with is in the `rbac.authorization.k8s.io` API group and its resource name is `roles`.

In a Kubernetes cluster, it is impossible to define the same API `<group>,<resource>,<version>` multiple times; i.e., it
does not support competing definitions for `rbac.authorization.k8s.io,roles,v1` or any other API resource type.

In kcp, however, it is possible for multiple API providers to each define the same API `<group>,<resource>,<version>`,
_without interference_! This is where `APIExport` "identity" becomes critical.

When you create an `APIExport` and you don't specify its identity (as is the case in the `example.kcp.io` example
above), kcp automatically generates one for you. The identity is always stored in a secret in the same workspace as
the `APIExport.` By default, it is created in the `kcp-system` namespace in a secret whose name is the name of
the `APIExport`.

An `APIExport`'s identity is similar to a private key; you should never share it with anyone.

The identity's **hash** is similar to a public key; it is not private and there are times when other consumers of your
exported APIs need to know and reference it.

Given 2 Workspaces, each with its own `APIExport` that exports `widgets.example.kcp.io`, kcp uses the identity hash (in
a mostly transparent manner) to ensure the correct instances associated with the appropriate `APIResourceSchema` are
served to clients. See [Build Your Controller](#build-your-controller) for more information.

### Permission Claims

When a consumer creates an `APIBinding` that binds to an `APIExport`, the API provider who owns the `APIExport`
implicitly has access to instances of the exported APIs in the consuming workspace. There are also times when the API
provider needs to access additional resource data in a consuming workspace. These resources might come from other
APIExports the consumer has created `APIBindings` for, or from APIs that are built in to kcp. The API provider
requests access to these additional resources by adding `PermissionClaims` for the desired API's group, resource, and
identity hash to their `APIExport`. The API provider is responsible for specifying what operations are required for the
requested resource by setting the appropriate [API verbs](https://kubernetes.io/docs/reference/using-api/api-concepts/#api-verbs).
Let's take the example `APIExport` from above and add permission claims for `ConfigMaps` and `Things`:

```yaml
apiVersion: apis.kcp.io/v1alpha2
kind: APIExport
metadata:
  name: example.kcp.io
spec:
  resources:
    - group: example.kcp.io
      name: widgets
      schema: v220801.widgets.example.kcp.io
      storage:
        crd: {}
  permissionClaims:
  - group: "" # (1)
    resource: configmaps
    verbs: ["get", "list", "create"] # (3)
  - group: somegroup.kcp.io
    resource: things
    identityHash: 5fdf7c7aaf407fd1594566869803f565bb84d22156cef5c445d2ee13ac2cfca6 # (2)
    verbs: ["*"] # (4)
```

1. This is how you specify the core API group
2. To claim another exported API, you must include its `identityHash`
3. Verbs is a list of the [API verbs](https://kubernetes.io/docs/reference/using-api/api-concepts/#api-verbs)
4. `"*"` is a special "verb" that matches any possible verb

This is essentially a request from the API provider, asking each consumer to grant permission for the claimed
resources. If the consumer does not accept a permission claim, the API provider is not allowed to access the claimed
resources. Consumer acceptance of permission claims is part of the `APIBinding` spec. The operations allowed on the
resource are the intersection of the verbs defined in the `APIExport` and the verbs accepted in the appropriate
`APIBinding`. For more details, see the section on [APIBindings](#apibinding).

### Maximal Permission Policy

If you want to set an upper bound on what is allowed for a consumer of your exported APIs. you can set a "maximal
permission policy" using standard RBAC resources. This is optional; if the policy is not set, no upper bound is applied,
and a consuming user is authorized based on the RBAC configuration in the consuming workspace.

The maximal permission policy consists of RBAC `(Cluster)Roles` and `(Cluster)RoleBindings`. Incoming requests to a
workspace that binds to an APIExport are additionally checked against
these rules, with the username and groups prefixed with `apis.kcp.io:binding:`.

For example: we have an `APIExport` in the `root` workspace called `tenancy.kcp.io` that provides APIs
for `workspaces` and `workspacetypes`:

```yaml
apiVersion: apis.kcp.io/v1alpha2
kind: APIExport
metadata:
  name: tenancy.kcp.io
spec:
  resources:
  - group: tenancy.kcp.io
    name: workspaces
    schema: v250421-25d98218b.workspaces.tenancy.kcp.io
    storage:
      crd: {}
  - group: tenancy.kcp.io
    name: workspacetypes
    schema: v250603-d4d365c8e.workspacetypes.tenancy.kcp.io
    storage:
      crd: {}
  maximalPermissionPolicy:
    local: {} # (1)
```

1. `local` is the only supported option at the moment. "Local" means the RBAC policy is defined in the same
   workspace as the `APIExport`.

We don't want users to be able to mutate the `status` subresource, so we set up
a maximal permission policy to limit what users can do:

```yaml title="Tenancy APIExport Maximal Permission Policy ClusterRole"
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system:kcp:apiexport:tenancy:maximal-permission-policy
rules:
- apiGroups: ["tenancy.kcp.io"]
  verbs: ["*"] # (1)
  resources:
  - workspaces
  - workspacetypes
- apiGroups: ["tenancy.kcp.io"]
  verbs: ["list","watch","get"] # (2)
  resources:
  - workspaces/status
  - workspacetypes/status
```

1. Users can perform any/all actions on the main resources
2. Users can only get/list/watch the status subresources

```yaml title="Tenancy APIExport Maximal Permission Policy ClusterRoleBinding"
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:kcp:authenticated:apiexport:tenancy:maximal-permission-policy
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:kcp:apiexport:tenancy:maximal-permission-policy
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: apis.kcp.io:binding:system:authenticated # (1)
```

1. Note the `apis.kcp.io:binding:` prefix; this identifies this `ClusterRoleBinding` as part of the maximal
   permission policy. It applies to `system:authenticated`.

Now imagine a user named `unicorn` with group `system:authenticated`. They create a workspace named
`magic` and bind to the `tenancy.kcp.io` APIExport from the workspace `root`. What actions `unicorn` is allowed to
perform in the `magic` workspace must be granted by **both**:

1. the standard RBAC authorization flow in the `magic` workspace; i.e., `(Cluster)Roles` and `(Cluster)RoleBindings`
   in the `magic` workspace itself, **and**
2. the maximal permission policy RBAC settings configured in the `root` workspace for the `tenancy` APIExport

## Build Your Controller

Controllers to reconcile resources backed by `APIExports` can be developed with kcp's [controller-runtime fork](https://github.com/kcp-dev/controller-runtime). The fork follows upstream and allows to write both kcp-aware and vanilla Kubernetes controllers at the same time. There is an [example controller](https://github.com/kcp-dev/controller-runtime/tree/kcp-0.18/examples/kcp) that serves as reference for implementations.

When reconciling exported APIs, controllers usually interact with the API resources of said `APIExport` through a [virtual workspace](../workspaces/virtual-workspaces.md). Because kcp can be sharded, it is possible that there are multiple virtual workspaces (across the different shards) that show a subset of API resources created across a sharded kcp instance.

### Endpoint Slices

!!! information "Changed with kcp v0.28"
    Previous versions of kcp did not automatically create an `APIExportEndpointSlice`. Check their respective versioned documentation for details.

Since kcp is a sharded system, global access to the resources exposed via an `APIExport` is not trivial and can mean that a service provider has to connect to multiple shard endpoints to get the "full picture". This behaviour of kcp is reflected in `APIExportEndpointSlices`. They return a list of endpoints to connect to for a particular `APIExport`. kcp creates one global `APIExportEndpointSlice` per `APIExport` automatically. So unless a certain partition of the kcp installation should be targeted, it is not required to create one manually. If you wish to disable this behaviour, label your `APIExport` with `apiexports.apis.kcp.io/skip-endpointslice=true`.

An API provider that implements a controller can optionally also create an `APIExportEndpointSlice` themselves. This will "slice and dice" the list of available virtual workspace endpoints based on [Partitions](../sharding/partitions.md). This is what it looks like:

```yaml title="APIExportEndpointSlice for a partition's virtual workspace endpoints"
kind: APIExportEndpointSlice
apiVersion: apis.kcp.io/v1alpha1
metadata:
    name: example-gcp-europe
spec:
    export:
        path: root # (1)
        name: example.kcp.io
    # optional
    partition: cloud-region-gcp-europe-xdfgs # (2)
```

1. The workspace path at which the `APIExport` sits. This can be in a different workspace than the `APIExportEndpointSlice`.
2. The `Partition` object targeted by this endpoint slice.

Based on this, only virtual workspace URLs for the `cloud-region-gcp-europe-xdfgs` partition will be populated into this endpoint slice. If you wish to get a full list of virtual workspace endpoints, just omit the `spec.partition` field from the example above.

Once created, the `status` field of the `APIExportEndpointSlice` will be populated according to its specification:

```shell
$ kubectl get apiexportendpointslice example-gcp-europe -o yaml
kind: APIExportEndpointSlice
apiVersion: apis.kcp.io/v1alpha1
metadata:
    name: example-gcp-europe
...
status:
    endpoints
        - url: https://host1:6443/services/apiexport/root/example.kcp.io
        - url: https://host2:6443/services/apiexport/root/example.kcp.io
...
```

Controller implementations need to watch the `APIExportEndpointSlice` and use all endpoints provided by the status. Each virtual workspace endpoint is its own Kubernetes-like API endpoint. This means that it's likely necessary to spin up managers that each "own" one of the endpoints and process them.

### Authorization

Controllers need to run with proper authorization. That means they need to be able to access the `APIExportEndpointSlice` above (read-only access is sufficient). The `ClusterRole` in the workspace hosting the `APIExportEndpointSlice` could look like this:

```yaml title="ClusterRole for APIExportEndpointSlice access"
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: example.kcp.io:access-endpointslice
rules:
- apiGroups: ["apis.kcp.io"]
  verbs: ["list", "watch", "get"]
  resources:
  - apiexportendpointslices
```

In addition, the user used by the controller needs to be authorized to access the content of the `APIExport` virtual workspace. For that, authorization needs to be given for the `content` subresource of the `APIExport` resource (which might exist in a different workspace than the `APIExportEndpointSlice`). A suitable `ClusterRole` would look like this:

```yaml title="ClusterRole for APIExport content access"
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: example.kcp.io:access-content
rules:
- apiGroups: ["apis.kcp.io"]
  verbs: ["*"] # (1)
  resources:
  - apiexports/content
  resourceNames:
  - example.kcp.io
```

1. Allows read and write access to resources created from this `APIExport` across all workspaces

<!--

## APIResourceSchema Evolution & Maintenance

TODO
- conversions
- doc when it's ok to delete "old"/no longer used APIResourceSchemas

-->

## Bind to Exported APIs

### APIBinding

`APIBindings` are used to import API resources. They contain a reference to an `APIExport` using the name and kcp workspace path of an `APIExport` and will bind all APIs defined in the `APIExport` to your workspace. The reference path needs to be provided to you by the provider of the API or an external cataloging solution. Alternatively the provider of the api could give you read permissions on the `APIExport` in their workspace.

Returning to our previous example, we can use the following `APIBinding` to import the widgets api.

```yaml
apiVersion: apis.kcp.io/v1alpha2
kind: APIBinding
metadata:
  name: example.kcp.io
spec:
  reference:
    export:
      name: example.kcp.io
      path: "root:api-provider" # path of your api-provider workspace
```

#### Permission Claims

Furthermore, `APIBindings` provide the `APIExport` owner access to additional resources defined in an `APIExport`'s permission claims list. Permission claims must be accepted by the user explicitly, before this access is granted. The resources can be builtin Kubernetes resources or resources from other `APIExports`.
When an `APIExport` is changed after workspaces have bound to it, new or changed APIs are automatically propagated to all `APIBindings`. New permission claims on the other hand are NOT automatically accepted.

Returning to our example, we can grant the requested permissions in the `APIBinding`:

```yaml
apiVersion: apis.kcp.io/v1alpha2
kind: APIBinding
metadata:
  name: example.kcp.io
spec:
  reference:
    export:
      name: example.kcp.io
      path: "root:api-provider" # path of your api-provider workspace
  permissionClaims:
  - resource: configmaps
    verbs: ["get", "list", "create"]
    state: Accepted
    selector:
      matchAll: true
  - resource: things
    group: somegroup.kcp.io
    identityHash: 5fdf7c7aaf407fd1594566869803f565bb84d22156cef5c445d2ee13ac2cfca6
    verbs: ["*"]
    state: Accepted
    selector:
      matchAll: true
```

It should be noted that `APIBindings` do not create `CRDs` or `APIResourceSchemas`in the workspace. Instead APIs are directly bound using Kubernetes' internal binding mechanism behind the scenes.

##### Verbs

Operations allowed on the resources for which permission claims are accepted is defined as the intersection of
the verbs in the APIBinding and the verbs in the appropriate APIExport.

##### Selector

`APIBindings` allow API consumers to scope an API provider's access to claimed resources via the `selector` field on a permission claim. This means that providers will only be able to see and access those objects matched by the `selector`.

!!! information
    Currently, only `selector.matchAll=true` is supported, giving the provider that owns the `APIExport` full access to all objects of a claimed resource. Additional selectors are planned for upcoming releases.

---

In practice, bound APIs behave similarly to other resources in kcp or Kubernetes. This means you can query for imported APIs using `kubectl api-resources`. Additionally you can use `kubectl explain` to get a detailed view on all fields of the API.

```sh
# inside consumer workspace
$ kubectl api-resources --api-group='example.kcp.io'

NAME      SHORTNAMES   APIVERSION                NAMESPACED   KIND
widgets                example.kcp.io/v1alpha1   false        Widget
```

Furthermore, you can use the `.status.boundResources` field to precisely identify which `APIResourceSchemas` have been imported.
