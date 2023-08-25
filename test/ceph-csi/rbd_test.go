package ceph_csi

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/pod-security-admission/api"
)

var (
	defaultRbdSc = "csi-rbd-sc"
)

func rbdOptions(pool string) string {
	if radosNamespace != "" {
		return "--pool=" + pool + " --namespace " + radosNamespace
	}

	return "--pool=" + pool
}

func listRBDImages(f *framework.Framework, pool string) ([]string, error) {
	var imgInfos []string

	stdout, err := exec.Command("rbd", "ls", "--format=json", rbdOptions(pool)).CombinedOutput()
	if err != nil {
		return imgInfos, fmt.Errorf("failed to list images %s, %s", err.Error(), string(stdout))
	}

	err = json.Unmarshal(stdout, &imgInfos)
	if err != nil {
		return imgInfos, err
	}

	return imgInfos, nil
}

func validateRBDImageCount(f *framework.Framework, count int, pool string) {
	imageList, err := listRBDImages(f, pool)
	if err != nil {
		framework.Failf("failed to list rbd images: %v", err)
	}
	if len(imageList) != count {
		framework.Failf(
			"backend images not matching kubernetes resource count,image count %d kubernetes resource count %d"+
				"\nbackend image Info:\n %v",
			len(imageList),
			count,
			imageList)
	}
}

func validateRbdRwoVolume(pvcPath, podPath string, f *framework.Framework) {
	pvc, err := createPVC(pvcPath, f)
	if err != nil {
		framework.Failf("failed to create RBD pvc: %v", err)
	}

	pod, err := createPod(podPath, deployTimeout, f)
	if err != nil {
		framework.Failf("failed to create pod: %v", err)
	}

	validateRBDImageCount(f, 1, defaultRbdPool)

	err = deletePod(pod.Name, pod.Namespace, f.ClientSet, deployTimeout)
	if err != nil {
		framework.Failf("failed to delete pod: %v", err)
	}

	err = deletePVCAndValidatePV(f.ClientSet, pvc, deployTimeout)
	if err != nil {
		framework.Failf("failed to delete pvc: %v", err)
	}

	validateRBDImageCount(f, 0, defaultRbdPool)
}

func validateRdbBlock(pod *v1.Pod, f *framework.Framework) {
	cmd := `fdisk -l /dev/rbdblock`
	stdout, stdErr, err := execCommandInContainerByPodName(
		f,
		cmd,
		pod.Namespace,
		pod.Name,
		pod.Spec.Containers[0].Name,
	)
	if err != nil {
		framework.Failf("failed to exec cmd in pod: %v, %s", err, stdErr)
	}
	framework.Logf("Access RWX rdb block: %s", stdout)
}

// https://github.com/ceph/ceph-csi/tree/devel/examples#how-to-test-rbd-multi_node_multi_writer-block-feature
func validateRbdRwxVolume(pvcPath, podPath, anotherPodPath string, f *framework.Framework) {
	pvc, err := createPVC(pvcPath, f)
	if err != nil {
		framework.Failf("failed to create RBD pvc: %v", err)
	}

	pod, err := createPod(podPath, deployTimeout, f)
	if err != nil {
		framework.Failf("failed to create pod: %v", err)
	}

	anotherPod, err := createPod(anotherPodPath, deployTimeout, f)
	if err != nil {
		framework.Failf("failed to create another pod: %v", err)
	}

	validateRBDImageCount(f, 1, defaultRbdPool)

	validateRdbBlock(pod, f)
	validateRdbBlock(anotherPod, f)

	err = deletePod(pod.Name, pod.Namespace, f.ClientSet, deployTimeout)
	if err != nil {
		framework.Failf("failed to delete pod: %v", err)
	}

	err = deletePod(anotherPod.Name, anotherPod.Namespace, f.ClientSet, deployTimeout)
	if err != nil {
		framework.Failf("failed to delete another pod: %v", err)
	}

	err = deletePVCAndValidatePV(f.ClientSet, pvc, deployTimeout)
	if err != nil {
		framework.Failf("failed to delete pvc: %v", err)
	}

	validateRBDImageCount(f, 0, defaultRbdPool)
}

func validateRbdVolumeClone(pvcPath, podPath, clonePvcPath, clonePodPath string, f *framework.Framework) {
	pvc, err := createPVC(pvcPath, f)
	if err != nil {
		framework.Failf("failed to create RBD pvc: %v", err)
	}

	pod, err := createPod(podPath, deployTimeout, f)
	if err != nil {
		framework.Failf("failed to create pod: %v", err)
	}

	validateRBDImageCount(f, 1, defaultRbdPool)

	clonePvc, err := createPVC(clonePvcPath, f)
	if err != nil {
		framework.Failf("failed to create RBD pvc clone: %v", err)
	}

	clonePod, err := createPod(clonePodPath, deployTimeout, f)
	if err != nil {
		framework.Failf("failed to create pod clone: %v", err)
	}

	validateRBDImageCount(f, 3, defaultRbdPool)

	err = deletePod(pod.Name, pod.Namespace, f.ClientSet, deployTimeout)
	if err != nil {
		framework.Failf("failed to delete pod: %v", err)
	}

	err = deletePod(clonePod.Name, clonePod.Namespace, f.ClientSet, deployTimeout)
	if err != nil {
		framework.Failf("failed to delete clone pod: %v", err)
	}

	err = deletePVCAndValidatePV(f.ClientSet, pvc, deployTimeout)
	if err != nil {
		framework.Failf("failed to delete pvc: %v", err)
	}

	err = deletePVCAndValidatePV(f.ClientSet, clonePvc, deployTimeout)
	if err != nil {
		framework.Failf("failed to delete clone pvc: %v", err)
	}

	validateRBDImageCount(f, 0, defaultRbdPool)
}

