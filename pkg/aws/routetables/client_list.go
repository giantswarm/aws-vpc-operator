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

type ListRouteTablesOutput []RouteTableOutput

type RouteTableOutput struct {
	RouteTableId      string
	AssociatedSubnets []RouteTableAssociation
	Tags              map[string]string
}

func (c *client) List(ctx context.Context, input ListRouteTablesInput) (output ListRouteTablesOutput, err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started listing route tables")
	defer func() {
		if err == nil {
			logger.Info("Finished listing route tables", "count", len(output))
		} else {
			logger.Error(err, "Failed to list route tables")
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
	output, err = c.listWithFilter(ctx, input.RoleARN, input.Region, vpcIdFilterName, input.VpcId)
	if err != nil {
		return ListRouteTablesOutput{}, microerror.Mask(err)
	}

	return output, nil
}

func (c *client) listWithFilter(ctx context.Context, roleArn, region, filterName, filterValue string) (output ListRouteTablesOutput, err error) {
	logger := log.FromContext(ctx)
	ec2Input := ec2.DescribeRouteTablesInput{
		Filters: []ec2Types.Filter{
			{
				Name:   aws.String(filterName),
				Values: []string{filterValue},
			},
		},
	}
	ec2Output, err := c.ec2Client.DescribeRouteTables(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(roleArn, region))
	if err != nil {
		return ListRouteTablesOutput{}, microerror.Mask(err)
	}

	output = ListRouteTablesOutput{}
	for _, ec2RouteTable := range ec2Output.RouteTables {
		if ec2RouteTable.RouteTableId == nil || *ec2RouteTable.RouteTableId == "" {
			continue
		}

		routeTableOutput := RouteTableOutput{
			RouteTableId: *ec2RouteTable.RouteTableId,
			Tags:         tags.ToMap(ec2RouteTable.Tags),
		}

		for _, ec2RouteTableAssociation := range ec2RouteTable.Associations {
			if ec2RouteTableAssociation.RouteTableAssociationId == nil ||
				ec2RouteTableAssociation.SubnetId == nil {
				continue
			}

			routeTableAssociation := RouteTableAssociation{
				AssociationId: *ec2RouteTableAssociation.RouteTableAssociationId,
				SubnetId:      *ec2RouteTableAssociation.SubnetId,
			}

			if ec2RouteTableAssociation.AssociationState != nil {
				routeTableAssociation.AssociationStateCode = AssociationStateCode(ec2RouteTableAssociation.AssociationState.State)
			} else {
				routeTableAssociation.AssociationStateCode = AssociationStateCodeUnknown
			}

			routeTableOutput.AssociatedSubnets = append(routeTableOutput.AssociatedSubnets, routeTableAssociation)
		}

		output = append(output, routeTableOutput)
		logger.Info("Found route table", "route-table-id", routeTableOutput.RouteTableId, "route-table-tags", routeTableOutput.Tags)
	}

	return output, nil
}
