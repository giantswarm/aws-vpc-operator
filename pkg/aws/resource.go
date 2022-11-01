package aws

import (
	"sigs.k8s.io/cluster-api/util/conditions"
)

type CloudResourceRequest[TResourceSpec any] struct {
	RoleARN string
	Region  string
	Spec    TResourceSpec
}

type CloudResourcesRequest[TResourceSpec any] struct {
	RoleARN string
	Region  string
	Specs   []TResourceSpec
}

type ReconcileRequest[TResourceSpec any] struct {
	CloudResourceRequest[TResourceSpec]

	// Resource that is being reconciled.
	Resource conditions.Setter

	ClusterName string
}

// ReconcileDeleteRequest is a generic request object for deleting a resource with ID.
type ReconcileDeleteRequest struct {
	CloudResourceRequest[DeletedCloudResource]

	// Resource that is being reconciled.
	Resource conditions.Setter

	ClusterName string
}

// ReconcileDeleteAllRequest is a generic request object for deleting resources with IDs.
type ReconcileDeleteAllRequest struct {
	CloudResourcesRequest[DeletedCloudResource]

	// Resource that is being reconciled.
	Resource conditions.Setter

	ClusterName string
}

type DeletedCloudResource struct {
	Id string
}