func createRbdVolumeFromSnapshot(pvcPath, podPath, snapshotPath, restorePvcPath, restorePodPath string, f *framework.Framework) {
	By("create pvc")
	pvc, err := createPVC(pvcPath, f)
	if err != nil {
		framework.Failf("failed to create RBD pvc: %v", err)
	}

	By("create pod")
	pod, err := createPod(podPath, deployTimeout, f)
	if err != nil {
		framework.Failf("failed to create pod: %v", err)
	}

	By("validate rbd image count")
	validateRBDImageCount(f, 1, defaultRbdPool)

	snap := getSnapshot(snapshotPath)
	snap.Namespace = f.UniqueName
	snap.Spec.Source.PersistentVolumeClaimName = &pvc.Name

	By("create snapshot")
	err = createSnapshot(&snap, deployTimeout)
	if err != nil {
		framework.Failf("failed to create snapshot: %v", err)
	}

	By("validate rbd image count")
	validateRBDImageCount(f, 2, defaultRbdPool)

	By("delete pod")
	err = deletePod(pod.Name, pod.Namespace, f.ClientSet, deployTimeout)
	if err != nil {
		framework.Failf("failed to delete pod: %v", err)
	}

	By("delete pvc")
	err = deletePVCAndValidatePV(f.ClientSet, pvc, deployTimeout)
	if err != nil {
		framework.Failf("failed to delete pvc: %v", err)
	}

	By("validate rbd image count")
	validateRBDImageCount(f, 1, defaultRbdPool)

	By("create pvc from snapshot")
	restorePVC, err := createPVC(restorePvcPath, f)
	if err != nil {
		framework.Failf("failed to create RBD pvc: %v", err)
	}

	By("create pod from restored pvc")
	restorePod, err := createPod(restorePodPath, deployTimeout, f)
	if err != nil {
		framework.Failf("failed to create pod: %v", err)
	}

	By("validate rbd image count")
	validateRBDImageCount(f, 2, defaultRbdPool)

	By("delete snapshot")
	if err := deleteSnapshot(&snap, deployTimeout); err != nil {
		framework.Failf("failed to delete snapshot: %v", err)
	}

	By("validate rbd image count")
	validateRBDImageCount(f, 1, defaultRbdPool)

	By("delete pod")
	err = deletePod(restorePod.Name, restorePod.Namespace, f.ClientSet, deployTimeout)
	if err != nil {
		framework.Failf("failed to delete restore pod: %v", err)
	}

	By("delete pvc")
	err = deletePVCAndValidatePV(f.ClientSet, restorePVC, deployTimeout)
	if err != nil {
		framework.Failf("failed to delete restore pvc: %v", err)
	}

	By("validate rbd image count")
	validateRBDImageCount(f, 0, defaultRbdPool)
}

func validateRbdVolumeExpansion(pvcPath, podPath string, f *framework.Framework) {
	pvc, err := createPVC(pvcPath, f)
	if err != nil {
		framework.Failf("failed to create RBD pvc: %v", err)
	}

	pod, err := createPod(podPath, deployTimeout, f)
	if err != nil {
		framework.Failf("failed to create pod: %v", err)
	}

	validateRBDImageCount(f, 1, defaultRbdPool)

	By("validate volume size in pod")
	validateTestVolumeSize(pod, 900, f)

	expandPVC(f.ClientSet, pvc, deployTimeout)
	if err != nil {
		framework.Failf("failed to expand PVC: %v", err)
	}

	validateRBDImageCount(f, 1, defaultRbdPool)

	By("validate volume size in pod after expansion")
	validateTestVolumeSize(pod, 1800, f)

	err = deletePod(pod.Name, pod.Namespace, f.ClientSet, deployTimeout)
	if err != nil {
		framework.Failf("failed to delete pod: %v", err)
	}

	err = deletePVCAndValidatePV(f.ClientSet, pvc, deployTimeout)
	if err != nil {
		framework.Failf("failed to delete pvc: %v", err)
	}

	validateRBDImageCount(f, 0, defaultRbdPool)
}

