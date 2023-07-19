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
	RoleARN     string
	Region      string
	ServiceName string
	Tags        map[string]string
	Type        ec2Types.VpcEndpointType
	VpcId       string

	VPCEndpointGatewayConfig *VPCEndpointGatewayConfig
}

type VPCEndpointGatewayConfig struct {
	RouteTableIDs []string
}

type CreateVpcEndpointOutput struct {
	VpcEndpointId    string
	VpcEndpointState string
}

func (c *client) Create(ctx context.Context, input CreateVpcEndpointInput) (output CreateVpcEndpointOutput, err error) {
	logger := log.FromContext(ctx).WithValues("vpc-endpoint-type", input.Type).WithValues("service-name", input.ServiceName)
	logger.Info("Started creating VPC endpoint")
	defer func() {
		if err == nil {
			logger.Info("Finished creating VPC endpoint", "vpc-endpoint-id", output.VpcEndpointId, "vpc-endpoint-state", output.VpcEndpointState)
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
	if input.ServiceName == "" {
		return CreateVpcEndpointOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.ServiceName must not be empty", input)
	}
	if input.Type == "" {
		return CreateVpcEndpointOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.Type must not be empty", input)
	}
	if input.VpcId == "" {
		return CreateVpcEndpointOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.VpcId must not be empty", input)
	}
	if input.VPCEndpointGatewayConfig == nil {
		return CreateVpcEndpointOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.VPCEndpointGatewayConfig cannot be nil", input)
	}

	ec2Input := ec2.CreateVpcEndpointInput{
		VpcId:         aws.String(input.VpcId),
		ServiceName:   aws.String(input.ServiceName),
		RouteTableIds: input.VPCEndpointGatewayConfig.RouteTableIDs,
		TagSpecifications: []ec2Types.TagSpecification{
			tags.BuildParamsToTagSpecification(ec2Types.ResourceTypeVpcEndpoint, input.Tags),
		},
		VpcEndpointType: input.Type,
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
