package ceph_csi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	scv1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
)

const (
	rbdType    = "rbd"
	cephfsType = "cephfs"

	rbdStorageClass    = "ceph-rbd"
	cephfsStorageClass = "cephfs"

	rbdMountOptions = "mountOptions"

	retainPolicy = v1.PersistentVolumeReclaimRetain
	// deletePolicy is the default policy in E2E.
	deletePolicy = v1.PersistentVolumeReclaimDelete

	// ceph user names.
	keyringRBDProvisionerUsername          = "rook-csi-rbd-provisioner"
	keyringRBDNodePluginUsername           = "rook-csi-rbd-node"
	keyringRBDNamespaceProvisionerUsername = "rook-csi-rbd-ns-provisioner"
	keyringRBDNamespaceNodePluginUsername  = "rook-csi-rbd-ns-node"
	keyringCephFSProvisionerUsername       = "rook-csi-cephfs-provisioner"
	keyringCephFSNodePluginUsername        = "rook-csi-cephfs-node"

	// secret names.
	rbdNodePluginSecretName           = "rook-csi-rbd-node"
	rbdProvisionerSecretName          = "rook-csi-rbd-provisioner"
	rbdNamespaceNodePluginSecretName  = "rook-csi-rbd-ns-node"
	rbdNamespaceProvisionerSecretName = "rook-csi-rbd-ns-provisioner"
	rbdMigrationNodePluginSecretName  = "rook-csi-rbd-mig-node"
	rbdMigrationProvisionerSecretName = "rook-csi-rbd-mig-provisioner"
	cephFSNodePluginSecretName        = "rook-csi-cephfs-node"
	cephFSProvisionerSecretName       = "rook-csi-cephfs-provisioner"

	noError = ""
)

var (
	deployTimeout int = 2
	poll              = 2 * time.Second

	cephCSINamespace       string = "rook-ceph"
	cephCSISecretNamespace string = "rook-ceph-external"

	defaultRbdPool = "replicapool"

	defaultFileSystemName = "myfs"

	defaultSubvolumegroup = "csi"

	radosNamespace string
)

func addTopologyDomainsToDSYaml(template, labels string) string {
	return strings.ReplaceAll(template, "# - \"--domainlabels=failure-domain/region,failure-domain/zone\"",
		"- \"--domainlabels="+labels+"\"")
}

func oneReplicaDeployYaml(template string) string {
	re := regexp.MustCompile(`(\s+replicas:) \d+`)

	return re.ReplaceAllString(template, `$1 1`)
}

func enableTopologyInTemplate(data string) string {
	return strings.ReplaceAll(data, "--feature-gates=Topology=false", "--feature-gates=Topology=true")
}

// kubectlAction is used to tell retryKubectlInput() what action needs to be
// done.
type kubectlAction string

const (
	// kubectlCreate tells retryKubectlInput() to run "create".
	kubectlCreate = kubectlAction("create")
	// kubectlDelete tells retryKubectlInput() to run "delete".
	kubectlDelete = kubectlAction("delete")
)

// String returns the string format of the kubectlAction, this is automatically
// used when formatting strings with %s or %q.
func (ka kubectlAction) String() string {
	return string(ka)
}

// retryKubectlInput takes a namespace and action telling kubectl what to do,
// it then feeds data through stdin to the process. This function retries until
// no error occurred, or the timeout passed.
func retryKubectlInput(namespace string, action kubectlAction, data string, t int, args ...string) error {
	timeout := time.Duration(t) * time.Minute
	framework.Logf("waiting for kubectl (%s -f args %s) to finish", action, args)
	start := time.Now()

	return wait.PollUntilContextTimeout(context.TODO(), poll, timeout, true, func(_ context.Context) (bool, error) {
		cmd := []string{}
		if len(args) != 0 {
			cmd = append(cmd, strings.Join(args, ""))
		}
		cmd = append(cmd, []string{string(action), "-f", "-"}...)

		_, err := e2ekubectl.RunKubectlInput(namespace, data, cmd...)
		if err != nil {
			if isRetryableAPIError(err) {
				return false, nil
			}
			if action == kubectlCreate && isAlreadyExistsCLIError(err) {
				return true, nil
			}
			if action == kubectlDelete && isNotFoundCLIError(err) {
				return true, nil
			}
			framework.Logf(
				"will run kubectl (%s) args (%s) again (%d seconds elapsed)",
				action,
				args,
				int(time.Since(start).Seconds()))

			return false, fmt.Errorf("failed to run kubectl: %w", err)
		}

		return true, nil
	})
}

