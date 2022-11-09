package routetables

import (
	"context"
)

type DeleteRouteTableInput struct {
	RoleARN      string
	Region       string
	RouteTableId string
}

func (c *client) Delete(ctx context.Context, input DeleteRouteTableInput) error {
	return nil
}
