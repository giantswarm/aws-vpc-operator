package tags

import (
	"context"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type CreateTagsInput struct {
	RoleARN    string
	Region     string
	ResourceId string
	Tags       map[string]string
}

func (c *client) Create(ctx context.Context, input CreateTagsInput) error {
	logger := log.FromContext(ctx)
	logger.Info("Started creating tags")
	defer logger.Info("Finished creating tags")

	if input.RoleARN == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if input.ResourceId == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.ResourceId must not be empty", input)
	}

	// For testing, we need sorted keys
	sortedKeys := make([]string, 0, len(input.Tags))
	for k := range input.Tags {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	tags := make([]ec2Types.Tag, len(input.Tags))
	for _, key := range sortedKeys {
		tags = append(tags,
			ec2Types.Tag{
				Key:   aws.String(key),
				Value: aws.String(input.Tags[key]),
			})
	}

	ec2Input := ec2.CreateTagsInput{
		Resources: []string{input.ResourceId},
		Tags:      tags,
	}

	_, err := c.ec2Client.CreateTags(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}
