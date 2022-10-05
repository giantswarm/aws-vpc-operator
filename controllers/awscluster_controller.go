/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws/vpc"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"

	"github.com/giantswarm/microerror"
	"k8s.io/apimachinery/pkg/runtime"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// AWSClusterReconciler reconciles a AWSCluster object
type AWSClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	vpcReconciler vpc.Reconciler
}

// NewAWSClusterReconciler creates a new AWSClusterReconciler for specified client and scheme.
func NewAWSClusterReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	ec2Client *ec2.Client,
	assumeRoleAPIClient stscreds.AssumeRoleAPIClient,
) (*AWSClusterReconciler, error) {
	if ec2Client == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "ec2Client must not be empty")
	}
	if assumeRoleAPIClient == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "assumeRoleAPIClient must not be empty")
	}

	var vpcReconciler vpc.Reconciler
	{
		vpcClient, err := vpc.NewClient(ec2Client, assumeRoleAPIClient)
		if err == nil {
			return nil, microerror.Mask(err)
		}

		vpcReconciler, err = vpc.NewReconciler(vpcClient)
		if err == nil {
			return nil, microerror.Mask(err)
		}
	}

	return &AWSClusterReconciler{
		Client: client,
		Scheme: scheme,

		vpcReconciler: vpcReconciler,
	}, nil
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io.giantswarm.io,resources=awsclusters,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io.giantswarm.io,resources=awsclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io.giantswarm.io,resources=awsclusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AWSCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *AWSClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := log.FromContext(ctx)
	log.Info("Started reconciling AWSCluster", "namespace", req.Namespace, "name", req.Name)
	defer log.Info("Finished reconciling AWSCluster", "namespace", req.Namespace, "name", req.Name)

	//
	// Get AWSCluster that we are reconciling
	//
	awsCluster := &capa.AWSCluster{}
	err := r.Client.Get(ctx, req.NamespacedName, awsCluster)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	// We don't reconcile AWSClusters that have VPC managed by CAPA
	if awsCluster.Spec.NetworkSpec.VPC.IsManaged(awsCluster.Name) {
		return ctrl.Result{}, nil
	}

	//
	// Create patch helper that will update reconciler AWSCLuster if there are any changes in the CR
	//
	patchHelper, err := patch.NewHelper(awsCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}
	defer func() {
		conditionsToUpdate := []capi.ConditionType{
			capa.VpcReadyCondition,
			// capa.SubnetsReadyCondition,
			// capa.RouteTablesReadyCondition,
		}
		err := patchHelper.Patch(
			ctx,
			awsCluster,
			patch.WithOwnedConditions{
				Conditions: conditionsToUpdate,
			})
		if err != nil {
			reterr = err
		}
	}()

	// We need Spec.IdentityRef to be set, TODO check this
	if awsCluster.Spec.IdentityRef == nil {
		return ctrl.Result{}, microerror.Maskf(errors.IdentityNotSetError, "AWSCluster %s/%s does not have Spec.IdentityRef set", awsCluster.Namespace, awsCluster.Name)
	}

	identity := &capa.AWSClusterRoleIdentity{}
	identityNamespacedName := types.NamespacedName{
		Namespace: awsCluster.Namespace,
		Name:      awsCluster.Spec.IdentityRef.Name,
	}

	err = r.Client.Get(ctx, identityNamespacedName, identity)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	vpcSpec := vpc.Spec{
		ClusterName: awsCluster.Name,
		RoleARN:     identity.Spec.RoleArn,
		VpcId:       awsCluster.Spec.NetworkSpec.VPC.ID,
		CidrBlock:   awsCluster.Spec.NetworkSpec.VPC.CidrBlock,
	}
	status, err := r.vpcReconciler.Reconcile(ctx, vpcSpec)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	// Update AWSCluster CR
	awsCluster.Spec.NetworkSpec.VPC.ID = status.VpcId
	awsCluster.Spec.NetworkSpec.VPC.CidrBlock = status.CidrBlock
	awsCluster.Spec.NetworkSpec.VPC.Tags = status.Tags
	conditions.MarkTrue(awsCluster, capa.VpcReadyCondition)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AWSClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capa.AWSCluster{}).
		Complete(r)
}