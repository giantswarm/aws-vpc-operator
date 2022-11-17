package errors

import (
	"github.com/giantswarm/microerror"
)

var VpcNotFoundError = &microerror.Error{
	Kind: "VpcNotFoundError",
}

// IsVpcNotFound asserts VpcNotFoundError.
func IsVpcNotFound(err error) bool {
	return microerror.Cause(err) == VpcNotFoundError
}

var VpcConflictError = &microerror.Error{
	Kind: "VpcConflictError",
}

// IsVpcConflict asserts VpcConflictError.
func IsVpcConflict(err error) bool {
	return microerror.Cause(err) == VpcConflictError
}

var VpcStateUnknownError = &microerror.Error{
	Kind: "VpcStateUnknown",
}

// IsVpcStateUnknown asserts VpcStateUnknownError.
func IsVpcStateUnknown(err error) bool {
	return microerror.Cause(err) == VpcStateUnknownError
}

var VpcStateNotSetError = &microerror.Error{
	Kind: "VpcStateNotSet",
}

// IsVpcStateNotSet asserts VpcStateNotSetError.
func IsVpcStateNotSet(err error) bool {
	return microerror.Cause(err) == VpcStateNotSetError
}

var RouteTableIdNotSetError = &microerror.Error{
	Kind: "RouteTableIdNotSet",
}

// IsRouteTableIdNotSet asserts RouteTableIdNotSetError.
func IsRouteTableIdNotSet(err error) bool {
	return microerror.Cause(err) == RouteTableIdNotSetError
}

var RouteTableNotFoundError = &microerror.Error{
	Kind: "RouteTableNotFoundError",
}

// IsRouteTableNotFound asserts RouteTableNotFoundError.
func IsRouteTableNotFound(err error) bool {
	return microerror.Cause(err) == RouteTableNotFoundError
}

var UnknownVpcAttributeError = &microerror.Error{
	Kind: "UnknownVpcAttribute",
}

// IsUnknownVpcAttribute asserts UnknownVpcAttributeError.
func IsUnknownVpcAttribute(err error) bool {
	return microerror.Cause(err) == UnknownVpcAttributeError
}

var VpcEndpointNotFoundError = &microerror.Error{
	Kind: "VpcEndpointNotFoundError",
}

// IsVpcEndpointNotFound asserts VpcEndpointNotFoundError.
func IsVpcEndpointNotFound(err error) bool {
	return microerror.Cause(err) == VpcEndpointNotFoundError
}

var ResourceDeletionInProgressError = &microerror.Error{
	Kind: "ResourceDeletionInProgressError",
}

// IsResourceDeletionInProgress asserts ResourceDeletionInProgressError.
func IsResourceDeletionInProgress(err error) bool {
	return microerror.Cause(err) == ResourceDeletionInProgressError
}

var ResourceAlreadyDeletedError = &microerror.Error{
	Kind: "ResourceAlreadyDeletedError",
}

// IsResourceAlreadyDeleted asserts ResourceAlreadyDeletedError.
func IsResourceAlreadyDeleted(err error) bool {
	return microerror.Cause(err) == ResourceAlreadyDeletedError
}
