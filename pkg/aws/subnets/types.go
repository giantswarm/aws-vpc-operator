package subnets

import (
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type AssociationStateCode string

// Enum values for RouteTableAssociationStateCode
const (
	AssociationStateCodeAssociating    AssociationStateCode = "associating"
	AssociationStateCodeAssociated     AssociationStateCode = "associated"
	AssociationStateCodeDisassociating AssociationStateCode = "disassociating"
	AssociationStateCodeDisassociated  AssociationStateCode = "disassociated"
	AssociationStateCodeFailed         AssociationStateCode = "failed"
	AssociationStateCodeUnknown        AssociationStateCode = "unknown"
)

type RouteTableAssociation struct {
	RouteTableId         string
	AssociationStateCode AssociationStateCode
}

func getAssociationStateCode(ec2AssociationState *ec2Types.RouteTableAssociationState) AssociationStateCode {
	if ec2AssociationState != nil {
		return AssociationStateCode(ec2AssociationState.State)
	} else {
		return AssociationStateCodeUnknown
	}
}
