package routetables

import (
	"context"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type Reconciler interface {
	Reconcile(ctx context.Context, request aws.ReconcileRequest[Spec]) (aws.ReconcileResult[[]Status], error)
	ReconcileDelete(ctx context.Context, request aws.ReconcileRequest[[]aws.DeletedCloudResourceSpec]) error
}

type Spec struct {
	// VpcId is the ID of the VPC for which we want route tables.
	VpcId string

	// Subnets for which we want route tables. We create one route table per
	// subnet.
	Subnets []Subnet
}

type Subnet struct {
	Id               string
	AvailabilityZone string
}

type Status struct {
	RouteTableId          string
	RouteTableAssociation []RouteTableAssociation
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
