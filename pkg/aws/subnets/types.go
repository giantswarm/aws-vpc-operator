package subnets

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
