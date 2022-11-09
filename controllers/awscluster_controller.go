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
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/giantswarm/k8smetadata/pkg/annotation"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capiannotations "sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws"
	"github.com/giantswarm/aws-vpc-operator/pkg/aws/assumerole"
	"github.com/giantswarm/aws-vpc-operator/pkg/aws/routetables"
	"github.com/giantswarm/aws-vpc-operator/pkg/aws/subnets"
	"github.com/giantswarm/aws-vpc-operator/pkg/aws/vpc"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

const (
	AwsVpcOperatorFinalizer = "aws-vpc-operator.finalizers.giantswarm.io"
)

// AWSClusterReconciler reconciles a AWSCluster object
type AWSClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	vpcReconciler         vpc.Reconciler
	subnetsReconciler     subnets.Reconciler
	routeTablesReconciler routetables.Reconciler
}

// NewAWSClusterReconciler creates a new AWSClusterReconciler for specified client and scheme.
func NewAWSClusterReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	ec2Client *ec2.Client,
	assumeRoleClient assumerole.Client,
) (*AWSClusterReconciler, error) {
	if client == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "client must not be empty")
	}
	if ec2Client == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "ec2Client must not be empty")
	}
	if assumeRoleClient == nil {
		return nil, microerror.Maskf(errors.InvalidConfigError, "assumeRoleClient must not be empty")
	}

	var vpcReconciler vpc.Reconciler
	{
		vpcClient, err := vpc.NewClient(ec2Client, assumeRoleClient)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		vpcReconciler, err = vpc.NewReconciler(vpcClient)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var subnetsReconciler subnets.Reconciler
	{
		subnetsClient, err := subnets.NewClient(ec2Client, assumeRoleClient)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		subnetsReconciler, err = subnets.NewReconciler(subnetsClient)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var routeTablesReconciler routetables.Reconciler
	{
		routeTablesClient, err := routetables.NewClient(ec2Client, assumeRoleClient)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		routeTablesReconciler, err = routetables.NewReconciler(routeTablesClient)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	return &AWSClusterReconciler{
		Client: client,
		Scheme: scheme,

		vpcReconciler:         vpcReconciler,
		subnetsReconciler:     subnetsReconciler,
		routeTablesReconciler: routeTablesReconciler,
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

	// Check VPC mode. aws-vpc-operator reconciles only private VPCs.
	vpcMode, vpcModeSet := awsCluster.Annotations[annotation.AWSVPCMode]
	if !vpcModeSet || vpcMode != annotation.AWSVPCModePrivate {
		var message string
		if !vpcModeSet {
			message = fmt.Sprintf("Annotation %s is not set, skipping", annotation.AWSVPCMode)
		} else {
			message = fmt.Sprintf("Annotation %s set to %s, skipping", annotation.AWSVPCMode, vpcMode)
		}
		log.Info(message, "namespace", req.Namespace, "name", req.Name)
		return
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
			capa.SubnetsReadyCondition,
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

	if !awsCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, log, awsCluster, identity.Spec.RoleArn)
	}

	return r.reconcileNormal(ctx, log, awsCluster, identity.Spec.RoleArn)
}

func (r *AWSClusterReconciler) reconcileNormal(ctx context.Context, logger logr.Logger, awsCluster *capa.AWSCluster, roleArn string) (_ ctrl.Result, reterr error) {
	// If the AWSCluster doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(awsCluster, AwsVpcOperatorFinalizer)

	vpcSpec := vpc.Spec{
		ClusterName:    awsCluster.Name,
		RoleARN:        roleArn,
		Region:         awsCluster.Spec.Region,
		VpcId:          awsCluster.Spec.NetworkSpec.VPC.ID,
		CidrBlock:      awsCluster.Spec.NetworkSpec.VPC.CidrBlock,
		AdditionalTags: awsCluster.Spec.AdditionalTags,
	}
	status, err := r.vpcReconciler.Reconcile(ctx, vpcSpec)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	// Update AWSCluster CR
	awsCluster.Spec.NetworkSpec.VPC.ID = status.VpcId
	awsCluster.Spec.NetworkSpec.VPC.CidrBlock = status.CidrBlock
	awsCluster.Spec.NetworkSpec.VPC.Tags = status.Tags
	switch status.State {
	case vpc.VpcStateAvailable:
		conditions.MarkTrue(awsCluster, capa.VpcReadyCondition)
	case vpc.VpcStatePending:
		conditions.MarkFalse(awsCluster, capa.VpcReadyCondition, "VpcStatePending", capi.ConditionSeverityWarning, "VPC is in pending state")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	case "":
		conditions.MarkFalse(awsCluster, capa.VpcReadyCondition, "VpcStateNotSet", capi.ConditionSeverityError, "VPC state is not set")
		return ctrl.Result{}, microerror.Maskf(errors.VpcStateNotSetError, "VPC state is not set '%s'", status.State)
	default:
		conditions.MarkFalse(awsCluster, capa.VpcReadyCondition, "VpcStateUnknown", capi.ConditionSeverityError, "VPC is in unknown state")
		return ctrl.Result{}, microerror.Maskf(errors.VpcStateUnknownError, "VPC is in unknown state '%s'", status.State)
	}

	//
	// Reconcile subnets
	//
	subnetsReconcileRequest := subnets.ReconcileRequest{
		Resource: awsCluster,
		Spec: subnets.Spec{
			ClusterName:    awsCluster.Name,
			RoleARN:        roleArn,
			VpcId:          awsCluster.Spec.NetworkSpec.VPC.ID,
			AdditionalTags: awsCluster.Spec.AdditionalTags,
			Region:         awsCluster.Spec.Region,
		},
	}
	for _, awsSubnetSpec := range awsCluster.Spec.NetworkSpec.Subnets {
		subnetSpec := subnets.SubnetSpec{
			SubnetId:         awsSubnetSpec.ID,
			CidrBlock:        awsSubnetSpec.CidrBlock,
			AvailabilityZone: awsSubnetSpec.AvailabilityZone,
			Tags:             awsSubnetSpec.Tags,
		}
		subnetsReconcileRequest.Spec.Subnets = append(subnetsReconcileRequest.Spec.Subnets, subnetSpec)
	}
	subnetsReconcileResult, err := r.subnetsReconciler.Reconcile(ctx, subnetsReconcileRequest)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	// Update AWSCluster subnets
	allSubnetsAvailable := true
	allRouteTablesReady := true
	allRouteTablesNotReadyMessage := ""
	allRouteTablesNotReadyReason := ""
	for _, existingSubnet := range subnetsReconcileResult.Subnets {
		for i := range awsCluster.Spec.NetworkSpec.Subnets {
			desiredSubnetSpec := &awsCluster.Spec.NetworkSpec.Subnets[i]
			if desiredSubnetSpec.ID == existingSubnet.SubnetId || desiredSubnetSpec.CidrBlock == existingSubnet.CidrBlock {
				desiredSubnetSpec.ID = existingSubnet.SubnetId
				desiredSubnetSpec.CidrBlock = existingSubnet.CidrBlock
				desiredSubnetSpec.AvailabilityZone = existingSubnet.AvailabilityZone
				desiredSubnetSpec.Tags = existingSubnet.Tags

				// Update subnet route table ID in the subnet spec
				if existingSubnet.RouteTableAssociation.RouteTableId != "" {
					routeTableId := existingSubnet.RouteTableAssociation.RouteTableId
					desiredSubnetSpec.RouteTableID = &routeTableId
					if existingSubnet.RouteTableAssociation.AssociationStateCode != subnets.AssociationStateCodeAssociated {
						allRouteTablesReady = false
						allRouteTablesNotReadyMessage = fmt.Sprintf("Route table %s for subnet %s not ready", routeTableId, existingSubnet.SubnetId)
						// e.g. RouteTableAssociationStateAssociating
						allRouteTablesNotReadyReason = "RouteTableAssociationState" + cases.Title(language.English).String(string(existingSubnet.RouteTableAssociation.AssociationStateCode))
					}
				} else {
					allRouteTablesReady = false
				}

				if existingSubnet.State != subnets.SubnetStateAvailable {
					allSubnetsAvailable = false
				}
				break
			}
		}
	}
	if allSubnetsAvailable && allRouteTablesReady {
		conditions.MarkTrue(awsCluster, capa.SubnetsReadyCondition)
	} else {
		if !allSubnetsAvailable {
			// subnets are not available
			conditions.MarkFalse(awsCluster, capa.SubnetsReadyCondition, "SubnetNotAvailable", capi.ConditionSeverityWarning, "One or more subnets is still not available")
		} else {
			// route tables are not ready
			conditions.MarkFalse(awsCluster, capa.SubnetsReadyCondition, allRouteTablesNotReadyReason, capi.ConditionSeverityWarning, allRouteTablesNotReadyMessage)
		}

		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Reconcile route tables
	// TODO implement

	cluster := &capi.Cluster{}
	clusterKey := types.NamespacedName{
		Namespace: awsCluster.Namespace,
		Name:      awsCluster.Name,
	}
	err = r.Client.Get(ctx, clusterKey, cluster)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	//
	// We have successfully created private VPC and subnets, now we can unpause
	// the cluster, so that CAPA can take over the reconciliation.
	//
	// Unpause Cluster CR
	if capiannotations.IsPaused(cluster, cluster) {
		cluster.Spec.Paused = false
		delete(cluster.Annotations, capi.PausedAnnotation)
		err = r.Client.Update(ctx, cluster)
		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
	}
	// Unpause AWSCluster
	if capiannotations.IsPaused(cluster, awsCluster) {
		delete(awsCluster.Annotations, capi.PausedAnnotation)
		// We don't update the CR here, as patch helper in Reconcile method
		// will do that.
	}

	return ctrl.Result{}, nil
}

func (r *AWSClusterReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, awsCluster *capa.AWSCluster, roleArn string) (_ ctrl.Result, err error) {
	//
	// Delete route tables
	//
	routeTablesDeleted := capiconditions.IsFalse(awsCluster, capa.RouteTablesReadyCondition) &&
		capiconditions.GetReason(awsCluster, capa.RouteTablesReadyCondition) == capi.DeletedReason
	if routeTablesDeleted {
		logger.Info("Route tables are already deleted")
	} else {
		logger.Info("Deleting route tables")
		routeTablesDeleteRequest := aws.ReconcileRequest[aws.DeletedCloudResourceSpec]{
			Resource:    awsCluster,
			ClusterName: awsCluster.Name,
			CloudResourceRequest: aws.CloudResourceRequest[aws.DeletedCloudResourceSpec]{
				RoleARN: roleArn,
				Region:  awsCluster.Spec.Region,
				Spec: aws.DeletedCloudResourceSpec{
					Id: awsCluster.Spec.NetworkSpec.VPC.ID,
				},
			},
		}
		err = r.routeTablesReconciler.ReconcileDelete(ctx, routeTablesDeleteRequest)
		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
		// remove subnet IDs
		for i := range awsCluster.Spec.NetworkSpec.Subnets {
			awsCluster.Spec.NetworkSpec.Subnets[i].RouteTableID = nil
		}
		conditions.MarkFalse(awsCluster, capa.RouteTablesReadyCondition, capi.DeletedReason, capi.ConditionSeverityInfo, "Route tables have been deleted")
		logger.Info("Deleted route tables")
	}

	//
	// Delete subnets
	//
	subnetsDeleted := capiconditions.IsFalse(awsCluster, capa.SubnetsReadyCondition) &&
		capiconditions.GetReason(awsCluster, capa.SubnetsReadyCondition) == capi.DeletedReason
	if subnetsDeleted {
		logger.Info("Subnets are already deleted")
	} else {
		logger.Info("Deleting subnets")
		subnetsDeleteRequest := aws.ReconcileRequest[[]aws.DeletedCloudResourceSpec]{
			Resource:    awsCluster,
			ClusterName: awsCluster.Name,
			CloudResourceRequest: aws.CloudResourceRequest[[]aws.DeletedCloudResourceSpec]{
				RoleARN: roleArn,
				Region:  awsCluster.Spec.Region,
			},
		}
		for _, awsSubnetSpec := range awsCluster.Spec.NetworkSpec.Subnets {
			if awsSubnetSpec.ID != "" {
				deletedSubnetSpec := aws.DeletedCloudResourceSpec{
					Id: awsSubnetSpec.ID,
				}
				subnetsDeleteRequest.Spec = append(subnetsDeleteRequest.Spec, deletedSubnetSpec)
			}
		}
		err = r.subnetsReconciler.ReconcileDelete(ctx, subnetsDeleteRequest)
		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
		// remove subnet IDs
		for i := range awsCluster.Spec.NetworkSpec.Subnets {
			awsCluster.Spec.NetworkSpec.Subnets[i].ID = ""
		}
		conditions.MarkFalse(awsCluster, capa.SubnetsReadyCondition, capi.DeletedReason, capi.ConditionSeverityInfo, "Subnets have been deleted")
		logger.Info("Deleted subnets")
	}

	//
	// Delete VPC
	//
	vpcDeleted := capiconditions.IsFalse(awsCluster, capa.VpcReadyCondition) &&
		capiconditions.GetReason(awsCluster, capa.VpcReadyCondition) == capi.DeletedReason
	if vpcDeleted {
		logger.Info("VPC already deleted")
	} else {
		vpcId := awsCluster.Spec.NetworkSpec.VPC.ID
		logger.Info("Deleting VPC", "vpc-id", vpcId)
		vpcDeleteRequest := aws.ReconcileRequest[aws.DeletedCloudResourceSpec]{
			Resource:    awsCluster,
			ClusterName: awsCluster.Name,
			CloudResourceRequest: aws.CloudResourceRequest[aws.DeletedCloudResourceSpec]{
				RoleARN: roleArn,
				Region:  awsCluster.Spec.Region,
				Spec: aws.DeletedCloudResourceSpec{
					Id: awsCluster.Spec.NetworkSpec.VPC.ID,
				},
			},
		}
		err = r.vpcReconciler.ReconcileDelete(ctx, vpcDeleteRequest)
		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
		conditions.MarkFalse(awsCluster, capa.VpcReadyCondition, capi.DeletedReason, capi.ConditionSeverityInfo, "VPC has been deleted")
		// unset VPC ID as we have deleted the AWS VPC, so the ID is not valid anymore
		awsCluster.Spec.NetworkSpec.VPC.ID = ""
		logger.Info("Deleted VPC", "vpc-id", vpcId)
	}

	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(awsCluster, AwsVpcOperatorFinalizer)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AWSClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capa.AWSCluster{}).
		Complete(r)
}
