package vpc

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type GetVpcInput struct {
	RoleARN     string
	Region      string
	VpcId       string
	ClusterName string
}

type GetVpcOutput struct {
	VpcId     string
	CidrBlock string
	State     VpcState
	Tags      map[string]string
}

func (c *client) Get(ctx context.Context, input GetVpcInput) (GetVpcOutput, error) {
	logger := log.FromContext(ctx)
	logger.Info("Started getting VPC")
	defer logger.Info("Finished getting VPC")

	if input.RoleARN == "" {
		return GetVpcOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return GetVpcOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if input.VpcId == "" {
		return GetVpcOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.VpcId must not be empty", input)
	}

	ec2Input := ec2.DescribeVpcsInput{
		VpcIds: []string{input.VpcId},
	}

	ec2Output, err := c.ec2Client.DescribeVpcs(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
	if err != nil {
		return GetVpcOutput{}, microerror.Mask(err)
	}

	if len(ec2Output.Vpcs) == 0 {
		return GetVpcOutput{}, microerror.Maskf(errors.VpcNotFoundError, "could not find vpc %q", input.VpcId)
	} else if len(ec2Output.Vpcs) > 1 {
		return GetVpcOutput{}, microerror.Maskf(errors.VpcConflictError, "found %v VPCs with matching tags for %v. Only one VPC per cluster name is supported. Ensure duplicate VPCs are deleted for this AWS account and there are no conflicting instances of Cluster API Provider AWS. filtered VPCs: %v", len(ec2Output.Vpcs), input.ClusterName, ec2Output.Vpcs)
	}

	output := GetVpcOutput{
		VpcId:     *ec2Output.Vpcs[0].VpcId,
		CidrBlock: *ec2Output.Vpcs[0].CidrBlock,
		State:     VpcState(ec2Output.Vpcs[0].State),
		Tags:      TagsToMap(ec2Output.Vpcs[0].Tags),
	}
	logger.Info("Got existing VPC", "vpc-id", output.VpcId, "cidr-block", output.CidrBlock)

	return output, nil
}
