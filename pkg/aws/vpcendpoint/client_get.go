package vpcendpoint

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type GetVpcEndpointInput struct {
	RoleARN     string
	Region      string
	ServiceName string
	Type        ec2Types.VpcEndpointType
	VpcId       string
}

type GetVpcEndpointOutput struct {
	VpcEndpointId              string
	VpcEndpointState           string
	Type                       ec2Types.VpcEndpointType
	VPCEndpointInterfaceConfig *VPCEndpointInterfaceConfig
	VPCEndpointGatewayConfig   *VPCEndpointGatewayConfig
}

func (c *client) Get(ctx context.Context, input GetVpcEndpointInput) (output GetVpcEndpointOutput, err error) {
	logger := log.FromContext(ctx).WithValues("vpc-endpoint-type", input.Type).WithValues("service-name", input.ServiceName)
	logger.Info("Started getting VPC endpoint")
	defer func() {
		if err == nil {
			logger.Info("Finished getting VPC endpoint", "vpc-endpoint-id", output.VpcEndpointId, "vpc-endpoint-state", output.VpcEndpointState)
		} else {
			logger.Error(err, "Failed to get VPC endpoint")
		}
	}()

	if input.RoleARN == "" {
		return GetVpcEndpointOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return GetVpcEndpointOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if input.ServiceName == "" {
		return GetVpcEndpointOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.ServiceName must not be empty", input)
	}
	if input.Type == "" {
		return GetVpcEndpointOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.Type must not be empty", input)
	}
	if input.VpcId == "" {
		return GetVpcEndpointOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.VpcId must not be empty", input)
	}

	ec2Input := ec2.DescribeVpcEndpointsInput{
		Filters: []ec2Types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{input.VpcId},
			},
			{
				Name:   aws.String("service-name"),
				Values: []string{input.ServiceName},
			},
			{
				Name:   aws.String("vpc-endpoint-type"),
				Values: []string{string(input.Type)},
			},
		},
	}
	ec2Output, err := c.ec2Client.DescribeVpcEndpoints(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
	if err != nil {
		return GetVpcEndpointOutput{}, microerror.Mask(err)
	}

	if len(ec2Output.VpcEndpoints) == 0 {
		return GetVpcEndpointOutput{}, microerror.Maskf(errors.VpcEndpointNotFoundError, "VPC %s endpoint %s for VPC %s not found", input.Type, input.ServiceName, input.VpcId)
	}

	// endpoint type interface
	if ec2Output.VpcEndpoints[0].VpcEndpointType == ec2Types.VpcEndpointTypeInterface {
		output = GetVpcEndpointOutput{
			VpcEndpointId:    *ec2Output.VpcEndpoints[0].VpcEndpointId,
			VpcEndpointState: string(ec2Output.VpcEndpoints[0].State),
			VPCEndpointInterfaceConfig: &VPCEndpointInterfaceConfig{
				SubnetIds: ec2Output.VpcEndpoints[0].SubnetIds,
			},
			Type: ec2Output.VpcEndpoints[0].VpcEndpointType,
		}

		for _, securityGroup := range ec2Output.VpcEndpoints[0].Groups {
			if securityGroup.GroupId == nil {
				continue
			}
			output.VPCEndpointInterfaceConfig.SecurityGroupIds = append(output.VPCEndpointInterfaceConfig.SecurityGroupIds, *securityGroup.GroupId)
		}
		// endpoint type gateway
	} else if ec2Output.VpcEndpoints[0].VpcEndpointType == ec2Types.VpcEndpointTypeGateway {
		output = GetVpcEndpointOutput{
			VpcEndpointId:    *ec2Output.VpcEndpoints[0].VpcEndpointId,
			VpcEndpointState: string(ec2Output.VpcEndpoints[0].State),
			VPCEndpointGatewayConfig: &VPCEndpointGatewayConfig{
				RouteTableIDs: ec2Output.VpcEndpoints[0].RouteTableIds,
			},
			Type: ec2Output.VpcEndpoints[0].VpcEndpointType,
		}
	}

	return output, err
}
