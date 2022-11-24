package subnets

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type DeleteSubnetsInput struct {
	RoleARN   string
	Region    string
	SubnetIds []string
}

func (c *client) Delete(ctx context.Context, input DeleteSubnetsInput) (err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started deleting subnets")
	defer func() {
		if err == nil {
			logger.Info("Finished deleting subnets")
		} else {
			logger.Error(err, "Failed to delete subnets")
		}
	}()

	if input.RoleARN == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if len(input.SubnetIds) == 0 {
		return microerror.Maskf(errors.InvalidConfigError, "%T.SubnetIds must not be empty", input)
	}

	for _, subnetId := range input.SubnetIds {
		logger.Info("Deleting subnet", "subnet-id", subnetId)
		ec2Input := ec2.DeleteSubnetInput{
			SubnetId: aws.String(subnetId),
		}
		_, err = c.ec2Client.DeleteSubnet(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
		if errors.IsAWSHTTPStatusNotFound(err) {
			logger.Info("Subnet not found, nothing to delete", "subnet-id", subnetId)
			continue
		} else if err != nil {
			return microerror.Mask(err)
		}
		logger.Info("Deleted subnet", "subnet-id", subnetId)
	}

	return nil
}
