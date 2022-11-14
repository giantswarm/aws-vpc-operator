package vpcendpoint

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws/tags"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type CreateVpcEndpointInput struct {
	RoleARN          string
	Region           string
	VpcId            string
	SubnetIds        []string
	SecurityGroupIds []string
	Tags             map[string]string
}

type CreateVpcEndpointOutput struct {
	VpcEndpointId    string
	VpcEndpointState string
}

func (c *client) Create(ctx context.Context, input CreateVpcEndpointInput) (output CreateVpcEndpointOutput, err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started creating VPC endpoint")
	defer func() {
		if err == nil {
			logger.Info("Finished creating VPC endpoint")
		} else {
			logger.Error(err, "Failed to create VPC endpoint")
		}
	}()

	if input.RoleARN == "" {
		return CreateVpcEndpointOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return CreateVpcEndpointOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if input.VpcId == "" {
		return CreateVpcEndpointOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.VpcId must not be empty", input)
	}

	ec2Input := ec2.CreateVpcEndpointInput{
		VpcId:       aws.String(input.VpcId),
		ServiceName: aws.String(secretsManagerServiceName(input.Region)),
		DnsOptions: &ec2Types.DnsOptionsSpecification{
			DnsRecordIpType: ec2Types.DnsRecordIpTypeIpv4,
		},
		IpAddressType:     ec2Types.IpAddressTypeIpv4,
		PrivateDnsEnabled: aws.Bool(true),
		SecurityGroupIds:  input.SecurityGroupIds,
		SubnetIds:         input.SubnetIds,
		TagSpecifications: []ec2Types.TagSpecification{
			tags.BuildParamsToTagSpecification(ec2Types.ResourceTypeVpcEndpoint, input.Tags),
		},
		VpcEndpointType: ec2Types.VpcEndpointTypeInterface,
	}
	ec2Output, err := c.ec2Client.CreateVpcEndpoint(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
	if err != nil {
		return CreateVpcEndpointOutput{}, microerror.Mask(err)
	}

	output = CreateVpcEndpointOutput{
		VpcEndpointId:    *ec2Output.VpcEndpoint.VpcEndpointId,
		VpcEndpointState: string(ec2Output.VpcEndpoint.State),
	}

	return output, err
}
