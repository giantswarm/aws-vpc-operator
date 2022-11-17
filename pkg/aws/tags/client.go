package tags

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/giantswarm/microerror"

	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type Client interface {
	Create(ctx context.Context, input CreateTagsInput) error
}

func NewClient(ec2Client *ec2.Client, stsCredsAssumeRoleAPIClient stscreds.AssumeRoleAPIClient) (Client, error) {
	if ec2Client == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "ec2Client must not be empty")
	}
	if stsCredsAssumeRoleAPIClient == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "stsCredsAssumeRoleAPIClient must not be empty")
	}

	return &client{
		ec2Client:                   ec2Client,
		stsCredsAssumeRoleAPIClient: stsCredsAssumeRoleAPIClient,
	}, nil
}

type client struct {
	ec2Client                   *ec2.Client
	stsCredsAssumeRoleAPIClient stscreds.AssumeRoleAPIClient
}

func (c *client) assumeRoleFunc(roleArn string) func(o *ec2.Options) {
	return func(o *ec2.Options) {
		assumeRoleProvider := stscreds.NewAssumeRoleProvider(c.stsCredsAssumeRoleAPIClient, roleArn)
		o.Credentials = aws.NewCredentialsCache(assumeRoleProvider)
	}
}
