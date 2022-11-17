package routetables

import (
	"context"

	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws/tags"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type UpdateRouteTableInput struct {
	RoleARN      string
	Region       string
	RouteTableId string
	Tags         map[string]string
}

func (c *client) Update(ctx context.Context, input UpdateRouteTableInput) (err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started updating route table")
	defer func() {
		if err == nil {
			logger.Info("Finished updating route table")
		} else {
			logger.Error(err, "Failed to update route table")
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

	// update route table tags
	createTagsInput := tags.CreateTagsInput{
		RoleARN:    input.RoleARN,
		Region:     input.Region,
		ResourceId: input.RouteTableId,
		Tags:       input.Tags,
	}
	err = c.tagsClient.Create(ctx, createTagsInput)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}
