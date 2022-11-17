package controllers

import (
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
)

const (
	// deletionFailedReason is hard-coded in CAPA as a string literal, there is
	// no const that we can reuse in our operator
	deletionFailedReason = "DeletingFailed"
)

func isBeingDeleted(object capiconditions.Getter, condition capi.ConditionType) bool {
	return capiconditions.IsFalse(object, condition) &&
		capiconditions.GetReason(object, condition) == capi.DeletingReason
}

func isDeleted(object capiconditions.Getter, condition capi.ConditionType) bool {
	return capiconditions.IsFalse(object, condition) &&
		capiconditions.GetReason(object, condition) == capi.DeletedReason
}

func deletionFailed(object capiconditions.Getter, condition capi.ConditionType) bool {
	return capiconditions.IsFalse(object, condition) &&
		capiconditions.GetReason(object, condition) == deletionFailedReason
}
