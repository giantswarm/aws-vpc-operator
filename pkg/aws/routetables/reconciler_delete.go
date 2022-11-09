package routetables

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws"
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

	return nil
}
