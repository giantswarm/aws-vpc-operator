package subnets

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/giantswarm/microerror"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws/tags"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type Client interface {
	Create(ctx context.Context, input CreateSubnetInput) (CreateSubnetOutput, error)
	Update(ctx context.Context, input UpdateSubnetInput) (UpdateSubnetOutput, error)
	Get(ctx context.Context, input GetSubnetsInput) (GetSubnetsOutput, error)
}

func NewClient(ec2Client *ec2.Client, stsCredsAssumeRoleAPIClient stscreds.AssumeRoleAPIClient) (Client, error) {
	if ec2Client == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "ec2Client must not be empty")
	}
	if stsCredsAssumeRoleAPIClient == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "stsCredsAssumeRoleAPIClient must not be empty")
	}

	tagsClient, err := tags.NewClient(ec2Client, stsCredsAssumeRoleAPIClient)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return &client{
		ec2Client:                   ec2Client,
		stsCredsAssumeRoleAPIClient: stsCredsAssumeRoleAPIClient,
		tagsClient:                  tagsClient,
	}, nil
}

type client struct {
	ec2Client                   *ec2.Client
	stsCredsAssumeRoleAPIClient stscreds.AssumeRoleAPIClient
	tagsClient                  tags.Client
}

func (c *client) assumeRoleFunc(roleArn string) func(o *ec2.Options) {
	return func(o *ec2.Options) {
		assumeRoleProvider := stscreds.NewAssumeRoleProvider(c.stsCredsAssumeRoleAPIClient, roleArn)
		o.Credentials = aws.NewCredentialsCache(assumeRoleProvider)
	}
}

// TagsToMap converts EC2 tags to map[string]string.
func TagsToMap(src []ec2Types.Tag) map[string]string {
	tags := make(map[string]string, len(src))

	for _, t := range src {
		tags[*t.Key] = *t.Value
	}

	return tags
}
