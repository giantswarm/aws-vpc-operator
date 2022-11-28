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
	getInput := GetRouteTableInput(input)
	getOutput, err := c.Get(ctx, getInput)
	if err != nil {
		return microerror.Mask(err)
	}

	isMainRouteTable := false

	// now delete all existing route table associations
	for _, association := range getOutput.GetAllAssociations() {
		if association.Main {
			isMainRouteTable = true
			logger.Info("Skipped deleting route table association for main route table", "route-table-id", input.RouteTableId, "association", association)
			continue
		}
		err = c.deleteRouteTableAssociation(ctx, input.RoleARN, input.Region, input.RouteTableId, association.AssociationId)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	if isMainRouteTable {
		logger.Info("Skipped deleting main route table", "route-table-id", input.RouteTableId)
	} else {
		// finally, delete the route table itself
		err = c.deleteRouteTable(ctx, input.RoleARN, input.Region, input.RouteTableId)
		if err != nil {
			return microerror.Mask(err)
		}
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
		return microerror.Maskf(errors.InvalidConfigError, "%T.VpcId must not be empty", input)
	}

	listInput := ListRouteTablesInput(input)
	listOutput, err := c.List(ctx, listInput)
	if err != nil {
		return microerror.Mask(err)
	}

	for _, routeTable := range listOutput {
		isMainRouteTable := false
		allAssociations := routeTable.GetAllAssociations()

		// then we delete all route table associations
		if len(allAssociations) > 0 {
			logger.Info("Deleting subnet associations for route table", "route-table-id", routeTable.RouteTableId)
		}
		for _, association := range allAssociations {
			if association.Main {
				logger.Info("Skipped deleting route table association for main route table", "route-table-id", routeTable.RouteTableId, "association", association)
				isMainRouteTable = true
				continue
			}
			err = c.deleteRouteTableAssociation(ctx, input.RoleARN, input.Region, routeTable.RouteTableId, association.AssociationId)
			if err != nil {
				return microerror.Mask(err)
			}
		}

		if isMainRouteTable {
			logger.Info("Skipped deleting main route table", "route-table-id", routeTable.RouteTableId)
		} else {
			// finally delete the route table itself
			err = c.deleteRouteTable(ctx, input.RoleARN, input.Region, routeTable.RouteTableId)
			if err != nil {
				return microerror.Mask(err)
			}
		}
	}

	return nil
}

func (c *client) deleteRouteTableAssociation(ctx context.Context, roleArn, region, routeTableId, associationId string) error {
	logger := log.FromContext(ctx)
	logger.Info("Deleting route table association", "route-table-id", routeTableId, "association", associationId)
	ec2Input := ec2.DisassociateRouteTableInput{
		AssociationId: aws.String(associationId),
	}
	_, err := c.ec2Client.DisassociateRouteTable(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(roleArn, region))
	if errors.IsAWSHTTPStatusNotFound(err) {
		logger.Info("Route table association not found, nothing to delete", "route-table-id", routeTableId, "association-id", associationId)
		return nil
	} else if err != nil {
		return microerror.Mask(err)
	}

	logger.Info("Deleted route table association", "route-table-id", routeTableId, "association-id", associationId)
	return nil
}

func (c *client) deleteRouteTable(ctx context.Context, roleArn, region, routeTableId string) error {
	logger := log.FromContext(ctx)
	logger.Info("Deleting route table", "route-table-id", routeTableId)
	ec2Input := ec2.DeleteRouteTableInput{
		RouteTableId: aws.String(routeTableId),
	}
	_, err := c.ec2Client.DeleteRouteTable(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(roleArn, region))
	if errors.IsAWSHTTPStatusNotFound(err) {
		logger.Info("Route table not found, nothing to delete", "route-table-id", routeTableId)
		return nil
	} else if err != nil {
		return microerror.Mask(err)
	}

	logger.Info("Deleted route table", "route-table-id", routeTableId)
	return nil
}
