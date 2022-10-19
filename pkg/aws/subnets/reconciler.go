package subnets

import (
	"context"
	"strings"

	"github.com/giantswarm/microerror"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capaservices "sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws/tags"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

// ReconcileRequest specified which resource is being reconciled and what is
// the specification of the desired subnets.
type ReconcileRequest struct {
	// Resource that is being reconciled.
	Resource conditions.Setter

	// Spec of the desired subnets.
	Spec Spec
}

type Spec struct {
	ClusterName    string
	RoleARN        string
	VpcId          string
	Subnets        []SubnetSpec
	AdditionalTags map[string]string
}

type SubnetSpec struct {
	SubnetId         string
	CidrBlock        string
	AvailabilityZone string
	Tags             map[string]string
}

type ReconcileResult struct {
	Subnets []SubnetStatus
}

type SubnetStatus struct {
	SubnetId         string
	VpcId            string
	CidrBlock        string
	AvailabilityZone string
	State            SubnetState
	Tags             map[string]string
}

type Reconciler interface {
	Reconcile(ctx context.Context, request ReconcileRequest) (ReconcileResult, error)
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

func (r *reconciler) Reconcile(ctx context.Context, request ReconcileRequest) (ReconcileResult, error) {
	logger := log.FromContext(ctx)
	logger.Info("Started reconciling subnets")
	defer logger.Info("Finished reconciling subnets")
	result := ReconcileResult{
		Subnets: []SubnetStatus{},
	}

	spec := request.Spec

	if spec.ClusterName == "" {
		return ReconcileResult{}, microerror.Maskf(errors.InvalidConfigError, "ClusterName must not be empty")
	}
	if spec.RoleARN == "" {
		return ReconcileResult{}, microerror.Maskf(errors.InvalidConfigError, "RoleARN must not be empty")
	}
	if spec.VpcId == "" {
		return ReconcileResult{}, microerror.Maskf(errors.InvalidConfigError, "VpcId must not be empty")
	}
	if len(spec.Subnets) == 0 {
		return ReconcileResult{}, microerror.Maskf(errors.InvalidConfigError, "Subnets must not be empty")
	}

	//
	// Get existing subnets, so we can see what is missing, and what is out of
	// date and needs updating.
	//
	getSubnetsInput := GetSubnetsInput{
		RoleARN:     spec.RoleARN,
		VpcId:       spec.VpcId,
		ClusterName: spec.ClusterName,
	}
	existingSubnets, err := r.client.Get(ctx, getSubnetsInput)
	if err != nil {
		return ReconcileResult{}, microerror.Mask(err)
	}

	//
	// Now when we know the desired and existing (actual) state, let's reconcile
	// those two sets of subnets.
	//
	for _, desiredSubnet := range request.Spec.Subnets {
		if existingSubnet, found := findExistingSubnet(existingSubnets, desiredSubnet); found {
			if desiredSubnet.SubnetId == "" {
				// since we already found the existing subnet, the desired subnet
				// should already have SubnetId set, but here we set it just in
				// case
				desiredSubnet.SubnetId = existingSubnet.SubnetId
			}
			// ... check tags
			desiredSubnetTags := r.getSubnetTags(request.Spec.ClusterName, request.Spec.AdditionalTags, desiredSubnet)

			changedOrNewTags := tags.Diff(desiredSubnetTags, existingSubnet.Tags)
			if len(changedOrNewTags) > 0 {
				// update existing subnet
				updateSubnetInput := UpdateSubnetInput{
					RoleARN:  request.Spec.RoleARN,
					SubnetId: existingSubnet.SubnetId,
					Tags:     desiredSubnetTags,
				}
				_, err = r.client.Update(ctx, updateSubnetInput)
				if err != nil {
					return ReconcileResult{}, microerror.Mask(err)
				}
			}

			// update results with existing subnet that we found here
			result.Subnets = append(result.Subnets, SubnetStatus{
				SubnetId:         existingSubnet.SubnetId,
				VpcId:            existingSubnet.VpcId,
				CidrBlock:        existingSubnet.CidrBlock,
				AvailabilityZone: existingSubnet.AvailabilityZone,
				State:            existingSubnet.State,
				Tags:             desiredSubnetTags,
			})
		} else {
			// create new subnet
			createSubnetInput := CreateSubnetInput{
				RoleARN:          spec.RoleARN,
				VpcId:            spec.VpcId,
				CidrBlock:        desiredSubnet.CidrBlock,
				AvailabilityZone: desiredSubnet.AvailabilityZone,
				Tags:             r.getSubnetTags(request.Spec.ClusterName, request.Spec.AdditionalTags, desiredSubnet),
			}
			output, err := r.client.Create(ctx, createSubnetInput)
			if err != nil {
				return ReconcileResult{}, microerror.Mask(err)
			}

			// update results with new subnet that we created here
			result.Subnets = append(result.Subnets, SubnetStatus(output))
		}
	}
	// First we find the subnets that are already created (i.e. the desired ones
	// that already exist). TODO: update existing subnets, e.g. to update tags.

	// Then we

	// Convert client output to reconcile result
	for _, getSubnetOutput := range existingSubnets {
		result.Subnets = append(result.Subnets, SubnetStatus(getSubnetOutput))
	}

	return result, nil
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
	var name strings.Builder
	name.WriteString(clusterName)
	name.WriteString("-subnet-")
	name.WriteString(role)
	name.WriteString("-")
	name.WriteString(spec.AvailabilityZone)

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
