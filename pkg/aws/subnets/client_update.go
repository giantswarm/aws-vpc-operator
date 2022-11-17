package subnets

import (
	"context"

	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws/tags"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type UpdateSubnetInput struct {
	RoleARN  string
	Region   string
	SubnetId string
	Tags     map[string]string
}

type UpdateSubnetOutput struct {
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

	return UpdateSubnetOutput{}, nil
}
