package vpc

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type DeleteVpcInput struct {
	RoleARN string
	Region  string
	VpcId   string
}

func (c *client) Delete(ctx context.Context, input DeleteVpcInput) error {
	logger := log.FromContext(ctx)
	logger.Info("Started deleting VPC")
	defer logger.Info("Finished deleting VPC")

	if input.RoleARN == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if input.VpcId == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.VpcId must not be empty", input)
	}

	ec2Input := ec2.DeleteVpcInput{
		VpcId: aws.String(input.VpcId),
	}
	_, err := c.ec2Client.DeleteVpc(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
	if errors.IsVpcNotFound(err) {
		logger.Info("VPC not found, nothing to delete", "vpc-id", input.VpcId)
		return nil
	} else if err != nil {
		return microerror.Mask(err)
	}
	logger.Info("Deleted VPC with ID", "vpc-id", input.VpcId)

	return nil
}
