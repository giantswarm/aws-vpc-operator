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
	RouteTableId string

	// AssociationsToSubnets contains all subnets to which the route table is associated to.
	AssociationsToSubnets []RouteTableAssociation

	// OtherAssociations contains IDs of all route table associations to
	// resources other than subnets.
	//
	// These are separated from AssociationsToSubnets because in most cases we care
	// about subnet associations only. Other associations are used less often,
	// during route table deletion for example.
	OtherAssociations []string

	// Tags that are currently set on the AWS route table resource.
	Tags map[string]string
}

// GetAllAssociationIds returns IDs of all route table associations (to subnets
// and other resources).
func (rto RouteTableOutput) GetAllAssociationIds() []string {
	var allAssociations []string
	for _, association := range rto.AssociationsToSubnets {
		allAssociations = append(allAssociations, association.AssociationId)
	}
	allAssociations = append(allAssociations, rto.OtherAssociations...)
	return allAssociations
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
			logger.Info("Skipping route table without ID set")
			continue
		}

		routeTableOutput := RouteTableOutput{
			RouteTableId: *ec2RouteTable.RouteTableId,
			Tags:         tags.ToMap(ec2RouteTable.Tags),
		}

		for _, ec2RouteTableAssociation := range ec2RouteTable.Associations {
			if ec2RouteTableAssociation.RouteTableAssociationId == nil {
				logger.Info("Skipping adding route table association to output when listing (association ID not set)")
				continue
			}
			associationId := *ec2RouteTableAssociation.RouteTableAssociationId

			// We found an association to a resources other than subnet, we just
			// add association ID to the list of all associations
			if ec2RouteTableAssociation.SubnetId == nil {
				logger.Info("Found route table associated to a resource other than subnet", "route-table-id", *ec2RouteTable.RouteTableId, "association-id", *ec2RouteTableAssociation.RouteTableAssociationId)
				routeTableOutput.OtherAssociations = append(routeTableOutput.OtherAssociations, associationId)
				continue
			}

			// We found a subnet association
			routeTableAssociation := RouteTableAssociation{
				AssociationId: associationId,
				SubnetId:      *ec2RouteTableAssociation.SubnetId,
			}

			if ec2RouteTableAssociation.AssociationState != nil {
				routeTableAssociation.AssociationStateCode = AssociationStateCode(ec2RouteTableAssociation.AssociationState.State)
			} else {
				routeTableAssociation.AssociationStateCode = AssociationStateCodeUnknown
			}

			routeTableOutput.AssociationsToSubnets = append(routeTableOutput.AssociationsToSubnets, routeTableAssociation)
		}

		output = append(output, routeTableOutput)
		logger.Info("Found route table", "route-table-id", routeTableOutput.RouteTableId, "route-table-tags", routeTableOutput.Tags)
	}

	return output, nil
}
