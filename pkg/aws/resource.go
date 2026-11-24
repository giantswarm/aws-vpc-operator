package aws

import (
	"sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
)

type ReconcileRequest[TResourceSpec any] struct {
	CloudResourceRequest[TResourceSpec]

	// Resource that is being reconciled.
	Resource conditions.Setter

	ClusterName string
}

type CloudResourceRequest[TResourceSpec any] struct {
	RoleARN        string
	Region         string
	Spec           TResourceSpec
	AdditionalTags map[string]string
}

type DeletedCloudResourceSpec struct {
	Id string
}

type ReconcileResult[TResourceStatus any] struct {
	Status TResourceStatus
}
