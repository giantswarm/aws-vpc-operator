package routetables

import (
	"context"

	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

func (r *reconciler) ReconcileDelete(ctx context.Context, request aws.ReconcileRequest[aws.DeletedCloudResourceSpec]) (err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started reconciling route tables deletion")
	defer func() {
		if err == nil {
			logger.Info("Finished reconciling route tables deletion")
		} else {
			logger.Error(err, "Failed to reconcile route tables deletion")
		}
	}()

	if request.ClusterName == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.ClusterName must not be empty", request)
	}
	if request.RoleARN == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", request)
	}
	if request.Region == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", request)
	}
	if request.Spec.Id == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.Spec.Id must not be empty", request)
	}

	// Delete all route tables for the specified VPC
	input := DeleteRouteTablesInput{
		RoleARN: request.RoleARN,
		Region:  request.Region,
		VpcId:   request.Spec.Id,
	}
	err = r.client.DeleteAll(ctx, input)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}
