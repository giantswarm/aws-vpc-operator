package routetables

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/giantswarm/microerror"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws/assumerole"
	"github.com/giantswarm/aws-vpc-operator/pkg/aws/tags"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type Client interface {
	Create(ctx context.Context, input CreateRouteTableInput) (CreateRouteTableOutput, error)
	Update(ctx context.Context, input UpdateRouteTableInput) error
	Get(ctx context.Context, input GetRouteTableInput) (RouteTableOutput, error)
	List(ctx context.Context, input ListRouteTablesInput) (ListRouteTablesOutput, error)
	Delete(ctx context.Context, input DeleteRouteTableInput) error
	DeleteAll(ctx context.Context, input DeleteRouteTablesInput) error
}

func NewClient(ec2Client *ec2.Client, assumeRoleClient assumerole.Client) (Client, error) {
	if ec2Client == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "ec2Client must not be empty")
	}
	if assumeRoleClient == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "assumeRoleClient must not be empty")
	}

	tagsClient, err := tags.NewClient(ec2Client, assumeRoleClient)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return &client{
		ec2Client:        ec2Client,
		assumeRoleClient: assumeRoleClient,
		tagsClient:       tagsClient,
	}, nil
}

type client struct {
	ec2Client        *ec2.Client
	tagsClient       tags.Client
	assumeRoleClient assumerole.Client
}
