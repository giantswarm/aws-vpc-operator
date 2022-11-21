package vpcendpoint

import (
	"context"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/giantswarm/microerror"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type UpdateVpcEndpointInput struct {
	RoleARN                 string
	Region                  string
	VpcEndpointId           string
	CurrentSubnetIds        []string
	CurrentSecurityGroupIds []string
	WantedSubnetIds         []string
	WantedSecurityGroupIds  []string
	Tags                    map[string]string
}

func (c *client) Update(ctx context.Context, input UpdateVpcEndpointInput) (err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started updating VPC endpoint")
	defer func() {
		if err == nil {
			logger.Info("Finished updating VPC endpoint")
		} else {
			logger.Error(err, "Failed to update VPC endpoint")
		}
	}()

	if input.RoleARN == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", input)
	}
	if input.Region == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", input)
	}
	if input.VpcEndpointId == "" {
		return microerror.Maskf(errors.InvalidConfigError, "%T.VpcId must not be empty", input)
	}
	if len(input.WantedSubnetIds) == 0 {
		return microerror.Maskf(errors.InvalidConfigError, "%T.WantedSubnetIds must not be empty", input)
	}
	if len(input.WantedSecurityGroupIds) == 0 {
		return microerror.Maskf(errors.InvalidConfigError, "%T.WantedSecurityGroupIds must not be empty", input)
	}

	// Sort current securityGroupIds and subnetIds, so we can use sort.SearchStrings
	// when checking difference in slices.
	// This modifies slice in-place, but we just use it here anyway, so that's
	// fine.
	sort.Strings(input.CurrentSecurityGroupIds)
	sort.Strings(input.CurrentSubnetIds)

	// securityGroupIDs that we will add, those specified in the input, but not
	// already present in current state
	securityGroupIdsToBeAdded := diff(input.WantedSecurityGroupIds, input.CurrentSecurityGroupIds)

	// securityGroupIDs that we will remove, those already in the current state,
	// but not present in the input
	securityGroupIdsToBeRemoved := diff(input.CurrentSecurityGroupIds, input.WantedSecurityGroupIds)

	// subnets that we will add, those specified in the input, but not already
	// present in current state
	subnetIdsToBeAdded := diff(input.WantedSubnetIds, input.CurrentSubnetIds)

	// subnets that we will remove, those already in the current state, but not
	// present in the input
	subnetIdsToBeRemoved := diff(input.CurrentSubnetIds, input.WantedSubnetIds)

	if atLeastOneIsNotEmpty(securityGroupIdsToBeAdded, securityGroupIdsToBeRemoved, subnetIdsToBeAdded, subnetIdsToBeRemoved) {
		logger.Info("VPC endpoint needs updates",
			"vpc-endpoint-id", input.VpcEndpointId,
			"add-security-groups", securityGroupIdsToBeAdded,
			"remove-security-groups", securityGroupIdsToBeRemoved,
			"add-subnets", subnetIdsToBeAdded,
			"remove-subnets", subnetIdsToBeRemoved)

		ec2Input := ec2.ModifyVpcEndpointInput{
			VpcEndpointId:          aws.String(input.VpcEndpointId),
			AddSecurityGroupIds:    securityGroupIdsToBeAdded,
			AddSubnetIds:           subnetIdsToBeAdded,
			RemoveSecurityGroupIds: securityGroupIdsToBeRemoved,
			RemoveSubnetIds:        subnetIdsToBeRemoved,
			ResetPolicy:            aws.Bool(true),
		}
		_, err = c.ec2Client.ModifyVpcEndpoint(ctx, &ec2Input, c.assumeRoleClient.AssumeRoleFunc(input.RoleARN, input.Region))
		if err != nil {
			return microerror.Mask(err)
		}
	} else {
		logger.Info("VPC endpoint is  already up-to-date", "vpc-endpoint-id", input.VpcEndpointId)
	}

	return nil
}

// diff returns all values from sortedS1 and not present in sortedS2.
//
// Example:
//
//	["a", "b", "c", "d"] - ["a", "c", "e", "f"] = ["b", "d"]
func diff(sortedS1, sortedS2 []string) []string {
	var result []string

	for _, s := range sortedS1 {
		i := sort.SearchStrings(sortedS2, s)
		if i < len(sortedS2) && sortedS2[i] == s {
			// string s from sortedS1 found in sortedS2 at index i
			continue
		} else {
			result = append(result, s)
		}
	}

	return result
}

func atLeastOneIsNotEmpty(slices ...[]string) bool {
	for _, slice := range slices {
		if len(slice) > 0 {
			return true
		}
	}

	return false
}
