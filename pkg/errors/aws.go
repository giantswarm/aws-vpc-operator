package errors

import (
	"errors"
	"net/http"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/smithy-go"
	"github.com/giantswarm/microerror"
)

func IsAWSHTTPStatusNotFound(err error) bool {
	var httpResponseError *awshttp.ResponseError
	return errors.As(err, &httpResponseError) && httpResponseError.HTTPStatusCode() == http.StatusNotFound
}

// isAWSVpcNotFound asserts that the specified AWS SDK error means that the VPC
// is not found.
func isAWSVpcNotFound(err error) bool {
	const vpcNotFoundAWSErrorCode = "InvalidVpcID.NotFound"
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == vpcNotFoundAWSErrorCode
}

var VpcNotFoundError = &microerror.Error{
	Kind: "VpcNotFoundError",
}

// IsVpcNotFound asserts that the error is of type VpcNotFoundError or AWS SDK
// InvalidVpcID.NotFound error code.
func IsVpcNotFound(err error) bool {
	return microerror.Cause(err) == VpcNotFoundError ||
		isAWSVpcNotFound(err) ||
		IsAWSHTTPStatusNotFound(err)
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
