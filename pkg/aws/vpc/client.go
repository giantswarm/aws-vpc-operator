package vpc

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/giantswarm/microerror"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws/assumerole"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type VpcState string

// Enum values for VpcState
const (
	VpcStatePending   VpcState = "pending"
	VpcStateAvailable VpcState = "available"
)

type Client interface {
	Create(ctx context.Context, input CreateVpcInput) (CreateVpcOutput, error)
	Get(ctx context.Context, input GetVpcInput) (GetVpcOutput, error)
	Delete(ctx context.Context, input DeleteVpcInput) error
}

func NewClient(ec2Client *ec2.Client, assumeRoleClient assumerole.Client) (Client, error) {
	if ec2Client == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "ec2Client must not be empty")
	}
	if assumeRoleClient == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "assumeRoleClient must not be empty")
	}

	return &client{
		ec2Client:        ec2Client,
		assumeRoleClient: assumeRoleClient,
	}, nil
}

type client struct {
	ec2Client        *ec2.Client
	assumeRoleClient assumerole.Client
}

type attributes struct {
	EnableDnsHostnames bool
	EnableDnsSupport   bool
}

func (c *client) getAttributes(ctx context.Context, roleArn, region, vpcId string) (attributes, error) {
	result := attributes{}

	// get "enableDnsHostnames" attribute
	ec2Input := ec2.DescribeVpcAttributeInput{
		VpcId:     aws.String(vpcId),
		Attribute: ec2Types.VpcAttributeNameEnableDnsHostnames,
	}
	ec2Output, err := c.ec2Client.DescribeVpcAttribute(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(roleArn, region))
	if err != nil {
		return result, microerror.Mask(err)
	}

	if ec2Output.EnableDnsHostnames != nil {
		result.EnableDnsHostnames = aws.ToBool(ec2Output.EnableDnsHostnames.Value)
	}

	// get "enableDnsSupport" attribute
	ec2Input = ec2.DescribeVpcAttributeInput{
		VpcId:     aws.String(vpcId),
		Attribute: ec2Types.VpcAttributeNameEnableDnsSupport,
	}
	ec2Output, err = c.ec2Client.DescribeVpcAttribute(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(roleArn, region))
	if err != nil {
		return result, microerror.Mask(err)
	}

	if ec2Output.EnableDnsSupport != nil {
		result.EnableDnsSupport = aws.ToBool(ec2Output.EnableDnsSupport.Value)
	}

	return result, nil
}

func (c *client) updateAttribute(ctx context.Context, roleArn, region, vpcId string, attributeName ec2Types.VpcAttributeName, newValue bool) error {
	ec2Input := ec2.ModifyVpcAttributeInput{
		VpcId: aws.String(vpcId),
	}
	switch attributeName {
	case ec2Types.VpcAttributeNameEnableDnsHostnames:
		ec2Input.EnableDnsHostnames = &ec2Types.AttributeBooleanValue{
			Value: aws.Bool(newValue),
		}
	case ec2Types.VpcAttributeNameEnableDnsSupport:
		ec2Input.EnableDnsSupport = &ec2Types.AttributeBooleanValue{
			Value: aws.Bool(newValue),
		}
	default:
		return microerror.Maskf(errors.UnknownVpcAttributeError, "Trying to update unknown VPC attribute %q", attributeName)
	}
	_, err := c.ec2Client.ModifyVpcAttribute(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(roleArn, region))
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

func (c *client) ensureAttributes(ctx context.Context, roleArn, region, vpcId string, wanted attributes) error {
	current, err := c.getAttributes(ctx, roleArn, region, vpcId)
	if err != nil {
		return microerror.Mask(err)
	}

	// ensure "enableDnsSupport" attribute has wanted value
	if current.EnableDnsSupport != wanted.EnableDnsSupport {
		err = c.updateAttribute(ctx, roleArn, region, vpcId, ec2Types.VpcAttributeNameEnableDnsSupport, wanted.EnableDnsSupport)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	// ensure "enableDnsHostnames" attribute has wanted value
	if current.EnableDnsHostnames != wanted.EnableDnsHostnames {
		err = c.updateAttribute(ctx, roleArn, region, vpcId, ec2Types.VpcAttributeNameEnableDnsHostnames, wanted.EnableDnsHostnames)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	return nil
}

// TagsToMap converts EC2 tags to map[string]string.
func TagsToMap(src []ec2Types.Tag) map[string]string {
	tags := make(map[string]string, len(src))

	for _, t := range src {
		tags[*t.Key] = *t.Value
	}

	return tags
}
