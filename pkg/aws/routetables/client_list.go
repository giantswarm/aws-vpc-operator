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

type ListRouteTablesInput struct {
	RoleARN string
	Region  string
	VpcId   string
}

type ListRouteTablesOutput []GetRouteTableOutput

type GetRouteTableOutput struct {
	RouteTableId      string
	AssociatedSubnets []RouteTableAssociation
	Tags              map[string]string
}

func (c *client) List(ctx context.Context, input ListRouteTablesInput) (output ListRouteTablesOutput, err error) {
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
		return ListRouteTablesOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return ListRouteTablesOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if input.VpcId == "" {
		return ListRouteTablesOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.VpcId must not be empty", input)
	}

	const vpcIdFilterName = "vpc-id"
	ec2Input := ec2.DescribeRouteTablesInput{
		Filters: []ec2Types.Filter{
			{
				Name:   aws.String(vpcIdFilterName),
				Values: []string{input.VpcId},
			},
		},
	}
	ec2Output, err := c.ec2Client.DescribeRouteTables(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
	if err != nil {
		return ListRouteTablesOutput{}, microerror.Mask(err)
	}

	output = make(ListRouteTablesOutput, len(ec2Output.RouteTables))
	for _, ec2RouteTable := range ec2Output.RouteTables {
		if ec2RouteTable.RouteTableId == nil {
			continue
		}

		routeTableOutput := GetRouteTableOutput{
			RouteTableId: *ec2RouteTable.RouteTableId,
			Tags:         tags.ToMap(ec2RouteTable.Tags),
		}

		for _, ec2RouteTableAssociation := range ec2RouteTable.Associations {
			if ec2RouteTableAssociation.SubnetId == nil {
				continue
			}

			routeTableAssociation := RouteTableAssociation{
				SubnetId: *ec2RouteTableAssociation.SubnetId,
			}

			if ec2RouteTableAssociation.AssociationState != nil {
				routeTableAssociation.AssociationStateCode = AssociationStateCode(ec2RouteTableAssociation.AssociationState.State)
			} else {
				routeTableAssociation.AssociationStateCode = AssociationStateCodeUnknown
			}

			routeTableOutput.AssociatedSubnets = append(routeTableOutput.AssociatedSubnets, routeTableAssociation)
		}

		output = append(output, routeTableOutput)
	}

	return output, nil
}
