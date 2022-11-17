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
