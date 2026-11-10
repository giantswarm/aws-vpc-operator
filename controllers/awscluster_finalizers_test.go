package controllers

import (
	"context"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const otherFinalizer = "other-operator.finalizers.giantswarm.io/test"

func newFinalizersTestReconciler(t *testing.T, finalizers ...string) (*AWSClusterReconciler, *capa.AWSCluster) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := capa.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	awsCluster := &capa.AWSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test",
			Namespace:  "default",
			Finalizers: finalizers,
		},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(awsCluster).Build()

	read := &capa.AWSCluster{}
	if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(awsCluster), read); err != nil {
		t.Fatal(err)
	}

	return &AWSClusterReconciler{Client: k8sClient}, read
}

// removeOtherFinalizer simulates another controller removing its finalizer after awsCluster was read.
func removeOtherFinalizer(t *testing.T, k8sClient client.Client, awsCluster *capa.AWSCluster) {
	t.Helper()

	latest := &capa.AWSCluster{}
	if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(awsCluster), latest); err != nil {
		t.Fatal(err)
	}
	var finalizers []string
	for _, f := range latest.Finalizers {
		if f != otherFinalizer {
			finalizers = append(finalizers, f)
		}
	}
	latest.Finalizers = finalizers
	if err := k8sClient.Update(context.Background(), latest); err != nil {
		t.Fatal(err)
	}
}

func getFinalizers(t *testing.T, k8sClient client.Client, awsCluster *capa.AWSCluster) []string {
	t.Helper()

	actual := &capa.AWSCluster{}
	if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(awsCluster), actual); err != nil {
		t.Fatal(err)
	}
	return actual.Finalizers
}

func TestPatchFinalizers(t *testing.T) {
	ctx := context.Background()

	t.Run("adds the finalizer", func(t *testing.T) {
		r, awsCluster := newFinalizersTestReconciler(t, otherFinalizer)

		patched, err := r.patchFinalizers(ctx, awsCluster, []string{otherFinalizer, AwsVpcOperatorFinalizer})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := patched.GetFinalizers(); len(got) != 2 {
			t.Errorf("expected the patched object to have both finalizers, got %v", got)
		}
		if got := getFinalizers(t, r.Client, awsCluster); len(got) != 2 {
			t.Errorf("expected both finalizers, got %v", got)
		}
	})

	t.Run("adding returns a conflict when another controller removed its finalizer after the object was read", func(t *testing.T) {
		r, awsCluster := newFinalizersTestReconciler(t, otherFinalizer)
		removeOtherFinalizer(t, r.Client, awsCluster)

		_, err := r.patchFinalizers(ctx, awsCluster, []string{otherFinalizer, AwsVpcOperatorFinalizer})
		if !apierrors.IsConflict(err) {
			t.Fatalf("expected a conflict, got %v", err)
		}
		if got := getFinalizers(t, r.Client, awsCluster); len(got) != 0 {
			t.Errorf("expected no finalizers, got %v", got)
		}
	})

	t.Run("removing returns a conflict when another controller removed its finalizer after the object was read", func(t *testing.T) {
		r, awsCluster := newFinalizersTestReconciler(t, AwsVpcOperatorFinalizer, otherFinalizer)
		removeOtherFinalizer(t, r.Client, awsCluster)

		_, err := r.patchFinalizers(ctx, awsCluster, []string{otherFinalizer})
		if !apierrors.IsConflict(err) {
			t.Fatalf("expected a conflict, got %v", err)
		}
		if got := getFinalizers(t, r.Client, awsCluster); len(got) != 1 || got[0] != AwsVpcOperatorFinalizer {
			t.Errorf("expected only %s, got %v", AwsVpcOperatorFinalizer, got)
		}
	})

	t.Run("removing the last finalizer of an AWSCluster that is being deleted deletes it", func(t *testing.T) {
		r, awsCluster := newFinalizersTestReconciler(t, AwsVpcOperatorFinalizer)
		if err := r.Delete(ctx, awsCluster); err != nil {
			t.Fatal(err)
		}
		if err := r.Get(ctx, client.ObjectKeyFromObject(awsCluster), awsCluster); err != nil {
			t.Fatal(err)
		}

		patched, err := r.patchFinalizers(ctx, awsCluster, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if patched.DeletionTimestamp.IsZero() || len(patched.GetFinalizers()) != 0 {
			t.Errorf("expected a deleted object without finalizers, got deletionTimestamp %v and finalizers %v", patched.DeletionTimestamp, patched.GetFinalizers())
		}
		err = r.Get(ctx, client.ObjectKeyFromObject(awsCluster), &capa.AWSCluster{})
		if !apierrors.IsNotFound(err) {
			t.Errorf("expected the AWSCluster to be gone, got %v", err)
		}
	})
}
