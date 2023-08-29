package ceph_csi

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/pod-security-admission/api"
)

var (
	defaultCephfsSc = "csi-cephfs-sc"
)

type cephfsSubVolume struct {
	Name string `json:"name"`
}

func listCephFSSubVolumes(f *framework.Framework, filesystem, groupname string) ([]cephfsSubVolume, error) {
	var subVols []cephfsSubVolume

	stdout, err := exec.Command("ceph", "fs", "subvolume", "ls", filesystem, "--group_name="+groupname, "--format=json").CombinedOutput()
	if err != nil {
		return subVols, fmt.Errorf("error listing subvolumes %s, %s", err.Error(), string(stdout))
	}

	err = json.Unmarshal(stdout, &subVols)
	if err != nil {
		return subVols, err
	}

	return subVols, nil
}

func validateSubvolumeCount(f *framework.Framework, count int, fileSystemName, subvolumegroup string) {
	subVol, err := listCephFSSubVolumes(f, fileSystemName, subvolumegroup)
	if err != nil {
		framework.Failf("failed to list CephFS subvolumes: %v", err)
	}
	if len(subVol) != count {
		framework.Failf("subvolumes [%v]. subvolume count %d not matching expected count %d", subVol, len(subVol), count)
	}
}

func validateCephfsRwoVolume(pvcPath, podPath string, f *framework.Framework) {
	pvc, err := createPVC(pvcPath, f)
	if err != nil {
		framework.Failf("failed to create Cephfs pvc: %v", err)
	}

	pod, err := createPod(podPath, deployTimeout, f)
	if err != nil {
		framework.Failf("failed to create pod: %v", err)
	}

	validateSubvolumeCount(f, 1, defaultFileSystemName, defaultSubvolumegroup)

	err = deletePod(pod.Name, pod.Namespace, f.ClientSet, deployTimeout)
	if err != nil {
		framework.Failf("failed to delete pod: %v", err)
	}

	err = deletePVCAndValidatePV(f.ClientSet, pvc, deployTimeout)
	if err != nil {
		framework.Failf("failed to delete pvc: %v", err)
	}

	validateSubvolumeCount(f, 0, defaultFileSystemName, defaultSubvolumegroup)
}

const testData = "cephfs-test"

func writeToCephfsPod(pod *v1.Pod, f *framework.Framework) {
	cmd := fmt.Sprintf(`echo %s | tee /var/lib/www/html/test`, testData)
	_, stdErr, err := execCommandInContainerByPodName(
		f,
		cmd,
		pod.Namespace,
		pod.Name,
		pod.Spec.Containers[0].Name,
	)
	if err != nil {
		framework.Failf("failed to exec cmd in pod: %v, %s", err, stdErr)
	}
}

func readFromCephfsPod(pod *v1.Pod, f *framework.Framework) {
	cmd := `cat /var/lib/www/html/test`
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
	stdout = strings.TrimSuffix(stdout, "\n")
	if stdout != testData {
		framework.Logf("test data: %s is not expected: %s", stdout, testData)
	}
}

func validateCephfsRwxVolume(pvcPath, podPath, anotherPodPath string, f *framework.Framework) {
	pvc, err := createPVC(pvcPath, f)
	if err != nil {
		framework.Failf("failed to create Cephfs pvc: %v", err)
	}

	pod, err := createPod(podPath, deployTimeout, f)
	if err != nil {
		framework.Failf("failed to create pod: %v", err)
	}

	anotherPod, err := createPod(anotherPodPath, deployTimeout, f)
	if err != nil {
		framework.Failf("failed to create another pod: %v", err)
	}

	validateSubvolumeCount(f, 1, defaultFileSystemName, defaultSubvolumegroup)

	writeToCephfsPod(pod, f)
	readFromCephfsPod(anotherPod, f)

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

	validateSubvolumeCount(f, 0, defaultFileSystemName, defaultSubvolumegroup)
}

