package subnets

import (
	"context"

	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

func (r *reconciler) ReconcileDelete(ctx context.Context, request aws.ReconcileRequest[[]aws.DeletedCloudResourceSpec]) (err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started reconciling subnets deletion")
	defer func() {
		if err == nil {
			logger.Info("Finished reconciling subnets deletion")
		} else {
			logger.Error(err, "Failed to reconcile subnets deletion")
		}
	}()

	if request.ClusterName == "" {
		return microerror.Maskf(errors.InvalidConfigError, "ClusterName must not be empty")
	}
	if request.RoleARN == "" {
		return microerror.Maskf(errors.InvalidConfigError, "RoleARN must not be empty")
	}
	if request.Region == "" {
		return microerror.Maskf(errors.InvalidConfigError, "Region must not be empty")
	}
	if len(request.Spec) == 0 {
		return microerror.Maskf(errors.InvalidConfigError, "Specs must not be empty")
	}

	deleteSubnetsInput := DeleteSubnetsInput{
		RoleARN: request.RoleARN,
		Region:  request.Region,
	}
	for _, spec := range request.Spec {
		deleteSubnetsInput.SubnetIds = append(deleteSubnetsInput.SubnetIds, spec.Id)
	}
	err = r.client.Delete(ctx, deleteSubnetsInput)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}
