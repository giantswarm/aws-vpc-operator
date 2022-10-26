package vpc

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/giantswarm/microerror"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws/assumerole"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type VpcState string

// Enum values for VpcState
const (
	VpcStatePending   VpcState = "pending"
	VpcStateAvailable VpcState = "available"
)

type Client interface {
	Create(ctx context.Context, input CreateVpcInput) (CreateVpcOutput, error)
	Get(ctx context.Context, input GetVpcInput) (GetVpcOutput, error)
	Delete(ctx context.Context, input DeleteVpcInput) error
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

// TagsToMap converts EC2 tags to map[string]string.
func TagsToMap(src []ec2Types.Tag) map[string]string {
	tags := make(map[string]string, len(src))

	for _, t := range src {
		tags[*t.Key] = *t.Value
	}

	return tags
}
