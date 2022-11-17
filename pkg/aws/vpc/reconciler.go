package vpc

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capaservices "sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws/tags"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

const (
	defaultVPCCidr = "10.0.0.0/16"
)

type Spec struct {
	ClusterName    string
	RoleARN        string
	Region         string
	VpcId          string
	CidrBlock      string
	AdditionalTags map[string]string
}

type Status struct {
	VpcId     string
	CidrBlock string
	State     VpcState
	Tags      map[string]string
}

type Reconciler interface {
	Reconcile(ctx context.Context, spec Spec) (Status, error)
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

func (s *reconciler) Reconcile(ctx context.Context, spec Spec) (Status, error) {
	logger := log.FromContext(ctx)
	logger.Info("Started reconciling VPC")
	defer logger.Info("Finished reconciling VPC")

	if spec.ClusterName == "" {
		return Status{}, microerror.Maskf(errors.InvalidConfigError, "%T.ClusterName must not be empty", spec)
	}
	if spec.RoleARN == "" {
		return Status{}, microerror.Maskf(errors.InvalidConfigError, "%T.RoleARN must not be empty", spec)
	}
	if spec.Region == "" {
		return Status{}, microerror.Maskf(errors.InvalidConfigError, "%T.Region must not be empty", spec)
	}

	if spec.VpcId != "" {
		//
		// Get existing VPC and return status
		//
		getVpcInput := GetVpcInput{
			RoleARN:     spec.RoleARN,
			Region:      spec.Region,
			VpcId:       spec.VpcId,
			ClusterName: spec.ClusterName,
		}
		getVpcOutput, err := s.client.Get(ctx, getVpcInput)
		if err != nil {
			return Status{}, microerror.Mask(err)
		}

		status := Status(getVpcOutput)
		return status, nil
	}

	if spec.CidrBlock == "" {
		spec.CidrBlock = defaultVPCCidr
	}

	//
	// Create new VPC
	//
	createVpcInput := CreateVpcInput{
		RoleARN:   spec.RoleARN,
		Region:    spec.Region,
		CidrBlock: spec.CidrBlock,
		Tags:      s.getVpcTags(spec),
	}
	createVpcOutput, err := s.client.Create(ctx, createVpcInput)
	if err != nil {
		return Status{}, microerror.Mask(err)
	}

	status := Status(createVpcOutput)
	return status, nil
}

func (s *reconciler) getVpcTags(spec Spec) map[string]string {
	id := spec.VpcId
	if id == "" {
		id = capaservices.TemporaryResourceID
	}
	name := fmt.Sprintf("%s-vpc", spec.ClusterName)

	params := tags.BuildParams{
		ClusterName: spec.ClusterName,
		ResourceID:  id,
		Name:        name,
		Role:        capa.CommonRoleTagValue,
		Additional:  spec.AdditionalTags,
	}

	return params.Build()
}
