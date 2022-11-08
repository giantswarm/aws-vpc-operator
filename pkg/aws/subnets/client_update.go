package subnets

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

type UpdateSubnetInput struct {
	RoleARN      string
	Region       string
	SubnetId     string
	RouteTableId *string
	Tags         map[string]string
}

type UpdateSubnetOutput struct {
	RouteTableAssociation *RouteTableAssociation
}

func (c *client) Update(ctx context.Context, input UpdateSubnetInput) (output UpdateSubnetOutput, err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started updating subnet")
	defer func() {
		if err == nil {
			logger.Info("Finished updating subnet")
		} else {
			logger.Error(err, "Failed to update subnet")
		}
	}()

	if input.RoleARN == "" {
		return UpdateSubnetOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return UpdateSubnetOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.SubnetId == "" {
		return UpdateSubnetOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.SubnetId must not be empty", input)
	}

	// update subnet tags
	createTagsInput := tags.CreateTagsInput{
		RoleARN:    input.RoleARN,
		Region:     input.Region,
		ResourceId: input.SubnetId,
		Tags:       input.Tags,
	}
	err = c.tagsClient.Create(ctx, createTagsInput)
	if err != nil {
		return UpdateSubnetOutput{}, microerror.Mask(err)
	}

	var routeTableAssociation *RouteTableAssociation
	if input.RouteTableId != nil {
		routeTableAssociation, err = c.updateAssociatedRouteTable(ctx, input)
		if err != nil {
			return UpdateSubnetOutput{}, microerror.Mask(err)
		}
	}

	output = UpdateSubnetOutput{
		RouteTableAssociation: routeTableAssociation,
	}
	return output, nil
}

func (c *client) updateAssociatedRouteTable(ctx context.Context, input UpdateSubnetInput) (*RouteTableAssociation, error) {
	//
	// Before associating a route table with a subnet, first we remove an
	// existing association if it exists.
	//

	var existingAssociationId string
	{
		//
		// Look for an existing route table association for the specified subnet
		//
		const associationSubnetIdFilterName = "association.subnet-id"
		ec2Input := ec2.DescribeRouteTablesInput{
			Filters: []ec2Types.Filter{
				{
					Name:   aws.String(associationSubnetIdFilterName),
					Values: []string{input.SubnetId},
				},
			},
		}
		ec2Output, err := c.ec2Client.DescribeRouteTables(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
		if err != nil {
			return nil, microerror.Mask(err)
		}

		for _, routeTable := range ec2Output.RouteTables {
			for _, routeTableAssociation := range routeTable.Associations {
				// Not sure when these fields can be nil, but we need them, so
				// we skip the EC2 results that do not have them set.
				if routeTableAssociation.SubnetId == nil ||
					routeTableAssociation.RouteTableId == nil ||
					routeTableAssociation.RouteTableAssociationId == nil {
					continue
				}

				if *routeTableAssociation.SubnetId == input.SubnetId {
					// We have found an existing route table association for
					// the specified subnet.

					if *routeTableAssociation.RouteTableId == *input.RouteTableId {
						// specified route table has been already associated
						// with the specified subnet
						routeTableAssociationOutput := RouteTableAssociation{
							RouteTableId: *input.RouteTableId,
						}
						if routeTableAssociation.AssociationState != nil {
							routeTableAssociationOutput.AssociationStateCode = AssociationStateCode(routeTableAssociation.AssociationState.State)
						} else {
							routeTableAssociationOutput.AssociationStateCode = AssociationStateCodeUnknown
						}

						return &routeTableAssociationOutput, nil
					}

					existingAssociationId = *routeTableAssociation.RouteTableAssociationId
					break
				}
			}
		}
	}

	if existingAssociationId != "" {
		//
		// Remove an existing route table association
		//
		ec2Input := ec2.DisassociateRouteTableInput{
			AssociationId: aws.String(existingAssociationId),
		}
		_, err := c.ec2Client.DisassociateRouteTable(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	//
	// Finally, associate specified route table with the specified subnet.
	//
	ec2Input := ec2.AssociateRouteTableInput{
		RouteTableId: input.RouteTableId,
		SubnetId:     aws.String(input.SubnetId),
	}
	ec2Output, err := c.ec2Client.AssociateRouteTable(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
	if err != nil {
		return nil, microerror.Mask(err)
	}

	routeTableAssociationOutput := RouteTableAssociation{
		RouteTableId: *input.RouteTableId,
	}
	if ec2Output.AssociationState != nil {
		routeTableAssociationOutput.AssociationStateCode = AssociationStateCode(ec2Output.AssociationState.State)
	} else {
		routeTableAssociationOutput.AssociationStateCode = AssociationStateCodeUnknown
	}

	return &routeTableAssociationOutput, nil
}
