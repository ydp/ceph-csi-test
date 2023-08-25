package ceph_csi

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

func createNamespace(c kubernetes.Interface, name string) error {
	timeout := time.Duration(deployTimeout) * time.Minute
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	ctx := context.TODO()
	_, err := c.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !apierrs.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	return wait.PollUntilContextTimeout(ctx, poll, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := c.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Error getting namespace: '%s': %v", name, err)
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			if isRetryableAPIError(err) {
				return false, nil
			}

			return false, err
		}

		return true, nil
	})
}

func deleteNamespace(c kubernetes.Interface, name string) error {
	timeout := time.Duration(deployTimeout) * time.Minute
	ctx := context.TODO()
	err := c.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrs.IsNotFound(err) {
		return fmt.Errorf("failed to delete namespace: %w", err)
	}

	return wait.PollUntilContextTimeout(ctx, poll, timeout, true, func(ctx context.Context) (bool, error) {
		_, err = c.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return true, nil
			}
			framework.Logf("Error getting namespace: '%s': %v", name, err)
			if isRetryableAPIError(err) {
				return false, nil
			}

			return false, err
		}

		return false, nil
	})
}

func replaceNamespaceInTemplate(filePath string) (string, error) {
	read, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	return strings.ReplaceAll(string(read), "namespace: default", fmt.Sprintf("namespace: %s", cephCSINamespace)), nil
}
