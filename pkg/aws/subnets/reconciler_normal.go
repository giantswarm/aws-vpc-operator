package subnets

import (
	"context"

	"github.com/giantswarm/microerror"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws/tags"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

func (r *reconciler) Reconcile(ctx context.Context, request ReconcileRequest) (result ReconcileResult, err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started reconciling subnets")
	defer func() {
		if err == nil {
			logger.Info("Finished reconciling subnets")
		} else {
			logger.Error(err, "Failed to reconcile subnets")
		}
	}()

	result = ReconcileResult{
		Subnets: []SubnetStatus{},
	}

	spec := request.Spec

	if spec.ClusterName == "" {
		return ReconcileResult{}, microerror.Maskf(errors.InvalidConfigError, "ClusterName must not be empty")
	}
	if spec.RoleARN == "" {
		return ReconcileResult{}, microerror.Maskf(errors.InvalidConfigError, "RoleARN must not be empty")
	}
	if spec.Region == "" {
		return ReconcileResult{}, microerror.Maskf(errors.InvalidConfigError, "Region must not be empty")
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
		Region:      spec.Region,
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
			//
			// Existing subnet found
			//
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
				//
				// Update existing subnet with new tags.
				//
				updateSubnetInput := UpdateSubnetInput{
					RoleARN:  request.Spec.RoleARN,
					Region:   spec.Region,
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
				SubnetId:              existingSubnet.SubnetId,
				VpcId:                 existingSubnet.VpcId,
				CidrBlock:             existingSubnet.CidrBlock,
				AvailabilityZone:      existingSubnet.AvailabilityZone,
				State:                 existingSubnet.State,
				RouteTableAssociation: existingSubnet.RouteTableAssociation,
				Tags:                  desiredSubnetTags,
			})
		} else {
			//
			// Desired subnet not found, let's create it.
			//
			createSubnetInput := CreateSubnetInput{
				RoleARN:          spec.RoleARN,
				Region:           spec.Region,
				VpcId:            spec.VpcId,
				CidrBlock:        desiredSubnet.CidrBlock,
				AvailabilityZone: desiredSubnet.AvailabilityZone,
				Tags:             r.getSubnetTags(request.Spec.ClusterName, request.Spec.AdditionalTags, desiredSubnet),
			}
			output, err := r.client.Create(ctx, createSubnetInput)
			if err != nil {
				return ReconcileResult{}, microerror.Mask(err)
			}

			status := SubnetStatus{
				SubnetId:         output.SubnetId,
				VpcId:            output.VpcId,
				CidrBlock:        output.CidrBlock,
				AvailabilityZone: output.AvailabilityZone,
				State:            output.State,
				Tags:             output.Tags,
			}

			// update results with new subnet that we created here
			result.Subnets = append(result.Subnets, status)
		}
	}

	// Convert client output to reconcile result
	for _, getSubnetOutput := range existingSubnets {
		result.Subnets = append(result.Subnets, SubnetStatus(getSubnetOutput))
	}

	return result, nil
}

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
	Region         string
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
	SubnetId              string
	VpcId                 string
	CidrBlock             string
	AvailabilityZone      string
	State                 SubnetState
	RouteTableAssociation RouteTableAssociation
	Tags                  map[string]string
}
