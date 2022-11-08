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
	SubnetId              string
	VpcId                 string
	CidrBlock             string
	AvailabilityZone      string
	State                 SubnetState
	RouteTableAssociation RouteTableAssociation
	Tags                  map[string]string
}

func (c *client) Get(ctx context.Context, input GetSubnetsInput) (output GetSubnetsOutput, err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started getting subnet")
	defer func() {
		if err == nil {
			logger.Info("Finished getting subnet")
		} else {
			logger.Error(err, "Failed to get subnet")
		}
	}()

	if input.RoleARN == "" {
		return GetSubnetsOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return GetSubnetsOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if input.VpcId == "" {
		return GetSubnetsOutput{}, microerror.Maskf(errors.InvalidConfigError, "%T.VpcId must not be empty", input)
	}

	output = GetSubnetsOutput{}
	subnetsMap := map[string]*GetSubnetOutput{}

	//
	// Get subnet details for all subnets in the VPC
	//
	{
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

			newlyAddedSubnet := &output[len(output)-1]
			subnetsMap[newlyAddedSubnet.SubnetId] = newlyAddedSubnet
		}
	}

	//
	// Get route table associations for all subnets
	//
	{
		ec2Input := ec2.DescribeRouteTablesInput{
			Filters: []ec2Types.Filter{
				{
					Name:   aws.String(filterNameVpcID),
					Values: []string{input.VpcId},
				},
			},
		}
		ec2Output, err := c.ec2Client.DescribeRouteTables(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
		if err != nil {
			return GetSubnetsOutput{}, microerror.Mask(err)
		}

		// Now match route tables to subnets
		for _, ec2RouteTable := range ec2Output.RouteTables {
			if ec2RouteTable.RouteTableId == nil {
				continue
			}

			for _, ec2RouteTableAssociation := range ec2RouteTable.Associations {
				if ec2RouteTableAssociation.SubnetId == nil {
					continue
				}
				if subnetOutput, ok := subnetsMap[*ec2RouteTableAssociation.SubnetId]; ok {
					// We found a route table association for a VPC subnet
					subnetOutput.RouteTableAssociation.RouteTableId = *ec2RouteTable.RouteTableId

					if ec2RouteTableAssociation.AssociationState != nil {
						subnetOutput.RouteTableAssociation.AssociationStateCode = AssociationStateCode(ec2RouteTableAssociation.AssociationState.State)
					} else {
						subnetOutput.RouteTableAssociation.AssociationStateCode = AssociationStateCodeUnknown
					}

					logger.Info("Found route table for subnet",
						"subnet-id", subnetOutput.SubnetId,
						"route-table-id", subnetOutput.RouteTableAssociation.RouteTableId,
						"association-state", subnetOutput.RouteTableAssociation.AssociationStateCode)

					// We create one route table per subnet, so every route
					// table will be associated to a single subnet, meaning
					// that we could break out the loop here, as we found a
					// subnet for this route table.
					// By checking all associations for the route table, we
					// enable a possible future scenario where one route
					// table is associated to multiple subnets.
				}
			}
		}
	}

	// Output now contains all subnet details, as well as associated route
	// table for every subnet (if route table was created and associated).

	logger.Info(fmt.Sprintf("Got %d subnets for VPC", len(output)), "vpc-id", input.VpcId)
	return output, nil
}
