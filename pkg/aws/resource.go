package aws

import (
	"sigs.k8s.io/cluster-api/util/conditions"
)

type ReconcileRequest[TResourceSpec any] struct {
	CloudResourceRequest[TResourceSpec]

	// Resource that is being reconciled.
	Resource conditions.Setter

	ClusterName string
}

type CloudResourceRequest[TResourceSpec any] struct {
	RoleARN string
	Region  string
	Spec    TResourceSpec
}

type DeletedCloudResourceSpec struct {
	Id string
}