// retryKubectlFile takes a namespace and action telling kubectl what to do
// with the passed filename and arguments. This function retries until no error
// occurred, or the timeout passed.
func retryKubectlFile(namespace string, action kubectlAction, filename string, t int, args ...string) error {
	timeout := time.Duration(t) * time.Minute
	framework.Logf("waiting for kubectl (%s -f %q args %s) to finish", action, filename, args)
	start := time.Now()

	return wait.PollUntilContextTimeout(context.TODO(), poll, timeout, true, func(_ context.Context) (bool, error) {
		cmd := []string{}
		if len(args) != 0 {
			cmd = append(cmd, strings.Join(args, ""))
		}
		cmd = append(cmd, []string{string(action), "-f", filename}...)

		_, err := e2ekubectl.RunKubectl(namespace, cmd...)
		if err != nil {
			if isRetryableAPIError(err) {
				return false, nil
			}
			if action == kubectlCreate && isAlreadyExistsCLIError(err) {
				return true, nil
			}
			if action == kubectlDelete && isNotFoundCLIError(err) {
				return true, nil
			}
			framework.Logf(
				"will run kubectl (%s -f %q args %s) again (%d seconds elapsed)",
				action,
				filename,
				args,
				int(time.Since(start).Seconds()))

			return false, fmt.Errorf("failed to run kubectl: %w", err)
		}

		return true, nil
	})
}

// retryKubectlArgs takes a namespace and action telling kubectl what to do
// with the passed arguments. This function retries until no error occurred, or
// the timeout passed.
//
//nolint:unparam // retryKubectlArgs will be used with kubectlDelete arg later on.
func retryKubectlArgs(namespace string, action kubectlAction, t int, args ...string) error {
	timeout := time.Duration(t) * time.Minute
	args = append([]string{string(action)}, args...)
	framework.Logf("waiting for kubectl (%s args) to finish", args)
	start := time.Now()

	return wait.PollUntilContextTimeout(context.TODO(), poll, timeout, true, func(_ context.Context) (bool, error) {
		_, err := e2ekubectl.RunKubectl(namespace, args...)
		if err != nil {
			if isRetryableAPIError(err) {
				return false, nil
			}
			if action == kubectlCreate && isAlreadyExistsCLIError(err) {
				return true, nil
			}
			if action == kubectlDelete && isNotFoundCLIError(err) {
				return true, nil
			}
			framework.Logf(
				"will run kubectl (%s) again (%d seconds elapsed)",
				args,
				int(time.Since(start).Seconds()))

			return false, fmt.Errorf("failed to run kubectl: %w", err)
		}

		return true, nil
	})
}

func unmarshal(fileName string, obj interface{}) error {
	f, err := os.ReadFile(fileName)
	if err != nil {
		return err
	}
	data, err := utilyaml.ToJSON(f)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, obj)

	return err
}

func getStorageClass(path string) (scv1.StorageClass, error) {
	sc := scv1.StorageClass{}
	err := unmarshal(path, &sc)

	return sc, err
}

func deleteStorageClass(c kubernetes.Interface, name string) error {
	return c.StorageV1().StorageClasses().Delete(context.Background(), name, metav1.DeleteOptions{})
}

func createRBDStorageClass(
	c kubernetes.Interface,
	f *framework.Framework,
	name string,
	scOptions, parameters map[string]string,
	policy v1.PersistentVolumeReclaimPolicy,
) error {
	scPath := "manifest/rbd/storageclass.yaml"
	sc, err := getStorageClass(scPath)
	if err != nil {
		return fmt.Errorf("failed to get sc: %w", err)
	}

	sc.Provisioner = "rook-ceph.rbd.csi.ceph.com"
	sc.Name = name
	sc.Parameters["pool"] = defaultRbdPool
	sc.Parameters["csi.storage.k8s.io/provisioner-secret-namespace"] = cephCSISecretNamespace
	sc.Parameters["csi.storage.k8s.io/provisioner-secret-name"] = rbdProvisionerSecretName

	sc.Parameters["csi.storage.k8s.io/controller-expand-secret-namespace"] = cephCSISecretNamespace
	sc.Parameters["csi.storage.k8s.io/controller-expand-secret-name"] = rbdProvisionerSecretName

	sc.Parameters["csi.storage.k8s.io/node-stage-secret-namespace"] = cephCSISecretNamespace
	sc.Parameters["csi.storage.k8s.io/node-stage-secret-name"] = rbdNodePluginSecretName

	fsID, err := getCephClusterID()
	if err != nil {
		return fmt.Errorf("failed to get ceph clusterID: %w", err)
	}

	if sc.Parameters["clusterID"] == "" {
		sc.Parameters["clusterID"] = fsID
	}
	for k, v := range parameters {
		sc.Parameters[k] = v
		// if any values are empty remove it from the map
		if v == "" {
			delete(sc.Parameters, k)
		}
	}

	if scOptions["volumeBindingMode"] == "WaitForFirstConsumer" {
		value := scv1.VolumeBindingWaitForFirstConsumer
		sc.VolumeBindingMode = &value
	}

	// comma separated mount options
	if opt, ok := scOptions[rbdMountOptions]; ok {
		mOpt := strings.Split(opt, ",")
		sc.MountOptions = append(sc.MountOptions, mOpt...)
	}
	sc.ReclaimPolicy = &policy

	timeout := time.Duration(deployTimeout) * time.Minute

	return wait.PollUntilContextTimeout(context.TODO(), poll, timeout, true, func(ctx context.Context) (bool, error) {
		_, err = c.StorageV1().StorageClasses().Create(ctx, &sc, metav1.CreateOptions{})
		if err != nil {
			framework.Logf("error creating StorageClass %q: %v", sc.Name, err)
			if isRetryableAPIError(err) {
				return false, nil
			}

			return false, fmt.Errorf("failed to create StorageClass %q: %w", sc.Name, err)
		}

		return true, nil
	})
}

