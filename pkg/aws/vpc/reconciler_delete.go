package vpc

import (
	"context"

	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

func (r *reconciler) ReconcileDelete(ctx context.Context, request aws.ReconcileRequest[aws.DeletedCloudResourceSpec]) (err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started reconciling VPC deletion")
	defer func() {
		if err == nil {
			logger.Info("Finished reconciling VPC deletion")
		} else {
			logger.Error(err, "Failed to reconcile VPC deletion")
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
	if request.Spec.Id == "" {
		return microerror.Maskf(errors.InvalidConfigError, "Spec.Id must not be empty")
	}

	deleteVpcInput := DeleteVpcInput{
		RoleARN: request.RoleARN,
		Region:  request.Region,
		VpcId:   request.Spec.Id,
	}
	err = r.client.Delete(ctx, deleteVpcInput)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}
