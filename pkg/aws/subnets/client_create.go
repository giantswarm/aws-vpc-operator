package subnets

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws/tags"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type CreateSubnetInput struct {
	RoleARN          string
	Region           string
	VpcId            string
	CidrBlock        string
	AvailabilityZone string
	Tags             map[string]string
}

type SubnetState string

// Enum values for SubnetState
const (
	SubnetStatePending   SubnetState = "pending"
	SubnetStateAvailable SubnetState = "available"
	SubnetStateUnknown   SubnetState = "unknown"
)

type CreateSubnetOutput struct {
	SubnetId         string
	VpcId            string
	CidrBlock        string
	AvailabilityZone string
	State            SubnetState
	Tags             map[string]string
}

func (c *client) Create(ctx context.Context, input CreateSubnetInput) (CreateSubnetOutput, error) {
	logger := log.FromContext(ctx)
	logger.Info("Started creating subnet")
	defer logger.Info("Finished creating subnet")

	if input.RoleARN == "" {
		return CreateSubnetOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return CreateSubnetOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if input.VpcId == "" {
		return CreateSubnetOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.VpcId must not be empty", input)
	}
	if input.CidrBlock == "" {
		return CreateSubnetOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.CidrBlock must not be empty", input)
	}
	if input.AvailabilityZone == "" {
		return CreateSubnetOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.AvailabilityZone must not be empty", input)
	}

	ec2Input := ec2.CreateSubnetInput{
		VpcId:            &input.VpcId,
		CidrBlock:        &input.CidrBlock,
		AvailabilityZone: &input.AvailabilityZone,
		TagSpecifications: []ec2Types.TagSpecification{
			// tags.CreateTagSpecification(input.NameTagValue, ec2Types.ResourceTypeSubnet, input.Tags),
			tags.BuildParamsToTagSpecification(ec2Types.ResourceTypeSubnet, input.Tags),
		},
	}

	ec2Output, err := c.ec2Client.CreateSubnet(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
	if err != nil {
		return CreateSubnetOutput{}, microerror.Mask(err)
	}

	var subnetState SubnetState
	switch ec2Output.Subnet.State {
	case ec2Types.SubnetStatePending:
		subnetState = SubnetStatePending
	case ec2Types.SubnetStateAvailable:
		subnetState = SubnetStateAvailable
	default:
		subnetState = SubnetStateUnknown
	}

	output := CreateSubnetOutput{
		SubnetId:         *ec2Output.Subnet.SubnetId,
		VpcId:            *ec2Output.Subnet.VpcId,
		CidrBlock:        *ec2Output.Subnet.CidrBlock,
		AvailabilityZone: *ec2Output.Subnet.AvailabilityZone,
		State:            subnetState,
		Tags:             TagsToMap(ec2Output.Subnet.Tags),
	}

	logger.Info("Created new subnet",
		"subnet-id", output.SubnetId,
		"vpc-id", output.VpcId,
		"cidr-block", output.CidrBlock,
		"availability-zone", output.AvailabilityZone,
		"state", output.State)

	return output, nil
}
