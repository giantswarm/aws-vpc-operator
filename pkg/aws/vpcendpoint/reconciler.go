package vpcendpoint

import (
	"context"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type Reconciler interface {
	Reconcile(ctx context.Context, request aws.ReconcileRequest[Spec]) (aws.ReconcileResult[Status], error)
	ReconcileDelete(ctx context.Context, request aws.ReconcileRequest[aws.DeletedCloudResourceSpec]) error
}

func NewReconciler(client Client) (Reconciler, error) {
	if client == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "client must not be empty")
	}

	return &reconciler{
		client: client,
	}, nil
}

type reconciler struct {
	client Client
}