func validateCephfsVolumeClone(pvcPath, podPath, clonePvcPath, clonePodPath string, f *framework.Framework) {
	pvc, err := createPVC(pvcPath, f)
	if err != nil {
		framework.Failf("failed to create Cephfs pvc: %v", err)
	}

	pod, err := createPod(podPath, deployTimeout, f)
	if err != nil {
		framework.Failf("failed to create pod: %v", err)
	}

	validateSubvolumeCount(f, 1, defaultFileSystemName, defaultSubvolumegroup)

	clonePvc, err := createPVC(clonePvcPath, f)
	if err != nil {
		framework.Failf("failed to create Cephfs pvc clone: %v", err)
	}

	clonePod, err := createPod(clonePodPath, deployTimeout, f)
	if err != nil {
		framework.Failf("failed to create pod clone: %v", err)
	}

	validateSubvolumeCount(f, 2, defaultFileSystemName, defaultSubvolumegroup)

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

	validateSubvolumeCount(f, 0, defaultFileSystemName, defaultSubvolumegroup)
}

func createCephfsVolumeFromSnapshot(pvcPath, podPath, snapshotPath, restorePvcPath, restorePodPath string, f *framework.Framework) {
	By("create pvc")
	pvc, err := createPVC(pvcPath, f)
	if err != nil {
		framework.Failf("failed to create Cephfs pvc: %v", err)
	}

	By("create pod")
	pod, err := createPod(podPath, deployTimeout, f)
	if err != nil {
		framework.Failf("failed to create pod: %v", err)
	}

	By("validate cephfs subvolume count")
	validateSubvolumeCount(f, 1, defaultFileSystemName, defaultSubvolumegroup)

	snap := getSnapshot(snapshotPath)
	snap.Namespace = f.UniqueName
	snap.Spec.Source.PersistentVolumeClaimName = &pvc.Name

	By("create snapshot")
	err = createSnapshot(&snap, deployTimeout)
	if err != nil {
		framework.Failf("failed to create snapshot: %v", err)
	}

	By("validate cephfs subvolume count")
	validateSubvolumeCount(f, 1, defaultFileSystemName, defaultSubvolumegroup)

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

	By("validate cephfs subvolume count")
	validateSubvolumeCount(f, 1, defaultFileSystemName, defaultSubvolumegroup)

	By("create pvc from snapshot")
	restorePVC, err := createPVC(restorePvcPath, f)
	if err != nil {
		framework.Failf("failed to create Cephfs pvc: %v", err)
	}

	By("create pod from restored pvc")
	restorePod, err := createPod(restorePodPath, deployTimeout, f)
	if err != nil {
		framework.Failf("failed to create pod: %v", err)
	}

	By("validate cephfs subvolume count")
	validateSubvolumeCount(f, 2, defaultFileSystemName, defaultSubvolumegroup)

	By("delete snapshot")
	if err := deleteSnapshot(&snap, deployTimeout); err != nil {
		framework.Failf("failed to delete snapshot: %v", err)
	}

	By("validate cephfs subvolume count")
	validateSubvolumeCount(f, 1, defaultFileSystemName, defaultSubvolumegroup)

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

	By("validate cephfs subvolume count")
	validateSubvolumeCount(f, 0, defaultFileSystemName, defaultSubvolumegroup)
}

func validateCephfsVolumeExpansion(pvcPath, podPath string, f *framework.Framework) {
	pvc, err := createPVC(pvcPath, f)
	if err != nil {
		framework.Failf("failed to create Cephfs pvc: %v", err)
	}

	pod, err := createPod(podPath, deployTimeout, f)
	if err != nil {
		framework.Failf("failed to create pod: %v", err)
	}

	validateSubvolumeCount(f, 1, defaultFileSystemName, defaultSubvolumegroup)

	By("validate volume size in pod")
	validateTestVolumeSize(pod, 900, f)

	expandPVC(f.ClientSet, pvc, deployTimeout)
	if err != nil {
		framework.Failf("failed to expand PVC: %v", err)
	}

	validateSubvolumeCount(f, 1, defaultFileSystemName, defaultSubvolumegroup)

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

	validateSubvolumeCount(f, 0, defaultFileSystemName, defaultSubvolumegroup)
}

