package routetables

import (
	"context"
	"fmt"
	"strings"

	"github.com/giantswarm/microerror"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capaservices "sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/aws-vpc-operator/pkg/aws"
	"github.com/giantswarm/aws-vpc-operator/pkg/aws/tags"
)

func (r *reconciler) Reconcile(ctx context.Context, request aws.ReconcileRequest[Spec]) (result aws.ReconcileResult[[]Status], err error) {
	logger := log.FromContext(ctx)
	logger.Info("Started reconciling route tables")
	defer func() {
		if err == nil {
			logger.Info("Finished reconciling route tables")
		} else {
			logger.Error(err, "Failed to reconcile route tables")
		}
	}()

	result = aws.ReconcileResult[[]Status]{
		Status: []Status{},
	}

	// 1. We can split desired subnets (from request) into two categories:
	//   a. with associated route table - this is the desired end state
	//   b. without associated route table - we create route tables for these

	// 2. We can also categorize all discovered route tables:
	//   a. those associated to desired subnets - this is the desired end state,
	//      we just update these if necessary (e.g. outdated tags)
	//   b. those associated to other subnets - these could be subnets that are
	//      removed from AWSCluster, or subnets created by some other operator,
	//      in any case we ignore these here.
	//   c. route tables not associated to any subnet - these could be leftover
	//      route tables, previously created by aws-vpc-operator, in which case
	//      we delete them, or externally created route tables, in which case
	//      we ignore them

	// subnet -> route table map, i.e. subnets with already associated route
	// tables
	subnetToRouteTable := map[string]*GetRouteTableOutput{}
	for _, subnet := range request.Spec.Subnets {
		subnetToRouteTable[subnet.Id] = nil
	}

	// subnets without associated route tables, so we create new route tables
	// for these
	var subnetsWithoutRouteTables []Subnet

	// route tables not associated to any subnet - we delete these in case we
	// created them
	routeTablesWithoutSubnets := map[string]GetRouteTableOutput{}

	//
	// Get route tables for specified subnets
	//
	listRouteTablesInput := ListRouteTablesInput{
		RoleARN: request.RoleARN,
		Region:  request.Region,
		VpcId:   request.Spec.VpcId,
	}
	existingRouteTables, err := r.client.List(ctx, listRouteTablesInput)
	if err != nil {
		return aws.ReconcileResult[[]Status]{}, microerror.Mask(err)
	}

	// let's categorize the existing route tables
	for _, existingRouteTable := range existingRouteTables {
		isAssociatedToDesiredSubnet := false
		isAssociatedToExternalSubnet := false
		for _, associatedSubnet := range existingRouteTable.AssociatedSubnets {
			if _, isDesiredSubnet := subnetToRouteTable[associatedSubnet.SubnetId]; isDesiredSubnet {
				// see 1.a and 2.a above
				routeTable := existingRouteTable
				subnetToRouteTable[associatedSubnet.SubnetId] = &routeTable
				isAssociatedToDesiredSubnet = true
			} else {
				// see 2.b above, this route table is associated to a subnet
				// that is not in AWSCluster spec
				isAssociatedToExternalSubnet = true
			}
		}

		if isAssociatedToDesiredSubnet || isAssociatedToExternalSubnet {
			// isAssociatedToDesiredSubnet is desired state
			// isAssociatedToExternalSubnet is ignored
			continue
		}

		// see 2.c above, this route table is not associate to neither desired
		// nor external subnet
		routeTablesWithoutSubnets[existingRouteTable.RouteTableId] = existingRouteTable
	}

	//
	// first, let's delete leftover route tables (just those that this operator
	// created, see 2.c above)
	//
	for routeTableId, routeTable := range routeTablesWithoutSubnets {
		createdByThisOperator := false
		for tagName := range routeTable.Tags {
			if strings.HasPrefix(tagName, tags.NameAWSProviderPrefix) {
				createdByThisOperator = true
				break
			}
		}

		if createdByThisOperator {
			input := DeleteRouteTableInput{
				RoleARN:      request.RoleARN,
				Region:       request.Region,
				RouteTableId: routeTableId,
			}
			err = r.client.Delete(ctx, input)
			if err != nil {
				return aws.ReconcileResult[[]Status]{}, microerror.Mask(err)
			}
		}
	}

	//
	// now let's update outdated route tables (see 2.a above)
	//
	for subnetId, routeTable := range subnetToRouteTable {
		changedOrNewTags := tags.Diff(routeTable.Tags, request.AdditionalTags)
		if len(changedOrNewTags) > 0 {
			var zone string
			for _, subnet := range request.Spec.Subnets {
				if subnet.Id == subnetId {
					zone = subnet.AvailabilityZone
					break
				}
			}

			input := UpdateRouteTableInput{
				RoleARN:      request.RoleARN,
				Region:       request.Region,
				RouteTableId: routeTable.RouteTableId,
				Tags:         r.getRouteTableTags(request.ClusterName, routeTable.RouteTableId, zone, request.AdditionalTags),
			}
			err = r.client.Update(ctx, input)
			if err != nil {
				return aws.ReconcileResult[[]Status]{}, microerror.Mask(err)
			}
		}

		routeTableStatus := Status{
			RouteTableId:          routeTable.RouteTableId,
			RouteTableAssociation: routeTable.AssociatedSubnets,
		}
		result.Status = append(result.Status, routeTableStatus)
	}

	//
	// Now, let's create route tables for those subnets that do not have them
	// (see 1.b above)
	//
	for _, subnet := range request.Spec.Subnets {
		routeTable := subnetToRouteTable[subnet.Id]
		if routeTable == nil {
			subnetsWithoutRouteTables = append(subnetsWithoutRouteTables, subnet)
		}
	}
	for _, subnet := range subnetsWithoutRouteTables {
		input := CreateRouteTableInput{
			RoleARN:  request.RoleARN,
			Region:   request.Region,
			VpcId:    request.Spec.VpcId,
			SubnetId: subnet.Id,
			Tags:     r.getRouteTableTags(request.ClusterName, "", subnet.AvailabilityZone, request.AdditionalTags),
		}
		output, err := r.client.Create(ctx, input)
		if err != nil {
			return aws.ReconcileResult[[]Status]{}, microerror.Mask(err)
		}

		routeTableStatus := Status{
			RouteTableId: output.RouteTableId,
			RouteTableAssociation: []RouteTableAssociation{
				{
					SubnetId:             subnet.Id,
					AssociationStateCode: output.AssociationStateCode,
				},
			},
		}
		result.Status = append(result.Status, routeTableStatus)
	}

	return result, nil
}

func (r *reconciler) getRouteTableTags(clusterName, routeTableId, zone string, additionalTags map[string]string) map[string]string {
	if routeTableId == "" {
		routeTableId = capaservices.TemporaryResourceID
	}
	name := fmt.Sprintf("%s-rt-private-%s", clusterName, zone)

	params := tags.BuildParams{
		ClusterName: clusterName,
		ResourceID:  routeTableId,
		Name:        name,
		Role:        capa.PrivateRoleTagValue,
		Additional:  additionalTags,
	}

	return params.Build()
}
