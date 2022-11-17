package vpc

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws/tags"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type CreateVpcInput struct {
	RoleARN            string
	Region             string
	CidrBlock          string
	Tags               map[string]string
	EnableDnsHostnames bool
	EnableDnsSupport   bool
}

type CreateVpcOutput struct {
	VpcId     string
	CidrBlock string
	State     VpcState
	Tags      map[string]string
}

func (c *client) Create(ctx context.Context, input CreateVpcInput) (CreateVpcOutput, error) {
	logger := log.FromContext(ctx)
	logger.Info("Started creating VPC")
	defer logger.Info("Finished creating VPC")

	if input.RoleARN == "" {
		return CreateVpcOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return CreateVpcOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if input.CidrBlock == "" {
		return CreateVpcOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.CidrBlock must not be empty", input)
	}

	ec2Input := ec2.CreateVpcInput{
		CidrBlock: &input.CidrBlock,
		TagSpecifications: []ec2Types.TagSpecification{
			tags.BuildParamsToTagSpecification(ec2Types.ResourceTypeVpc, input.Tags),
		},
	}

	ec2Output, err := c.ec2Client.CreateVpc(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
	if err != nil {
		return CreateVpcOutput{}, microerror.Mask(err)
	}

	wantedAttributes := attributes{
		EnableDnsHostnames: input.EnableDnsHostnames,
		EnableDnsSupport:   input.EnableDnsSupport,
	}
	err = c.ensureAttributes(ctx, input.RoleARN, input.Region, *ec2Output.Vpc.VpcId, wantedAttributes)
	if err != nil {
		return CreateVpcOutput{}, microerror.Mask(err)
	}

	output := CreateVpcOutput{
		VpcId:     *ec2Output.Vpc.VpcId,
		CidrBlock: *ec2Output.Vpc.CidrBlock,
		State:     VpcState(ec2Output.Vpc.State),
		Tags:      TagsToMap(ec2Output.Vpc.Tags),
	}
	logger.Info("Created new VPC with CIDR", "vpc-id", output.VpcId, "cidr-block", output.CidrBlock)

	return output, nil
}