var _ = Describe("Cephfs", func() {
	f := framework.NewDefaultFramework(cephfsType)
	f.NamespacePodSecurityEnforceLevel = api.LevelPrivileged

	Context("[GA]", func() {
		BeforeEach(func() {
			if err := createCephfsStorageClass(
				f.ClientSet, f, true, nil); err != nil {
				framework.Failf("failed to create storageclass %s: %v", defaultCephfsSc, err)
			}
		})

		AfterEach(func() {
			if err := deleteStorageClass(f.ClientSet, defaultCephfsSc); err != nil {
				framework.Failf("failed to delete storageclass %s: %v", defaultCephfsSc, err)
			}
		})

		It("should be able to dynamically provision File mode RWO volume", Label("cephfs", "pvc", "rwo"), func() {
			validateCephfsRwoVolume(
				"manifest/cephfs/rwo-pvc.yaml",
				"manifest/cephfs/rwo-pod.yaml", f)
		})

		It("should be able to dynamically provision File mode RWX volume", Label("cephfs", "pvc", "rwx"), func() {
			validateCephfsRwxVolume(
				"manifest/cephfs/rwx-pvc.yaml",
				"manifest/cephfs/rwx-pod.yaml",
				"manifest/cephfs/rwx-pod-another.yaml", f)
		})

		It("should be able to provision volume from another volume", Label("cephfs", "clone"), func() {
			validateCephfsVolumeClone(
				"manifest/cephfs/rwx-pvc.yaml",
				"manifest/cephfs/rwx-pod.yaml",
				"manifest/cephfs/pvc-clone.yaml",
				"manifest/cephfs/pod-clone.yaml", f)
		})

		It("should be able to collect metrics of File mode volume", Label("cephfs", "metrics"), func() {
			Skip("Not implemented")
		})
	})

	Context("[GA]", func() {
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

			if err := createCephfsStorageClass(f.ClientSet, f, true, nil); err != nil {
				framework.Failf("failed to create storageclass %s: %v", defaultCephfsSc, err)
			}

			if err := createCephfsSnapshotClass(f); err != nil {
				framework.Failf("failed to create snapshotclass csi-cephfsplugin-snapclass: %v", err)
			}
		})

		AfterEach(func() {
			if err := deleteStorageClass(f.ClientSet, defaultCephfsSc); err != nil {
				framework.Failf("failed to delete storageclass %s: %v", defaultCephfsSc, err)
			}

			if err := deleteCephfsSnapshotClass(); err != nil {
				framework.Failf("failed to delete snapshotclass csi-cephfsplugin-snapclass: %v", err)
			}
		})

		It("should be able to provision volume from snapshot", Label("cephfs", "snapshot"), func() {
			createCephfsVolumeFromSnapshot(
				"manifest/cephfs/rwx-pvc.yaml",
				"manifest/cephfs/rwx-pod.yaml",
				"manifest/cephfs/snapshot.yaml",
				"manifest/cephfs/pvc-restore.yaml",
				"manifest/cephfs/pod-restore.yaml", f)
		})
	})

	Context("[Beta]", func() {
		BeforeEach(func() {
			if err := createCephfsStorageClass(
				f.ClientSet, f, true, nil); err != nil {
				framework.Failf("failed to create storageclass %s: %v", defaultCephfsSc, err)
			}
		})

		AfterEach(func() {
			if err := deleteStorageClass(f.ClientSet, defaultCephfsSc); err != nil {
				framework.Failf("failed to delete storageclass %s: %v", defaultCephfsSc, err)
			}
		})

		It("should be able to expand volume", Label("cephfs", "beta", "expansion"), func() {
			validateCephfsVolumeExpansion(
				"manifest/cephfs/rwx-pvc.yaml",
				"manifest/cephfs/rwx-pod.yaml", f)
		})
	})
})
