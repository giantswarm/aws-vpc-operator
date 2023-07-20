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
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/giantswarm/k8smetadata/pkg/annotation"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
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
	"github.com/giantswarm/aws-vpc-operator/pkg/aws/vpcendpoint"
	"github.com/giantswarm/aws-vpc-operator/pkg/errors"
)

const (
	AwsVpcOperatorFinalizer = "aws-vpc-operator.finalizers.giantswarm.io"

	VpcEndpointReady              capi.ConditionType = "VpcEndpointReady"
	ClusterSecurityGroupsNotReady string             = "ClusterSecurityGroupsNotReady"
	SubnetLookupFailed            string             = "SubnetLookupFailed"
	RouteTableLookupFailed        string             = "RouteTableLookupFailed"
)

// AWSClusterReconciler reconciles a AWSCluster object
type AWSClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	vpcReconciler         vpc.Reconciler
	subnetsReconciler     subnets.Reconciler
	subnetsClient         subnets.Client
	routeTablesReconciler routetables.Reconciler
	routeTablesClient     routetables.Client
	vpcEndpointReconciler vpcendpoint.Reconciler
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
	var err error

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

	var subnetsClient subnets.Client
	{
		subnetsClient, err = subnets.NewClient(ec2Client, assumeRoleClient)
		if err != nil {
			return nil, microerror.Mask(err)
		}

	}
	var subnetsReconciler subnets.Reconciler
	{
		subnetsReconciler, err = subnets.NewReconciler(subnetsClient)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var routeTablesClient routetables.Client
	{
		routeTablesClient, err = routetables.NewClient(ec2Client, assumeRoleClient)
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

	var vpcEndpointReconciler vpcendpoint.Reconciler
	{
		vpcEndpointClient, err := vpcendpoint.NewClient(ec2Client, assumeRoleClient)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		vpcEndpointReconciler, err = vpcendpoint.NewReconciler(vpcEndpointClient)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	return &AWSClusterReconciler{
		Client: client,
		Scheme: scheme,

		vpcReconciler:         vpcReconciler,
		subnetsReconciler:     subnetsReconciler,
		subnetsClient:         subnetsClient,
		routeTablesReconciler: routeTablesReconciler,
		routeTablesClient:     routeTablesClient,
		vpcEndpointReconciler: vpcEndpointReconciler,
	}, nil
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io.giantswarm.io,resources=awsclusters,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io.giantswarm.io,resources=awsclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io.giantswarm.io,resources=awsclusters/finalizers,verbs=update

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
	if apierrors.IsNotFound(err) {
		log.Info("AWSCluster no longer exists")
		return ctrl.Result{}, nil
	} else if err != nil {
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
		if !awsCluster.DeletionTimestamp.IsZero() && apierrors.IsNotFound(err) {
			// AWSCluster is already deleted, so ignore not found error since
			// there is nothing to upgrade
			return
		}
		if err != nil {
			// An error occurred while patching the AWSCluster resource
			reterr = err
		}
	}()

	if !awsCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, log, awsCluster, identity.Spec.RoleArn)
	}

	return r.reconcileNormal(ctx, log, awsCluster, identity.Spec.RoleArn)
}

