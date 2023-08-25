package ceph_csi

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"
)

func createPVC(path string, f *framework.Framework) (*v1.PersistentVolumeClaim, error) {
	pvc := &v1.PersistentVolumeClaim{}
	err := unmarshal(path, &pvc)
	if err != nil {
		return nil, err
	}
	pvc.Namespace = f.UniqueName

	err = createPVCAndvalidatePV(f.ClientSet, pvc, deployTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create pvc: %w", err)
	}

	return pvc, nil
}

func createPVCAndvalidatePV(c kubernetes.Interface, pvc *v1.PersistentVolumeClaim, t int) error {
	timeout := time.Duration(t) * time.Minute
	ctx := context.TODO()
	pv := &v1.PersistentVolume{}
	var err error
	_, err = c.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create pvc: %w", err)
	}
	if timeout == 0 {
		return nil
	}
	name := pvc.Name
	namespace := pvc.Namespace
	start := time.Now()
	framework.Logf("Waiting up to %v to be in Bound state", pvc)

	return wait.PollUntilContextTimeout(ctx, poll, timeout, true, func(ctx context.Context) (bool, error) {
		framework.Logf("waiting for PVC %s (%d seconds elapsed)", name, int(time.Since(start).Seconds()))
		pvc, err = c.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Error getting pvc %q in namespace %q: %v", name, namespace, err)
			if isRetryableAPIError(err) {
				return false, nil
			}
			if apierrs.IsNotFound(err) {
				return false, nil
			}

			return false, fmt.Errorf("failed to get pvc: %w", err)
		}

		if pvc.Spec.VolumeName == "" {
			return false, nil
		}

		pv, err = c.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
		if err != nil {
			if isRetryableAPIError(err) {
				return false, nil
			}
			if apierrs.IsNotFound(err) {
				return false, nil
			}

			return false, fmt.Errorf("failed to get pv: %w", err)
		}
		err = e2epv.WaitOnPVandPVC(
			ctx,
			c,
			&framework.TimeoutContext{ClaimBound: timeout, PVBound: timeout},
			namespace,
			pv,
			pvc)
		if err != nil {
			return false, fmt.Errorf("failed to wait for the pv and pvc to bind: %w", err)
		}

		return true, nil
	})
}

func expandPVC(c kubernetes.Interface, pvc *v1.PersistentVolumeClaim, t int) error {
	patchBytes := []byte(`{"spec":{"resources":{"requests":{"storage":"2Gi"}}}}`)
	_, err := c.CoreV1().PersistentVolumeClaims(pvc.Namespace).Patch(
		context.TODO(),
		pvc.Name,
		types.StrategicMergePatchType,
		patchBytes,
		metav1.PatchOptions{})
	if err != nil {
		framework.Failf("failed to patch PVC with larger size: %v", err)
	}

	timeout := time.Duration(t) * time.Minute
	ctx := context.Background()
	start := time.Now()

	return wait.PollUntilContextTimeout(ctx, poll, timeout, false, func(ctx context.Context) (bool, error) {
		framework.Logf("waiting for PVC %s (%d seconds elapsed)", pvc.Name, int(time.Since(start).Seconds()))
		pvc, err = c.CoreV1().PersistentVolumeClaims(pvc.Namespace).
			Get(ctx, pvc.Name, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Error getting pvc %q in namespace %q: %v", pvc.Name, pvc.Namespace, err)
			if isRetryableAPIError(err) {
				return false, nil
			}
			if apierrs.IsNotFound(err) {
				return false, nil
			}

			return false, fmt.Errorf("failed to get pvc: %w", err)
		}

		if pvc.Spec.VolumeName == "" {
			return false, nil
		}

		if pvc.Status.Phase != v1.ClaimBound {
			return false, nil
		}

		for _, cond := range pvc.Status.Conditions {
			framework.Logf("pvc status: %s conditions: %s", pvc.Status.Phase, cond.Type)
			if cond.Type == v1.PersistentVolumeClaimFileSystemResizePending {
				return false, nil
			}
		}

		return true, nil
	})
}

