package vpcendpoint

import (
	"context"
	"fmt"
	"sort"

	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/giantswarm/k8smetadata/pkg/annotation"
	"github.com/giantswarm/microerror"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capaservices "sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws"
	"github.com/giantswarm/aws-vpc-operator/pkg/aws/tags"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

const (
	S3         = "s3"
	SecretData = "secretdata"
)

type Spec struct {
	VpcId            string
	SubnetIds        []string
	SecurityGroupIds []string
	AdditionalTags   map[string]string
	RouteTableIds    []string
}

type Status struct {
	VpcEndpointId    string
	VpcEndpointState string
}

func (r *reconciler) Reconcile(ctx context.Context, request aws.ReconcileRequest[Spec]) (result aws.ReconcileResult[Status], err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started reconciling VPC endpoint")
	defer func() {
		if err == nil {
			logger.Info("Finished reconciling VPC endpoint")
		} else {
			logger.Error(err, "Failed to reconcile VPC endpoint")
		}
	}()

	if !shouldReconcileVpcEndpoint(request.Resource.GetAnnotations()) {
		result.Status = Status{
			VpcEndpointState: StateAvailable,
		}
		return result, nil
	}

	if request.ClusterName == "" {
		return aws.ReconcileResult[Status]{}, microerror.Maskf(errors.InvalidConfigError, "ClusterName must not be empty")
	}
	if request.RoleARN == "" {
		return aws.ReconcileResult[Status]{}, microerror.Maskf(errors.InvalidConfigError, "RoleARN must not be empty")
	}
	if request.Region == "" {
		return aws.ReconcileResult[Status]{}, microerror.Maskf(errors.InvalidConfigError, "Region must not be empty")
	}
	if request.Spec.VpcId == "" {
		return aws.ReconcileResult[Status]{}, microerror.Maskf(errors.InvalidConfigError, "VpcId must not be empty")
	}

	result = aws.ReconcileResult[Status]{
		Status: Status{},
	}

	// Get existing VPC endpoint
	getInput := GetVpcEndpointInput{
		RoleARN:     request.RoleARN,
		Region:      request.Region,
		Type:        ec2Types.VpcEndpointTypeGateway,
		ServiceName: s3ServiceName(request.Region),
		VpcId:       request.Spec.VpcId,
	}
	getOutput, err := r.client.Get(ctx, getInput)
	if errors.IsVpcEndpointNotFound(err) {
		// VPC endpoint not found, so we create one
		createInput := CreateVpcEndpointInput{
			RoleARN:     request.RoleARN,
			Region:      request.Region,
			Type:        ec2Types.VpcEndpointTypeGateway,
			ServiceName: s3ServiceName(request.Region),
			VpcId:       request.Spec.VpcId,
			VPCEndpointGatewayConfig: &VPCEndpointGatewayConfig{
				RouteTableIDs: request.Spec.RouteTableIds,
			},
			Tags: r.getVpcEndpointTags(request.ClusterName, request.Spec.VpcId, "", S3, request.Region, request.AdditionalTags),
		}
		createOutput, err := r.client.Create(ctx, createInput)
		if err != nil {
			return aws.ReconcileResult[Status]{}, microerror.Mask(err)
		}

		result.Status = Status(createOutput)
	} else if err != nil {
		return aws.ReconcileResult[Status]{}, microerror.Mask(err)
	} else {
		// Sort current routeTablesID, so we can use sort.SearchStrings
		// when checking difference in slices.
		// This modifies slice in-place, but we just use it here anyway, so that's
		// fine.
		currentRouteTableIds := cloneAndSort(getOutput.VPCEndpointGatewayConfig.RouteTableIDs)
		wantedRouteTableIds := cloneAndSort(request.Spec.RouteTableIds)

		// securityGroupIDs that we will add, those specified in the input, but not
		// already present in current state
		routeTableIdsToBeAdded := diff(wantedRouteTableIds, currentRouteTableIds)

		// securityGroupIDs that we will remove, those already in the current state,
		// but not present in the input
		routeTableIdsToBeRemoved := diff(currentRouteTableIds, wantedRouteTableIds)

		updateInput := UpdateVpcEndpointInput{
			RoleARN:       request.RoleARN,
			Region:        request.Region,
			VpcEndpointId: getOutput.VpcEndpointId,
			VPCEndpointGatewayConfig: &VPCEndpointGatewayUpdateConfig{
				AddRouteTableIDs:    routeTableIdsToBeAdded,
				RemoveRouteTableIDs: routeTableIdsToBeRemoved,
			},
			Type:        ec2Types.VpcEndpointTypeGateway,
			ServiceName: s3ServiceName(request.Region),
			Tags:        r.getVpcEndpointTags(request.ClusterName, request.Spec.VpcId, getOutput.VpcEndpointId, S3, request.Region, request.AdditionalTags),
		}

		err = r.client.Update(ctx, updateInput)
		if err != nil {
			return aws.ReconcileResult[Status]{}, microerror.Mask(err)
		}
		result.Status = Status{
			VpcEndpointId:    getOutput.VpcEndpointId,
			VpcEndpointState: getOutput.VpcEndpointState,
		}
	}

	return result, nil
}

func (r *reconciler) getVpcEndpointTags(clusterName, vpcId, vpcEndpointId, vpcServiceName, region string, additionalTags map[string]string) map[string]string {
	id := vpcEndpointId
	if id == "" {
		id = capaservices.TemporaryResourceID
	}
	name := fmt.Sprintf("%s-vpc-endpoint-%s-%s", clusterName, vpcServiceName, region)

	allTags := map[string]string{}
	for k, v := range additionalTags {
		allTags[k] = v
	}
	allTags["vpc-id"] = vpcId
	allTags["region"] = region

	params := tags.BuildParams{
		ClusterName: clusterName,
		ResourceID:  id,
		Name:        name,
		Role:        capa.PrivateRoleTagValue,
		Additional:  allTags,
	}

	return params.Build()
}

// cloneAndSort clones the input slice first and then sorts all elements in the
// ascending order before returning the output. The cloning is done first
// because the sorting will modify the sorted slice in-place.
//
// The function is used when we want to search through a slice and use functions
// like sort.SearchStrings which require that the input slice is sorted.
func cloneAndSort(input []string) []string {
	if len(input) == 0 {
		return []string{}
	}

	output := make([]string, len(input))
	output = append(output, input...)
	sort.Strings(output)

	return output
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

func shouldReconcileVpcEndpoint(annotations map[string]string) bool {
	return annotations[annotation.VPCEndpointModeAnnotation] != annotation.VPCEndpointModeUserManaged
}