func (r *AWSClusterReconciler) reconcileNormal(ctx context.Context, logger logr.Logger, awsCluster *capa.AWSCluster, roleArn string) (_ ctrl.Result, reterr error) {
	log := log.FromContext(ctx)
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
					allRouteTablesNotReadyReason = "RouteTableNotCreated"
					allRouteTablesNotReadyMessage = fmt.Sprintf("Route table not created for subnet %s", existingSubnet.SubnetId)
				}

				if existingSubnet.State != subnets.SubnetStateAvailable {
					allSubnetsAvailable = false
				}
				break
			}
		}
	}

	subnetsReadyConditionSeverity := conditions.GetSeverity(awsCluster, capa.SubnetsReadyCondition)
	subnetsReadyConditionSeverityInfo := subnetsReadyConditionSeverity != nil && *subnetsReadyConditionSeverity == capi.ConditionSeverityInfo

	subnetsReadyConditionLastChange := conditions.GetLastTransitionTime(awsCluster, capa.SubnetsReadyCondition)
	subnetsReadyConditionTimeSinceLastChange := 0 * time.Minute
	if subnetsReadyConditionLastChange != nil {
		subnetsReadyConditionTimeSinceLastChange = time.Since(subnetsReadyConditionLastChange.Time)
	}

	if !allSubnetsAvailable {
		// subnets are not available, so we wait for subnets to become available before proceeding
		const subnetsNotAvailableReason = "SubnetsNotAvailable"
		var newSeverity capi.ConditionSeverity
		var newMessage string
		var requeueTime time.Duration

		// initially retry more often, but then back off, so we don't hammer the API server
		if subnetsReadyConditionTimeSinceLastChange < 5*time.Minute && subnetsReadyConditionSeverityInfo {
			// it's been less than 5 minutes, all good, just an info here,
			// let's try again in a minute
			newSeverity = capi.ConditionSeverityInfo
			requeueTime = 1 * time.Minute
			newMessage = "One or more subnets is still not available"
		} else if subnetsReadyConditionTimeSinceLastChange < 15*time.Minute && subnetsReadyConditionSeverityInfo {
			// it's been less than 15 minutes, all good, just an info here,
			// let just wait a bit longer and try again in 5 minutes
			newSeverity = capi.ConditionSeverityInfo
			requeueTime = 5 * time.Minute
			newMessage = "One or more subnets is still not available"
		} else {
			// it's been more than 15 minutes, it's taking a while, so now
			// it's a warning, and let's try again in 15 minutes
			newSeverity = capi.ConditionSeverityWarning
			requeueTime = 15 * time.Minute
			newMessage = "One or more subnets is still not available for more than 15 minutes"
		}

		conditions.MarkFalse(awsCluster, capa.SubnetsReadyCondition, subnetsNotAvailableReason, newSeverity, newMessage)
		return ctrl.Result{RequeueAfter: requeueTime}, nil
	}

	if allRouteTablesReady {
		conditions.MarkTrue(awsCluster, capa.SubnetsReadyCondition)
	} else {
		// route tables are not ready (or not even created), we update condition
		// and proceed with route tables reconciliation
		if subnetsReadyConditionTimeSinceLastChange < 15*time.Minute && subnetsReadyConditionSeverityInfo {
			// it's been less than 15 minutes, so all is still good
			conditions.MarkFalse(awsCluster, capa.SubnetsReadyCondition, allRouteTablesNotReadyReason, capi.ConditionSeverityInfo, allRouteTablesNotReadyMessage)
		} else {
			// it's been more than 15 minutes, route tables should have been associated until now
			conditions.MarkFalse(awsCluster, capa.SubnetsReadyCondition, allRouteTablesNotReadyReason, capi.ConditionSeverityWarning, allRouteTablesNotReadyMessage+" for more than 15 minutes")
		}
	}

	//
	// Reconcile route tables
	//
	{
		reconcileRequest := aws.ReconcileRequest[routetables.Spec]{
			Resource:    awsCluster,
			ClusterName: awsCluster.Name,
			CloudResourceRequest: aws.CloudResourceRequest[routetables.Spec]{
				RoleARN:        roleArn,
				Region:         awsCluster.Spec.Region,
				AdditionalTags: awsCluster.Spec.AdditionalTags,
				Spec: routetables.Spec{
					VpcId: awsCluster.Spec.NetworkSpec.VPC.ID,
				},
			},
		}
		for _, awsSubnetSpec := range awsCluster.Spec.NetworkSpec.Subnets {
			reconcileRequest.Spec.Subnets = append(reconcileRequest.Spec.Subnets, routetables.Subnet{
				Id:               awsSubnetSpec.ID,
				AvailabilityZone: awsSubnetSpec.AvailabilityZone,
			})
		}

		result, err := r.routeTablesReconciler.Reconcile(ctx, reconcileRequest)
		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}

		allRouteTablesReady = true
		allRouteTablesNotReadyMessage = ""
		allRouteTablesNotReadyReason = ""
		for _, routeTableStatus := range result.Status {
			for _, association := range routeTableStatus.RouteTableAssociation {
				if association.AssociationStateCode != routetables.AssociationStateCodeAssociated {
					allRouteTablesReady = false
					allRouteTablesNotReadyMessage = fmt.Sprintf("Route table %s for subnet %s not ready", routeTableStatus.RouteTableId, association.SubnetId)
					// e.g. RouteTableAssociationStateAssociating
					allRouteTablesNotReadyReason = "RouteTableAssociationState" + cases.Title(language.English).String(string(association.AssociationStateCode))
					break
				}
			}
		}

		if allRouteTablesReady {
			// route tables are not ready
			conditions.MarkTrue(awsCluster, capa.RouteTablesReadyCondition)
		} else {
			// route tables are not ready, so we requeue, let's just check how
			// much we should wait before retrying
			routeTablesReadyConditionSeverity := conditions.GetSeverity(awsCluster, capa.RouteTablesReadyCondition)
			routeTablesReadyConditionSeverityInfo := routeTablesReadyConditionSeverity != nil && *routeTablesReadyConditionSeverity == capi.ConditionSeverityInfo

			routeTablesReadyConditionLastChange := conditions.GetLastTransitionTime(awsCluster, capa.RouteTablesReadyCondition)
			timeSinceLastChange := 0 * time.Minute
			if routeTablesReadyConditionLastChange != nil {
				timeSinceLastChange = time.Since(routeTablesReadyConditionLastChange.Time)
			}

			// initially retry more often, but then back off, so we don't hammer the API server
			if timeSinceLastChange < 5*time.Minute && routeTablesReadyConditionSeverityInfo {
				// it's been less than 5 minutes, all good, just an info here,
				// let's try again in a minute
				conditions.MarkFalse(awsCluster, capa.RouteTablesReadyCondition, allRouteTablesNotReadyReason, capi.ConditionSeverityInfo, allRouteTablesNotReadyMessage)
				return ctrl.Result{RequeueAfter: time.Minute}, nil
			} else if timeSinceLastChange < 15*time.Minute && routeTablesReadyConditionSeverityInfo {
				// it's been less than 15 minutes, all good, just an info here,
				// let just wait a bit longer and try again in 5 minutes
				conditions.MarkFalse(awsCluster, capa.RouteTablesReadyCondition, allRouteTablesNotReadyReason, capi.ConditionSeverityInfo, allRouteTablesNotReadyMessage)
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			} else {
				// it's been more than 15 minutes, it's taking a while, so now
				// it's a warning, and let's try again in 15 minutes
				conditions.MarkFalse(awsCluster, capa.RouteTablesReadyCondition, allRouteTablesNotReadyReason, capi.ConditionSeverityWarning, allRouteTablesNotReadyMessage+" for more than 15 minutes")
				return ctrl.Result{RequeueAfter: 15 * time.Minute}, nil
			}
		}
	}

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
	// Reconcile VPC endpoints
	//
	if !capiconditions.IsTrue(awsCluster, capa.ClusterSecurityGroupsReadyCondition) {
		// Security groups are still not ready, we wait for them first
		capiconditions.MarkFalse(awsCluster, VpcEndpointReady, ClusterSecurityGroupsNotReady, capi.ConditionSeverityWarning, "Security groups are still not ready")
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	} else {
		reconcileRequest := aws.ReconcileRequest[vpcendpoint.Spec]{
			Resource:    awsCluster,
			ClusterName: awsCluster.Name,
			CloudResourceRequest: aws.CloudResourceRequest[vpcendpoint.Spec]{
				RoleARN: roleArn,
				Region:  awsCluster.Spec.Region,
				Spec: vpcendpoint.Spec{
					VpcId: awsCluster.Spec.NetworkSpec.VPC.ID,
				},
				AdditionalTags: awsCluster.Spec.AdditionalTags,
			},
		}

		subnetIDs, err := r.subnetsClient.GetEndpointSubnets(ctx, subnets.GetEndpointSubnetsInput{
			ClusterName: awsCluster.Name,
			RoleARN:     roleArn,
			Region:      awsCluster.Spec.Region,
		})
		if err != nil {
			log.Error(err, "Failed to lookup subnets")
			capiconditions.MarkFalse(awsCluster, VpcEndpointReady, SubnetLookupFailed, capi.ConditionSeverityWarning, "Failed to lookup subnets to use for VPC Endpoints")
			return ctrl.Result{}, err
		}
		// If no specific subnets found we'll fallback to picking any from the cluster
		if len(subnetIDs) == 0 {
			log.Info("No specific subnets found for VPC endpoints, falling back to using subnets from AWSCluster spec")
			selectedAZs := map[string]bool{}
			for _, subnet := range awsCluster.Spec.NetworkSpec.Subnets {
				if selectedAZs[subnet.AvailabilityZone] {
					// VPC endpoints can only have a single subnet per AZ so we'll skip any additional
					continue
				}
				subnetIDs = append(reconcileRequest.Spec.SubnetIds, subnet.ID)
				selectedAZs[subnet.AvailabilityZone] = true
			}
		}
		reconcileRequest.Spec.SubnetIds = subnetIDs

		for _, securityGroup := range awsCluster.Status.Network.SecurityGroups {
			reconcileRequest.Spec.SecurityGroupIds = append(reconcileRequest.Spec.SecurityGroupIds, securityGroup.ID)
		}

		routeTablesListOutput, err := r.routeTablesClient.List(ctx, routetables.ListRouteTablesInput{
			Region:  awsCluster.Spec.Region,
			RoleARN: roleArn,
			VpcId:   awsCluster.Spec.NetworkSpec.VPC.ID,
		})
		if err != nil {
			log.Error(err, "Failed to lookup route tables")
			capiconditions.MarkFalse(awsCluster, VpcEndpointReady, SubnetLookupFailed, capi.ConditionSeverityWarning, "Failed to lookup route tables to use for VPC Endpoints")
			return ctrl.Result{}, err
		}

		for _, rt := range routeTablesListOutput {
			reconcileRequest.Spec.RouteTableIds = append(reconcileRequest.Spec.RouteTableIds, rt.RouteTableId)
		}

		result, err := r.vpcEndpointReconciler.Reconcile(ctx, reconcileRequest)
		if err != nil {
			capiconditions.MarkFalse(awsCluster, VpcEndpointReady, "ReconciliationError", capi.ConditionSeverityError, "An error occurred during reconciliation, check logs")
			return ctrl.Result{}, microerror.Mask(err)
		}

		if strings.EqualFold(result.Status.VpcEndpointState, vpcendpoint.StateAvailable) {
			capiconditions.MarkTrue(awsCluster, VpcEndpointReady)
		} else {
			reason := fmt.Sprintf("VpcEndpointState%s", result.Status.VpcEndpointState)
			capiconditions.MarkFalse(awsCluster, VpcEndpointReady, reason, capi.ConditionSeverityWarning, "VPC endpoint is not available")
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *AWSClusterReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, awsCluster *capa.AWSCluster, roleArn string) (_ ctrl.Result, err error) {
	//
	// Delete VPC endpoint. We delete VPC endpoint first, regardless of what CAPA
	// deleted (if anything) until now.
	//
	if isDeleted(awsCluster, VpcEndpointReady) {
		logger.Info("VPC endpoint is already deleted")
	} else {
		vpcId := awsCluster.Spec.NetworkSpec.VPC.ID
		logger.Info("Deleting VPC endpoint", "vpc-id", vpcId)
		vpcEndpointDeleteRequest := aws.ReconcileRequest[aws.DeletedCloudResourceSpec]{
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
		err = r.vpcEndpointReconciler.ReconcileDelete(ctx, vpcEndpointDeleteRequest)
		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
		conditions.MarkFalse(awsCluster, VpcEndpointReady, capi.DeletedReason, capi.ConditionSeverityInfo, "VPC endpoint has been deleted")
		logger.Info("Deleted VPC endpoint", "vpc-id", vpcId)
	}

	//
	// Wait for CAPA to delete load balancer before we delete VPC, subnets and route tables.
	//
	if controllerutil.ContainsFinalizer(awsCluster, capa.ClusterFinalizer) {
		if capiconditions.IsTrue(awsCluster, capa.LoadBalancerReadyCondition) ||
			isBeingDeleted(awsCluster, capa.LoadBalancerReadyCondition) {
			// load balancer deletion did not start, or it is in progress
			logger.Info("Waiting for CAPA to delete load balancer, trying deletion again in a minute")
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		} else if deletionFailed(awsCluster, capa.LoadBalancerReadyCondition) {
			logger.Info("CAPA failed to delete load balancer, trying deletion of route tables, subnets and VPC again in 15 minutes")
			return ctrl.Result{RequeueAfter: 15 * time.Minute}, nil
		}
		logger.Info("CAPA deleted load balancer, proceeding with deletion")

		//
		// Wait for CAPA to delete security groups before we delete VPC, subnets and route tables.
		//
		if capiconditions.IsTrue(awsCluster, capa.ClusterSecurityGroupsReadyCondition) ||
			isBeingDeleted(awsCluster, capa.ClusterSecurityGroupsReadyCondition) {
			// security groups deletion did not start, or it is in progress
			logger.Info("Waiting for CAPA to delete security groups, trying deletion again in a minute")
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		} else if deletionFailed(awsCluster, capa.ClusterSecurityGroupsReadyCondition) {
			logger.Info("CAPA failed to delete security groups, trying deletion of route tables, subnets and VPC again in 15 minutes")
			return ctrl.Result{RequeueAfter: 15 * time.Minute}, nil
		}
		logger.Info("CAPA deleted security groups, proceeding with deletion")
	} else {
		logger.Info("CAPA finalizer already gone, proceeding with deletion")
	}

	//
	// Delete route tables
	//
	if awsCluster.Spec.NetworkSpec.VPC.ID != "" {
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
		conditions.MarkFalse(awsCluster, capa.RouteTablesReadyCondition, capi.DeletingReason, capi.ConditionSeverityInfo, "Route tables are being deleted")
		err = r.routeTablesReconciler.ReconcileDelete(ctx, routeTablesDeleteRequest)
		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
		// remove route table IDs
		for i := range awsCluster.Spec.NetworkSpec.Subnets {
			awsCluster.Spec.NetworkSpec.Subnets[i].RouteTableID = nil
		}
		conditions.MarkFalse(awsCluster, capa.RouteTablesReadyCondition, capi.DeletedReason, capi.ConditionSeverityInfo, "Route tables have been deleted")
		logger.Info("Deleted route tables")
	}

	//
	// Delete subnets
	//
	var subnetsToDelete []string
	for _, subnet := range awsCluster.Spec.NetworkSpec.Subnets {
		if subnet.ID != "" {
			subnetsToDelete = append(subnetsToDelete, subnet.ID)
		}
	}
	if len(subnetsToDelete) > 0 {
		logger.Info("Deleting subnets", "subnet-ids", subnetsToDelete)
		subnetsDeleteRequest := aws.ReconcileRequest[[]aws.DeletedCloudResourceSpec]{
			Resource:    awsCluster,
			ClusterName: awsCluster.Name,
			CloudResourceRequest: aws.CloudResourceRequest[[]aws.DeletedCloudResourceSpec]{
				RoleARN: roleArn,
				Region:  awsCluster.Spec.Region,
			},
		}
		for _, subnetId := range subnetsToDelete {
			deletedSubnetSpec := aws.DeletedCloudResourceSpec{
				Id: subnetId,
			}
			subnetsDeleteRequest.Spec = append(subnetsDeleteRequest.Spec, deletedSubnetSpec)
		}
		conditions.MarkFalse(awsCluster, capa.SubnetsReadyCondition, capi.DeletingReason, capi.ConditionSeverityInfo, "Subnets are being deleted")
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
	if awsCluster.Spec.NetworkSpec.VPC.ID != "" {
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
		conditions.MarkFalse(awsCluster, capa.VpcReadyCondition, capi.DeletingReason, capi.ConditionSeverityInfo, "VPC is being deleted")
		err = r.vpcReconciler.ReconcileDelete(ctx, vpcDeleteRequest)
		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
		conditions.MarkFalse(awsCluster, capa.VpcReadyCondition, capi.DeletedReason, capi.ConditionSeverityInfo, "VPC has been deleted")
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
