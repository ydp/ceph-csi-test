package ceph_csi

import (
	"context"
	"fmt"
	"time"

	snapapi "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	snapclient "github.com/kubernetes-csi/external-snapshotter/client/v6/clientset/versioned/typed/volumesnapshot/v1"
	. "github.com/onsi/gomega"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
)

func getSnapshotClass(path string) snapapi.VolumeSnapshotClass {
	sc := snapapi.VolumeSnapshotClass{}
	err := unmarshal(path, &sc)
	Expect(err).ShouldNot(HaveOccurred())

	return sc
}

func createRBDSnapshotClass(f *framework.Framework) error {
	scPath := "manifest/rbd/snapshotclass.yaml"
	sc := getSnapshotClass(scPath)

	sclient, err := newSnapshotClient()
	if err != nil {
		return err
	}
	_, err = sclient.VolumeSnapshotClasses().Create(context.TODO(), &sc, metav1.CreateOptions{})

	return err
}

func deleteRBDSnapshotClass() error {
	scPath := "manifest/rbd/snapshotclass.yaml"
	sc := getSnapshotClass(scPath)

	sclient, err := newSnapshotClient()
	if err != nil {
		return err
	}

	return sclient.VolumeSnapshotClasses().Delete(context.TODO(), sc.Name, metav1.DeleteOptions{})
}

func getSnapshot(path string) snapapi.VolumeSnapshot {
	sc := snapapi.VolumeSnapshot{}
	err := unmarshal(path, &sc)
	Expect(err).ShouldNot(HaveOccurred())

	return sc
}

func newSnapshotClient() (*snapclient.SnapshotV1Client, error) {
	config, err := framework.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("error creating client: %w", err)
	}
	c, err := snapclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating snapshot client: %w", err)
	}

	return c, err
}

func createSnapshot(snap *snapapi.VolumeSnapshot, t int) error {
	sclient, err := newSnapshotClient()
	if err != nil {
		return err
	}

	ctx := context.TODO()
	_, err = sclient.
		VolumeSnapshots(snap.Namespace).
		Create(ctx, snap, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create volumesnapshot: %w", err)
	}
	framework.Logf("snapshot with name %v created in %v namespace", snap.Name, snap.Namespace)

	timeout := time.Duration(t) * time.Minute
	name := snap.Name
	start := time.Now()
	framework.Logf("waiting for %v to be in ready state", snap)

	return wait.PollUntilContextTimeout(ctx, poll, timeout, true, func(ctx context.Context) (bool, error) {
		framework.Logf("waiting for snapshot %s (%d seconds elapsed)", snap.Name, int(time.Since(start).Seconds()))
		snaps, err := sclient.
			VolumeSnapshots(snap.Namespace).
			Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Error getting snapshot in namespace: '%s': %v", snap.Namespace, err)
			if isRetryableAPIError(err) {
				return false, nil
			}
			if apierrs.IsNotFound(err) {
				return false, nil
			}

			return false, fmt.Errorf("failed to get volumesnapshot: %w", err)
		}
		if snaps.Status == nil || snaps.Status.ReadyToUse == nil {
			return false, nil
		}
		if *snaps.Status.ReadyToUse {
			return true, nil
		}
		framework.Logf("snapshot %s in %v state", snap.Name, *snaps.Status.ReadyToUse)

		return false, nil
	})
}

func deleteSnapshot(snap *snapapi.VolumeSnapshot, t int) error {
	sclient, err := newSnapshotClient()
	if err != nil {
		return err
	}

	ctx := context.TODO()
	err = sclient.
		VolumeSnapshots(snap.Namespace).
		Delete(ctx, snap.Name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete volumesnapshot: %w", err)
	}

	timeout := time.Duration(t) * time.Minute
	name := snap.Name
	start := time.Now()
	framework.Logf("Waiting up to %v to be deleted", snap)

	return wait.PollUntilContextTimeout(ctx, poll, timeout, true, func(ctx context.Context) (bool, error) {
		framework.Logf("deleting snapshot %s (%d seconds elapsed)", name, int(time.Since(start).Seconds()))
		_, err := sclient.
			VolumeSnapshots(snap.Namespace).
			Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			return false, nil
		}

		if isRetryableAPIError(err) {
			return false, nil
		}
		if !apierrs.IsNotFound(err) {
			return false, fmt.Errorf(
				"get on deleted snapshot %v failed : other than \"not found\": %w",
				name,
				err)
		}

		return true, nil
	})
}

func createCephfsSnapshotClass(f *framework.Framework) error {
	scPath := "manifest/cephfs/snapshotclass.yaml"
	sc := getSnapshotClass(scPath)

	sclient, err := newSnapshotClient()
	if err != nil {
		return err
	}
	_, err = sclient.VolumeSnapshotClasses().Create(context.TODO(), &sc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create volumesnapshotclass: %w", err)
	}

	return err
}

func deleteCephfsSnapshotClass() error {
	scPath := "manifest/cephfs/snapshotclass.yaml"
	sc := getSnapshotClass(scPath)

	sclient, err := newSnapshotClient()
	if err != nil {
		return err
	}

	return sclient.VolumeSnapshotClasses().Delete(context.TODO(), sc.Name, metav1.DeleteOptions{})
}
