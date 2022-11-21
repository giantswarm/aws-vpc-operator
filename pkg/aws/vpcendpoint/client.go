package vpcendpoint

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/giantswarm/microerror"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws/assumerole"
	"github.com/giantswarm/aws-vpc-operator/pkg/aws/tags"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

const (
	StateAvailable = string(ec2Types.StateAvailable)
	StateDeleting  = string(ec2Types.StateDeleting)
	StateDeleted   = string(ec2Types.StateDeleted)
)

type Client interface {
	Create(ctx context.Context, input CreateVpcEndpointInput) (CreateVpcEndpointOutput, error)
	Get(ctx context.Context, input GetVpcEndpointInput) (GetVpcEndpointOutput, error)
	Update(ctx context.Context, input UpdateVpcEndpointInput) error
	Delete(ctx context.Context, input DeleteVpcEndpointInput) error
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

func secretsManagerServiceName(region string) string {
	return fmt.Sprintf("com.amazonaws.%s.secretsmanager", region)
}
