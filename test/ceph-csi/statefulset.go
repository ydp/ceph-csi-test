package ceph_csi

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

func createStatefulset(path string, timeout int, f *framework.Framework) (*v1.StatefulSet, error) {
	app := &v1.StatefulSet{}
	if err := unmarshal(path, app); err != nil {
		return nil, err
	}

	app, err := f.ClientSet.AppsV1().StatefulSets(f.UniqueName).Create(context.TODO(), app, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create statefulset: %w", err)
	}

	err = waitForStatefulsetReady(app.Name, f.UniqueName, f.ClientSet, timeout, noError)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for statefulset ready: %w", err)
	}
	return app, nil
}

func waitForStatefulsetReady(name, ns string, c kubernetes.Interface, t int, expectedError string) error {
	timeout := time.Duration(t) * time.Minute
	framework.Logf("Waiting up to %v to be in Running state", name)

	return wait.PollUntilContextTimeout(context.TODO(), poll, timeout, true, func(ctx context.Context) (bool, error) {
		sfs, err := c.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if isRetryableAPIError(err) {
				return false, nil
			}

			return false, fmt.Errorf("failed to get statefulset: %w", err)
		}

		if sfs.Status.ReadyReplicas == *sfs.Spec.Replicas {
			opts := metav1.ListOptions{
				LabelSelector: "app=ceph-csi-nginx",
			}
			podList, err := c.CoreV1().Pods(ns).List(ctx, opts)
			if err != nil {
				if isRetryableAPIError(err) {
					return false, nil
				}

				return false, fmt.Errorf("failed to get statefulset: %w", err)
			}

			if len(podList.Items) != int(*sfs.Spec.Replicas) {
				return false, nil
			}

			for _, po := range podList.Items {
				if po.Status.Phase != coreV1.PodRunning {
					framework.Logf("Pod %s is not in Running state, %s", po.Name, po.Status.Phase)
					return false, nil
				}
			}
			return true, nil
		}

		return false, nil
	})
}

func deleteStatefulset(name, ns string, c kubernetes.Interface, t int) error {
	timeout := time.Duration(t) * time.Minute
	ctx := context.TODO()
	err := c.AppsV1().StatefulSets(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete statefulset: %w", err)
	}
	start := time.Now()
	framework.Logf("Waiting for statefulset %v to be deleted", name)

	return wait.PollUntilContextTimeout(ctx, poll, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := c.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if isRetryableAPIError(err) {
				return false, nil
			}
			if apierrs.IsNotFound(err) {
				return true, nil
			}
			framework.Logf("%s statefulset to be deleted (%d seconds elapsed)", name, int(time.Since(start).Seconds()))

			return false, fmt.Errorf("failed to get statefulset: %w", err)
		}

		return false, nil
	})
}
