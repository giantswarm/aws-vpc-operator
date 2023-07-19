package vpcendpoint

import (
	"context"

	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

func (r *reconciler) ReconcileDelete(ctx context.Context, request aws.ReconcileRequest[aws.DeletedCloudResourceSpec]) (err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started reconciling VPC endpoint deletion")
	defer func() {
		if err == nil {
			logger.Info("Finished reconciling VPC endpoint deletion")
		} else {
			logger.Error(err, "Failed to reconcile VPC endpoint deletion")
		}
	}()

	if !shouldReconcileVpcEndpoint(request.Resource.GetAnnotations()) {
		return nil
	}

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

	// s3 endpoint
	{
		input := DeleteVpcEndpointInput{
			RoleARN:     request.RoleARN,
			Region:      request.Region,
			VpcId:       request.Spec.Id,
			Type:        ec2Types.VpcEndpointTypeGateway,
			ServiceName: s3ServiceName(request.Region),
		}
		err = r.client.Delete(ctx, input)
		if errors.IsVpcEndpointNotFound(err) {
			logger.Info("Nothing to delete, VPC endpoint not found")
		} else if errors.IsResourceAlreadyDeleted(err) {
			logger.Info("Nothing to delete, VPC endpoint already deleted")
		} else if errors.IsResourceDeletionInProgress(err) {
			logger.Info("VPC endpoint deletion is already in progress")
		} else if err != nil {
			return microerror.Mask(err)
		}
	}

	return nil
}
