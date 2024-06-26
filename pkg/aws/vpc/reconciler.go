package vpc

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	capaservices "sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws"
	"github.com/giantswarm/aws-vpc-operator/pkg/aws/tags"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

const (
	defaultVPCCidr = "10.0.0.0/16"
)

type Reconciler interface {
	Reconcile(ctx context.Context, spec Spec) (Status, error)
	ReconcileDelete(ctx context.Context, request aws.ReconcileRequest[aws.DeletedCloudResourceSpec]) error
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