// getPersistentVolumeClaim returns the PersistentVolumeClaim with the given
// name in the given namespace and retries if there is any API error.
func getPersistentVolumeClaim(c kubernetes.Interface, namespace, name string) (*v1.PersistentVolumeClaim, error) {
	var pvc *v1.PersistentVolumeClaim
	var err error
	timeout := time.Duration(deployTimeout) * time.Minute
	err = wait.PollUntilContextTimeout(
		context.TODO(),
		1*time.Second,
		timeout,
		true,
		func(ctx context.Context) (bool, error) {
			pvc, err = c.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting pvc %q in namespace %q: %v", name, namespace, err)
				if isRetryableAPIError(err) {
					return false, nil
				}

				return false, fmt.Errorf("failed to get pvc: %w", err)
			}

			return true, err
		})

	return pvc, err
}

// getPersistentVolume returns the PersistentVolume with the given
// name and retries if there is any API error.
func getPersistentVolume(c kubernetes.Interface, name string) (*v1.PersistentVolume, error) {
	var pv *v1.PersistentVolume
	var err error
	timeout := time.Duration(deployTimeout) * time.Minute
	err = wait.PollUntilContextTimeout(
		context.TODO(),
		1*time.Second,
		timeout,
		true,
		func(ctx context.Context) (bool, error) {
			pv, err = c.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting pv %q: %v", name, err)
				if isRetryableAPIError(err) {
					return false, nil
				}

				return false, fmt.Errorf("failed to get pv: %w", err)
			}

			return true, err
		})

	return pv, err
}

func deletePVCAndValidatePV(c kubernetes.Interface, pvc *v1.PersistentVolumeClaim, t int) error {
	timeout := time.Duration(t) * time.Minute
	nameSpace := pvc.Namespace
	name := pvc.Name
	ctx := context.TODO()
	var err error
	framework.Logf("Deleting PersistentVolumeClaim %v on namespace %v", name, nameSpace)

	pvc, err = getPersistentVolumeClaim(c, nameSpace, name)
	if err != nil {
		return fmt.Errorf("failed to get pvc: %w", err)
	}
	pv, err := getPersistentVolume(c, pvc.Spec.VolumeName)
	if err != nil {
		return fmt.Errorf("failed to get pv: %w", err)
	}

	err = c.CoreV1().PersistentVolumeClaims(nameSpace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("delete of PVC %v failed: %w", name, err)
	}
	start := time.Now()

	return wait.PollUntilContextTimeout(ctx, poll, timeout, true, func(ctx context.Context) (bool, error) {
		// Check that the PVC is really deleted.
		framework.Logf(
			"waiting for PVC %s in state %s to be deleted (%d seconds elapsed)",
			name,
			pvc.Status.String(),
			int(time.Since(start).Seconds()))
		pvc, err = c.CoreV1().PersistentVolumeClaims(nameSpace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			framework.Logf("PVC %s (status: %s) has not been deleted yet, rechecking...", name, pvc.Status)

			return false, nil
		}
		if isRetryableAPIError(err) {
			framework.Logf("failed to verify deletion of PVC %s (status: %s): %v", name, pvc.Status, err)

			return false, nil
		}
		if !apierrs.IsNotFound(err) {
			return false, fmt.Errorf("get on deleted PVC %v failed with error other than \"not found\": %w", name, err)
		}

		// Examine the pv.ClaimRef and UID. Expect nil values.
		oldPV, err := c.CoreV1().PersistentVolumes().Get(ctx, pv.Name, metav1.GetOptions{})
		if err == nil {
			framework.Logf("PV %s (status: %s) has not been deleted yet, rechecking...", pv.Name, oldPV.Status)

			return false, nil
		}
		if isRetryableAPIError(err) {
			framework.Logf("failed to verify deletion of PV %s (status: %s): %v", pv.Name, oldPV.Status, err)

			return false, nil
		}
		if !apierrs.IsNotFound(err) {
			return false, fmt.Errorf("delete PV %v failed with error other than \"not found\": %w", pv.Name, err)
		}

		return true, nil
	})
}
