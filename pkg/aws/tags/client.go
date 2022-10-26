package tags

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/giantswarm/microerror"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws/assumerole"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type Client interface {
	Create(ctx context.Context, input CreateTagsInput) error
}

func NewClient(ec2Client *ec2.Client, assumeRoleClient assumerole.Client) (Client, error) {
	if ec2Client == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "ec2Client must not be empty")
	}
	if assumeRoleClient == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "assumeRoleClient must not be empty")
	}

	return &client{
		ec2Client:        ec2Client,
		assumeRoleClient: assumeRoleClient,
	}, nil
}

type client struct {
	ec2Client        *ec2.Client
	assumeRoleClient assumerole.Client
}
