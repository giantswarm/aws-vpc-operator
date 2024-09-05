package vpcendpoint

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type DeleteVpcEndpointInput struct {
	RoleARN     string
	Region      string
	ServiceName string
	Type        ec2Types.VpcEndpointType
	VpcId       string
}

func (c *client) Delete(ctx context.Context, input DeleteVpcEndpointInput) (err error) {
	logger := log.FromContext(ctx).WithValues("vpc-endpoint-type", input.Type).WithValues("service-name", input.ServiceName)
	logger.Info("Started deleting VPC endpoint", "vpc-id", input.VpcId)
	defer func() {
		if err == nil {
			logger.Info("Finished deleting VPC endpoint")
		} else {
			logger.Error(err, "Failed to delete VPC endpoint")
		}
	}()

	if input.RoleARN == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if input.ServiceName == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.ServiceName must not be empty", input)
	}
	if input.Type == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.Type must not be empty", input)
	}
	if input.VpcId == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.VpcId must not be empty", input)
	}

	getInput := GetVpcEndpointInput(input)
	vpcEndpoint, err := c.Get(ctx, getInput)
	if err != nil {
		return microerror.Mask(err)
	}

	if vpcEndpoint.VpcEndpointState == StateDeleted {
		message := "VPC endpoint is already deleted"
		logger.Info(message, "vpc-endpoint-id", vpcEndpoint.VpcEndpointId)
		return microerror.Maskf(errors.ResourceAlreadyDeletedError, "%s", message)
	}
	if vpcEndpoint.VpcEndpointState == StateDeleting {
		message := "VPC endpoint deletion is already in progress"
		logger.Info(message, "vpc-endpoint-id", vpcEndpoint.VpcEndpointId)
		return microerror.Maskf(errors.ResourceDeletionInProgressError, "%s", message)
	}

	logger.Info("Found VPC endpoint to delete", "vpc-endpoint-id", vpcEndpoint.VpcEndpointId)
	ec2Input := ec2.DeleteVpcEndpointsInput{
		VpcEndpointIds: []string{vpcEndpoint.VpcEndpointId},
	}
	_, err = c.ec2Client.DeleteVpcEndpoints(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
	if err != nil {
		return microerror.Mask(err)
	}

	return err
}
