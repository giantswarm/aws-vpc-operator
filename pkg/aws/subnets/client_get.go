package subnets

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

const (
	filterNameVpcID = "vpc-id"
	filterNameState = "state"
)

type GetSubnetsInput struct {
	RoleARN     string
	Region      string
	VpcId       string
	ClusterName string
}

type GetSubnetsOutput []GetSubnetOutput

type GetSubnetOutput struct {
	SubnetId         string
	VpcId            string
	CidrBlock        string
	AvailabilityZone string
	State            SubnetState
	Tags             map[string]string
}

func (c *client) Get(ctx context.Context, input GetSubnetsInput) (GetSubnetsOutput, error) {
	logger := log.FromContext(ctx)
	logger.Info("Started getting subnet")
	defer logger.Info("Finished getting subnet")

	if input.RoleARN == "" {
		return GetSubnetsOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return GetSubnetsOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if input.VpcId == "" {
		return GetSubnetsOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.VpcId must not be empty", input)
	}

	ec2Input := ec2.DescribeSubnetsInput{
		Filters: []ec2Types.Filter{
			{
				Name:   aws.String(filterNameState),
				Values: []string{string(ec2Types.SubnetStatePending), string(ec2Types.SubnetStateAvailable)},
			},
			{
				Name:   aws.String(filterNameVpcID),
				Values: []string{input.VpcId},
			},
		},
	}

	ec2Output, err := c.ec2Client.DescribeSubnets(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
	if err != nil {
		return GetSubnetsOutput{}, microerror.Mask(err)
	}

	output := make(GetSubnetsOutput, len(ec2Output.Subnets))

	for _, ec2Subnet := range ec2Output.Subnets {
		var subnetState SubnetState
		switch ec2Subnet.State {
		case ec2Types.SubnetStatePending:
			subnetState = SubnetStatePending
		case ec2Types.SubnetStateAvailable:
			subnetState = SubnetStateAvailable
		default:
			subnetState = SubnetStateUnknown
		}

		output = append(output, GetSubnetOutput{
			SubnetId:         *ec2Subnet.SubnetId,
			VpcId:            *ec2Subnet.VpcId,
			CidrBlock:        *ec2Subnet.CidrBlock,
			AvailabilityZone: *ec2Subnet.AvailabilityZone,
			State:            subnetState,
			Tags:             TagsToMap(ec2Subnet.Tags),
		})
	}

	logger.Info(fmt.Sprintf("Got %d subnets for VPC", len(output)), "vpc-id", input.VpcId)
	return output, nil
}
