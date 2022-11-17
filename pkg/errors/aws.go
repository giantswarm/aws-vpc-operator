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
