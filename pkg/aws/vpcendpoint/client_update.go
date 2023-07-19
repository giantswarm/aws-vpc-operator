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

type UpdateVpcEndpointInput struct {
	RoleARN                  string
	Region                   string
	Type                     ec2Types.VpcEndpointType
	ServiceName              string
	VpcEndpointId            string
	VPCEndpointGatewayConfig *VPCEndpointGatewayUpdateConfig
	Tags                     map[string]string
}

type VPCEndpointGatewayUpdateConfig struct {
	AddRouteTableIDs    []string
	RemoveRouteTableIDs []string
}

func (c *client) Update(ctx context.Context, input UpdateVpcEndpointInput) (err error) {
	logger := log.FromContext(ctx).WithValues("vpc-endpoint-type", input.Type).WithValues("service-name", input.ServiceName)
	logger.Info("Started updating VPC endpoint")
	defer func() {
		if err == nil {
			logger.Info("Finished updating VPC endpoint")
		} else {
			logger.Error(err, "Failed to update VPC endpoint")
		}
	}()

	if input.RoleARN == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if input.ServiceName == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.ServiceName must not be empty", input)
	}
	if input.Type == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.Type must not be empty", input)
	}
	if input.VpcEndpointId == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.VpcId must not be empty", input)
	}
	if input.VPCEndpointGatewayConfig == nil {
		return microerror.Maskf(errors.InvalidConfigError, "%T.VPCEndpointGatewayConfig cannot be nil", input, input)
	}

	if atLeastOneIsNotEmpty(input.VPCEndpointGatewayConfig.AddRouteTableIDs, input.VPCEndpointGatewayConfig.RemoveRouteTableIDs) {
		logger.Info("VPC endpoint needs updates",
			"vpc-endpoint-id", input.VpcEndpointId,
			"add-route-tables", input.VPCEndpointGatewayConfig.AddRouteTableIDs,
			"remove-route-tables", input.VPCEndpointGatewayConfig.RemoveRouteTableIDs)

		ec2Input := ec2.ModifyVpcEndpointInput{
			VpcEndpointId:       aws.String(input.VpcEndpointId),
			AddRouteTableIds:    input.VPCEndpointGatewayConfig.AddRouteTableIDs,
			RemoveRouteTableIds: input.VPCEndpointGatewayConfig.RemoveRouteTableIDs,
			ResetPolicy:         aws.Bool(true),
		}
		_, err = c.ec2Client.ModifyVpcEndpoint(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
		if err != nil {
			return microerror.Mask(err)
		}
	} else {
		logger.Info("VPC endpoint is  already up-to-date", "vpc-endpoint-id", input.VpcEndpointId)
	}

	logger.Info("Updating VPC endpoint tags", "tags", input.Tags)

	// Update VPC endpoint tags
	createTagsInput := tags.CreateTagsInput{
		RoleARN:    input.RoleARN,
		Region:     input.Region,
		ResourceId: input.VpcEndpointId,
		Tags:       input.Tags,
	}
	err = c.tagsClient.Create(ctx, createTagsInput)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

func atLeastOneIsNotEmpty(slices ...[]string) bool {
	for _, slice := range slices {
		if len(slice) > 0 {
			return true
		}
	}

	return false
}