var _ = Describe("Rbd", func() {
	f := framework.NewDefaultFramework(rbdType)
	f.NamespacePodSecurityEnforceLevel = api.LevelPrivileged

	Context("[GA] [PVC]", func() {
		BeforeEach(func() {
			if err := createRBDStorageClass(f.ClientSet, f,
				defaultRbdSc, nil, nil, deletePolicy); err != nil {
				framework.Failf("failed to create storageclass %s: %v", defaultRbdSc, err)
			}
		})

		AfterEach(func() {
			if err := deleteStorageClass(f.ClientSet, defaultRbdSc); err != nil {
				framework.Failf("failed to delete storageclass %s: %v", defaultRbdSc, err)
			}
		})

		It("should be able to dynamically provision Block mode RWO volume", Label("pvc"), func() {
			validateRbdRwoVolume(
				"manifest/rbd/block-rwo-pvc.yaml",
				"manifest/rbd/block-rwo-pod.yaml", f)
		})

		It("should be able to dynamically provision Block mode RWX volume", Label("rdb", "pvc", "rwx"), func() {
			validateRbdRwxVolume(
				"manifest/rbd/block-rwx-pvc.yaml",
				"manifest/rbd/block-rwx-pod.yaml",
				"manifest/rbd/block-rwx-pod-another.yaml", f)
		})

		It("should be able to dynamically provision File mode RWO volume", Label("pvc"), func() {
			validateRbdRwoVolume(
				"manifest/rbd/file-rwo-pvc.yaml",
				"manifest/rbd/file-rwo-pod.yaml", f)
		})

		It("[clone] should be able to provision File mode RWO volume from another volume", Label("clone"), func() {
			validateRbdVolumeClone(
				"manifest/rbd/file-rwo-pvc.yaml",
				"manifest/rbd/file-rwo-pod.yaml",
				"manifest/rbd/file-pvc-clone.yaml",
				"manifest/rbd/file-pod-clone.yaml", f)
		})

		It("[clone] should be able to provision Block mode RWO volume from another volume", Label("clone"), func() {
			validateRbdVolumeClone(
				"manifest/rbd/block-rwo-pvc.yaml",
				"manifest/rbd/block-rwo-pod.yaml",
				"manifest/rbd/block-pvc-clone.yaml",
				"manifest/rbd/block-pod-clone.yaml", f)
		})

		It("should be able to collect metrics of Block mode volume", Label("metrics"), func() {
			Skip("Not implemented")
		})

		It("should be able to collect metrics of File mode volume", Label("metrics"), func() {
			Skip("Not implemented")
		})

	})

	Context("[GA] [snapshot]", func() {
		BeforeEach(func() {
			_, err := f.DynamicClient.Resource(schema.GroupVersionResource{
				Group:    "apiextensions.k8s.io",
				Version:  "v1",
				Resource: "customresourcedefinitions",
			}).Get(context.Background(),
				"volumesnapshotclasses.snapshot.storage.k8s.io", metav1.GetOptions{})
			if err != nil {
				framework.Logf("Get volumesnapshotclasses error due to %v", err)
				Skip("Skip snapshot cases")
			}
			if err := createRBDStorageClass(f.ClientSet, f,
				defaultRbdSc, nil, nil, deletePolicy); err != nil {
				framework.Failf("failed to create storageclass %s: %v", defaultRbdSc, err)
			}

			if err := createRBDSnapshotClass(f); err != nil {
				framework.Failf("failed to create snapshotclass csi-rbdplugin-snapclass: %v", err)
			}
		})

		AfterEach(func() {
			if err := deleteStorageClass(f.ClientSet, defaultRbdSc); err != nil {
				framework.Failf("failed to delete storageclass %s: %v", defaultRbdSc, err)
			}

			if err := deleteRBDSnapshotClass(); err != nil {
				framework.Failf("failed to delete snapshotclass csi-rbdplugin-snapclass: %v", err)
			}
		})

		It("should be able to provision volume from snapshot", Label("snapshot"), func() {
			createRbdVolumeFromSnapshot(
				"manifest/rbd/file-rwo-pvc.yaml",
				"manifest/rbd/file-rwo-pod.yaml",
				"manifest/rbd/snapshot.yaml",
				"manifest/rbd/pvc-restore.yaml",
				"manifest/rbd/pod-restore.yaml", f)
		})
	})

	Context("[Beta]", func() {
		BeforeEach(func() {
			if err := createRBDStorageClass(f.ClientSet, f,
				defaultRbdSc, nil, nil, deletePolicy); err != nil {
				framework.Failf("failed to create storageclass %s: %v", defaultRbdSc, err)
			}
		})

		AfterEach(func() {
			if err := deleteStorageClass(f.ClientSet, defaultRbdSc); err != nil {
				framework.Failf("failed to delete storageclass %s: %v", defaultRbdSc, err)
			}
		})

		It("should be able to expand volume", Label("rbd", "beta", "expansion"), func() {
			validateRbdVolumeExpansion(
				"manifest/rbd/file-rwo-pvc.yaml",
				"manifest/rbd/file-rwo-pod.yaml", f)
		})
	})

})
