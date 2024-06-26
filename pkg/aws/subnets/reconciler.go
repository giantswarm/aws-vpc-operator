package subnets

import (
	"context"
	"strings"

	"github.com/giantswarm/microerror"
	capaservices "sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws"
	"github.com/giantswarm/aws-vpc-operator/pkg/aws/tags"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type Reconciler interface {
	Reconcile(ctx context.Context, request ReconcileRequest) (ReconcileResult, error)
	ReconcileDelete(ctx context.Context, request aws.ReconcileRequest[[]aws.DeletedCloudResourceSpec]) error
}

func NewReconciler(client Client) (Reconciler, error) {
	if client == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "client must not be empty")
	}

	return &reconciler{
		client: client,
	}, nil
}

type reconciler struct {
	client Client
}

func findExistingSubnet(existingSubnets GetSubnetsOutput, desiredSubnet SubnetSpec) (GetSubnetOutput, bool) {
	for _, existingSubnet := range existingSubnets {
		sameId := existingSubnet.SubnetId == desiredSubnet.SubnetId
		sameCIDR := existingSubnet.CidrBlock == desiredSubnet.CidrBlock

		if sameId || sameCIDR {
			return existingSubnet, true
		}
	}

	return GetSubnetOutput{}, false
}

func (r *reconciler) getSubnetTags(clusterName string, clusterTags map[string]string, spec SubnetSpec) map[string]string {
	var role string

	// 1. set cluster-wide tags (coming from AWSCluster.AdditionalTags)
	allSubnetTags := make(map[string]string)
	for k, v := range clusterTags {
		allSubnetTags[k] = v
	}

	// 2. set load balancer tags
	const internalLoadBalancerTag = "kubernetes.io/role/internal-elb"
	role = capa.PrivateRoleTagValue
	allSubnetTags[internalLoadBalancerTag] = "1"
	// Add tag needed for Service type=LoadBalancer
	allSubnetTags[capa.NameKubernetesAWSCloudProviderPrefix+clusterName] = string(capa.ResourceLifecycleShared)

	// 3. set subnet-specific tags (coming from SubnetSpec.Tags)
	for k, v := range spec.Tags {
		allSubnetTags[k] = v
	}

	// 4. finally, build all tags with tag builder which also sets predefined/fixed tags

	// Prefer `Name` tag if given, else generate a name
	var name strings.Builder
	if manualTagName, ok := spec.Tags["Name"]; ok {
		name.WriteString(manualTagName)
	} else {
		name.WriteString(clusterName)
		name.WriteString("-subnet-")
		name.WriteString(role)
		name.WriteString("-")
		name.WriteString(spec.AvailabilityZone)
	}

	id := spec.SubnetId
	if id == "" {
		id = capaservices.TemporaryResourceID
	}

	params := tags.BuildParams{
		ClusterName: clusterName,
		ResourceID:  id,
		Name:        name.String(),
		Role:        role,
		Additional:  allSubnetTags,
	}

	return params.Build()
}
