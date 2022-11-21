package vpcendpoint

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capaservices "sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws"
	"github.com/giantswarm/aws-vpc-operator/pkg/aws/tags"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

type Spec struct {
	VpcId            string
	SubnetIds        []string
	SecurityGroupIds []string
	AdditionalTags   map[string]string
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
		RoleARN: request.RoleARN,
		Region:  request.Region,
		VpcId:   request.Spec.VpcId,
	}
	getOutput, err := r.client.Get(ctx, getInput)
	if errors.IsVpcEndpointNotFound(err) {
		// VPC endpoint not found, so we create one
		createInput := CreateVpcEndpointInput{
			RoleARN:          request.RoleARN,
			Region:           request.Region,
			VpcId:            request.Spec.VpcId,
			SubnetIds:        request.Spec.SubnetIds,
			SecurityGroupIds: request.Spec.SecurityGroupIds,
			Tags:             r.getVpcEndpointTags(request.ClusterName, request.Spec.VpcId, "", request.Region, request.AdditionalTags),
		}
		createOutput, err := r.client.Create(ctx, createInput)
		if err != nil {
			return aws.ReconcileResult[Status]{}, microerror.Mask(err)
		}

		result.Status = Status(createOutput)
		return result, nil
	} else if err != nil {
		return aws.ReconcileResult[Status]{}, microerror.Mask(err)
	} else {
		updateInput := UpdateVpcEndpointInput{
			RoleARN:                 request.RoleARN,
			Region:                  request.Region,
			VpcEndpointId:           getOutput.VpcEndpointId,
			CurrentSubnetIds:        getOutput.SubnetIds,
			CurrentSecurityGroupIds: getOutput.SecurityGroupIds,
			WantedSubnetIds:         request.Spec.SubnetIds,
			WantedSecurityGroupIds:  request.Spec.SecurityGroupIds,
			Tags:                    r.getVpcEndpointTags(request.ClusterName, request.Spec.VpcId, getOutput.VpcEndpointId, request.Region, request.AdditionalTags),
		}

		err = r.client.Update(ctx, updateInput)
		if err != nil {
			return aws.ReconcileResult[Status]{}, microerror.Mask(err)
		}
	}

	result.Status = Status{
		VpcEndpointId:    getOutput.VpcEndpointId,
		VpcEndpointState: getOutput.VpcEndpointState,
	}
	return result, nil
}

func (r *reconciler) getVpcEndpointTags(clusterName, vpcId, vpcEndpointId, region string, additionalTags map[string]string) map[string]string {
	id := vpcEndpointId
	if id == "" {
		id = capaservices.TemporaryResourceID
	}
	name := fmt.Sprintf("%s-vpc-endpoint-secretsmanager-%s", clusterName, region)

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
