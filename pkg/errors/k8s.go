package errors

import (
	"github.com/giantswarm/microerror"
)

var IdentityNotSetError = &microerror.Error{
	Kind: "IdentityNotSetError",
}

// IsIdentityNotSet asserts IdentityNotSetError.
func IsIdentityNotSet(err error) bool {
	return microerror.Cause(err) == IdentityNotSetError
}
