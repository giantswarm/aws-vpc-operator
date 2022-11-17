package routetables

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type DeleteRouteTableInput struct {
	RoleARN      string
	Region       string
	RouteTableId string
}

type DeleteRouteTablesInput struct {
	RoleARN string
	Region  string
	VpcId   string
}

func (c *client) Delete(ctx context.Context, input DeleteRouteTableInput) (err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started deleting route table")
	defer func() {
		if err == nil {
			logger.Info("Finished deleting route table")
		} else {
			logger.Error(err, "Failed to delete route table")
		}
	}()

	if input.RoleARN == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if input.RouteTableId == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.RouteTableId must not be empty", input)
	}

	// Get existing route table associations, so we can delete them
	var associations []RouteTableAssociation
	{
		getInput := GetRouteTableInput(input)
		getOutput, err := c.Get(ctx, getInput)
		if err != nil {
			return microerror.Mask(err)
		}
		associations = getOutput.AssociatedSubnets
	}

	// now delete all existing route table associations
	for _, association := range associations {
		err = c.deleteRouteTableAssociation(ctx, input.RoleARN, input.Region, association.AssociationId)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	// finally, delete the route table itself
	err = c.deleteRouteTable(ctx, input.RoleARN, input.Region, input.RouteTableId)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

func (c *client) DeleteAll(ctx context.Context, input DeleteRouteTablesInput) (err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started deleting all route tables")
	defer func() {
		if err == nil {
			logger.Info("Finished deleting all route tables")
		} else {
			logger.Error(err, "Failed to delete all route tables")
		}
	}()

	if input.RoleARN == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if input.VpcId == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.RouteTableId must not be empty", input)
	}

	listInput := ListRouteTablesInput(input)
	listOutput, err := c.List(ctx, listInput)
	if err != nil {
		return microerror.Mask(err)
	}

	for _, routeTable := range listOutput {
		// first delete all existing route table associations
		for _, association := range routeTable.AssociatedSubnets {
			err = c.deleteRouteTableAssociation(ctx, input.RoleARN, input.Region, association.AssociationId)
			if err != nil {
				return microerror.Mask(err)
			}
		}

		// then delete the route table itself
		err = c.deleteRouteTable(ctx, input.RoleARN, input.Region, routeTable.RouteTableId)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	return nil
}

func (c *client) deleteRouteTableAssociation(ctx context.Context, roleArn, region, associationId string) error {
	ec2Input := ec2.DisassociateRouteTableInput{
		AssociationId: aws.String(associationId),
	}
	_, err := c.ec2Client.DisassociateRouteTable(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(roleArn, region))
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

func (c *client) deleteRouteTable(ctx context.Context, roleArn, region, routeTableId string) error {
	ec2Input := ec2.DeleteRouteTableInput{
		RouteTableId: aws.String(routeTableId),
	}
	_, err := c.ec2Client.DeleteRouteTable(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(roleArn, region))
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}
