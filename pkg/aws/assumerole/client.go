package assumerole

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/giantswarm/microerror"

	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type Client interface {
	AssumeRoleFunc(roleArn, region string) func(o *ec2.Options)
}

func NewClient(stsCredsAssumeRoleAPIClient stscreds.AssumeRoleAPIClient) (Client, error) {
	if stsCredsAssumeRoleAPIClient == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "stsCredsAssumeRoleAPIClient must not be empty")
	}

	return &client{
		stsCredsAssumeRoleAPIClient: stsCredsAssumeRoleAPIClient,
	}, nil
}

type client struct {
	stsCredsAssumeRoleAPIClient stscreds.AssumeRoleAPIClient
}

func (c *client) AssumeRoleFunc(roleArn, region string) func(o *ec2.Options) {
	return func(o *ec2.Options) {
		assumeRoleProvider := stscreds.NewAssumeRoleProvider(c.stsCredsAssumeRoleAPIClient, roleArn)
		o.Credentials = aws.NewCredentialsCache(assumeRoleProvider)
		o.Region = region
	}
}
