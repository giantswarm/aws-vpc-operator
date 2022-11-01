package subnets

import (
	"context"

	"github.com/giantswarm/microerror"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

// ReconcileDeleteRequest specified which resource is being deleted and what is
// the specification of the subnets that should be deleted.
type ReconcileDeleteRequest struct {
	// Resource that is being reconciled.
	Resource conditions.Setter

	// Spec of the desired subnets.
	Spec DeletedSpec
}

type DeletedSpec struct {
	ClusterName string
	RoleARN     string
	Region      string
	Subnets     []DeletedSubnetSpec
}

type DeletedSubnetSpec struct {
	SubnetId string
}

func (r *reconciler) ReconcileDelete(ctx context.Context, request ReconcileDeleteRequest) (err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started reconciling subnets deletion")
	defer func() {
		if err == nil {
			logger.Info("Finished reconciling subnets deletion")
		} else {
			logger.Error(err, "Failed to reconcile subnets deletion")
		}
	}()

	spec := request.Spec

	if spec.ClusterName == "" {
		return microerror.Maskf(errors.InvalidConfigError, "ClusterName must not be empty")
	}
	if spec.RoleARN == "" {
		return microerror.Maskf(errors.InvalidConfigError, "RoleARN must not be empty")
	}
	if spec.Region == "" {
		return microerror.Maskf(errors.InvalidConfigError, "Region must not be empty")
	}
	if len(spec.Subnets) == 0 {
		return microerror.Maskf(errors.InvalidConfigError, "Subnets must not be empty")
	}

	deleteSubnetsInput := DeleteSubnetsInput{
		RoleARN: request.Spec.RoleARN,
		Region:  request.Spec.Region,
	}
	for _, subnetSpec := range request.Spec.Subnets {
		deleteSubnetsInput.SubnetIds = append(deleteSubnetsInput.SubnetIds, subnetSpec.SubnetId)
	}
	err = r.client.Delete(ctx, deleteSubnetsInput)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}
