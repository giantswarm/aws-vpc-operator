package routetables

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

type CreateRouteTableInput struct {
	RoleARN  string
	Region   string
	VpcId    string
	SubnetId string
	Tags     map[string]string
}

type CreateRouteTableOutput struct {
	RouteTableId         string
	AssociationStateCode AssociationStateCode
}

func (c *client) Create(ctx context.Context, input CreateRouteTableInput) (output CreateRouteTableOutput, err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started creating route table")
	defer func() {
		if err == nil {
			logger.Info("Finished creating route table")
		} else {
			logger.Error(err, "Failed to create route table")
		}
	}()

	if input.RoleARN == "" {
		return CreateRouteTableOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return CreateRouteTableOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if input.VpcId == "" {
		return CreateRouteTableOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.VpcId must not be empty", input)
	}
	if input.SubnetId == "" {
		return CreateRouteTableOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.SubnetId must not be empty", input)
	}

	//
	// Create route table
	//
	var routeTableId string
	{
		ec2Input := ec2.CreateRouteTableInput{
			VpcId: &input.VpcId,
			TagSpecifications: []ec2Types.TagSpecification{
				tags.BuildParamsToTagSpecification(ec2Types.ResourceTypeRouteTable, input.Tags),
			},
		}
		ec2Output, err := c.ec2Client.CreateRouteTable(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
		if err != nil {
			return CreateRouteTableOutput{}, microerror.Mask(err)
		}
		if ec2Output.RouteTable.RouteTableId == nil {
			return CreateRouteTableOutput{}, microerror.Maskf(errors.RouteTableIdNotSetError, "Created route table for VPC %s, but route table ID is not set", input.VpcId)
		}

		routeTableId = *ec2Output.RouteTable.RouteTableId
	}
	output = CreateRouteTableOutput{
		RouteTableId: routeTableId,
	}

	//
	// Associate route table to a specified subnet
	//
	{
		ec2Input := ec2.AssociateRouteTableInput{
			RouteTableId: aws.String(routeTableId),
			SubnetId:     aws.String(input.SubnetId),
		}
		ec2Output, err := c.ec2Client.AssociateRouteTable(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
		if err != nil {
			return CreateRouteTableOutput{}, microerror.Mask(err)
		}

		if ec2Output.AssociationState != nil {
			output.AssociationStateCode = AssociationStateCode(ec2Output.AssociationState.State)
		} else {
			output.AssociationStateCode = AssociationStateCodeUnknown
		}
	}

	return output, nil
}
