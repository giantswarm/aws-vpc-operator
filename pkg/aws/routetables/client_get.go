package routetables

import (
	"context"

	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type GetRouteTableInput struct {
	RoleARN      string
	Region       string
	RouteTableId string
}

func (c *client) Get(ctx context.Context, input GetRouteTableInput) (output RouteTableOutput, err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started getting route table")
	defer func() {
		if err == nil {
			logger.Info("Finished getting route table")
		} else {
			logger.Error(err, "Failed to get route table")
		}
	}()

	if input.RoleARN == "" {
		return RouteTableOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return RouteTableOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if input.RouteTableId == "" {
		return RouteTableOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.RouteTableId must not be empty", input)
	}

	const routeTableIdFilterName = "route-table-id"
	listOutput, err := c.listWithFilter(ctx, input.RoleARN, input.Region, routeTableIdFilterName, input.RouteTableId)
	if err != nil {
		return RouteTableOutput{}, microerror.Mask(err)
	}

	if len(listOutput) == 0 {
		return RouteTableOutput{}, microerror.Maskf(errors.RouteTableNotFoundError, "Route table with ID %s is not found", input.RouteTableId)
	}

	output = listOutput[0]
	return output, nil
}