// --------------------------

func createCephfsStorageClass(
	c kubernetes.Interface,
	f *framework.Framework,
	enablePool bool,
	params map[string]string,
) error {
	scPath := "manifest/cephfs/storageclass.yaml"
	sc, err := getStorageClass(scPath)
	if err != nil {
		return err
	}
	// TODO: remove this once the ceph-csi driver release-v3.9 is completed
	// and upgrade tests are done from v3.9 to devel.
	// The mountOptions from previous are not compatible with NodeStageVolume
	// request.
	sc.Provisioner = "rook-ceph.cephfs.csi.ceph.com"
	sc.MountOptions = []string{}

	sc.Parameters["fsName"] = defaultFileSystemName
	sc.Parameters["csi.storage.k8s.io/provisioner-secret-namespace"] = cephCSISecretNamespace
	sc.Parameters["csi.storage.k8s.io/provisioner-secret-name"] = cephFSProvisionerSecretName

	sc.Parameters["csi.storage.k8s.io/controller-expand-secret-namespace"] = cephCSISecretNamespace
	sc.Parameters["csi.storage.k8s.io/controller-expand-secret-name"] = cephFSProvisionerSecretName

	sc.Parameters["csi.storage.k8s.io/node-stage-secret-namespace"] = cephCSISecretNamespace
	sc.Parameters["csi.storage.k8s.io/node-stage-secret-name"] = cephFSNodePluginSecretName

	if enablePool {
		sc.Parameters["pool"] = "myfs-replicated"
	}

	// overload any parameters that were passed
	if params == nil {
		// create an empty params, so that params["clusterID"] below
		// does not panic
		params = map[string]string{}
	}
	for param, value := range params {
		sc.Parameters[param] = value
	}

	fsID, err := getCephClusterID()
	if err != nil {
		return fmt.Errorf("failed to get ceph clusterID: %w", err)
	}

	if sc.Parameters["clusterID"] == "" {
		sc.Parameters["clusterID"] = fsID
	}

	timeout := time.Duration(deployTimeout) * time.Minute

	return wait.PollUntilContextTimeout(context.TODO(), poll, timeout, true, func(ctx context.Context) (bool, error) {
		_, err = c.StorageV1().StorageClasses().Create(ctx, &sc, metav1.CreateOptions{})
		if err != nil {
			framework.Logf("error creating StorageClass %q: %v", sc.Name, err)
			if isRetryableAPIError(err) {
				return false, nil
			}

			return false, fmt.Errorf("failed to create StorageClass %q: %w", sc.Name, err)
		}

		return true, nil
	})
}

func validateTestVolumeSize(pod *v1.Pod, expectSizeInMb int, f *framework.Framework) error {
	cmd := `stat -f -c '%b' /var/lib/www/html`
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
	blocks, err := strconv.Atoi(strings.TrimSuffix(stdout, "\n"))
	if err != nil {
		framework.Failf("failed to convert disk total block number: %v", err)
	}

	cmd = `stat -f -c '%S' /var/lib/www/html`
	stdout, stdErr, err = execCommandInContainerByPodName(
		f,
		cmd,
		pod.Namespace,
		pod.Name,
		pod.Spec.Containers[0].Name,
	)
	if err != nil {
		framework.Failf("failed to exec cmd in pod: %v, %s", err, stdErr)
	}
	blockSize, err := strconv.Atoi(strings.TrimSuffix(stdout, "\n"))
	if err != nil {
		framework.Failf("failed to convert disk block size: %v", err)
	}

	diskSize := blockSize * blocks / 1024 / 1024
	framework.Logf("disk size %dM", diskSize)
	if diskSize < expectSizeInMb {
		framework.Failf("disk size %d is less than %d", diskSize, expectSizeInMb)
	}

	return nil
}

// --------------------------
//     ceph client operation
// --------------------------

var (
	clusterID string
)

func getCephClusterID() (string, error) {
	if clusterID != "" {
		return clusterID, nil
	}

	fsID, err := exec.Command("ceph", "fsid").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed getting clusterID: %w", err)
	}
	// remove new line present in fsID
	return strings.Trim(string(fsID), "\n"), nil
}
