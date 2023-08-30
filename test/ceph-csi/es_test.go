package ceph_csi

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/pod-security-admission/api"
)

func installEs(crdsPath, operatorPath, esPath string, f *framework.Framework) {
	if err := exec.
		Command("kubectl", "create", "-f", crdsPath, "-f", operatorPath).
		Run(); err != nil {
		framework.Failf("create es crds and operator error %v", err)
	}

	ctx := context.Background()

	err := wait.PollUntilContextTimeout(ctx, poll, time.Duration(deployTimeout)*time.Minute, true, func(_ context.Context) (bool, error) {
		pod, err := f.ClientSet.CoreV1().
			Pods("elastic-system").
			Get(ctx, "elastic-operator-0", metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		if pod.Status.Phase != v1.PodRunning {
			framework.Logf("es operator pod is %s", pod.Status.Phase)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		framework.Failf("es operator pod is not running %v", err)
	}

	if err := exec.
		Command("kubectl", "create", "-f", esPath, "-n", f.UniqueName).
		Run(); err != nil {
		framework.Failf("create ElasticSearch error %v", err)
	}

	err = wait.PollUntilContextTimeout(ctx, poll, time.Duration(deployTimeout)*time.Minute, true, func(_ context.Context) (bool, error) {
		podList, err := f.ClientSet.CoreV1().
			Pods(f.UniqueName).
			List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, nil
		}
		if len(podList.Items) == 0 {
			return false, nil
		}
		for _, po := range podList.Items {
			if po.Status.Phase != v1.PodRunning {
				framework.Logf("es operator pod %s is %s", po.Name, po.Status.Phase)
				return false, nil
			}
		}

		return true, nil
	})
	if err != nil {
		framework.Failf("ElasticSearch pod is not running %v", err)
	}
}

func uninstallEs(crdsPath, operatorPath, esPath string, f *framework.Framework) {
	if err := exec.
		Command("kubectl", "delete", "-f", esPath, "-n", f.UniqueName).
		Run(); err != nil {
		framework.Logf("delete ElasticSearch error %v", err)
	}

	ctx := context.Background()

	err := wait.PollUntilContextTimeout(ctx, poll, time.Duration(deployTimeout)*time.Minute, true, func(_ context.Context) (bool, error) {
		podList, err := f.ClientSet.CoreV1().Pods(f.UniqueName).
			List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, nil
		}
		if len(podList.Items) > 0 {
			framework.Logf("pods still exists")
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		framework.Logf("delete ElasticSearch pods error %v", err)
	}

	if err := exec.
		Command("kubectl", "delete", "-f", operatorPath, "-f", crdsPath).
		Run(); err != nil {
		framework.Logf("delete es crds and operator error %v", err)
	}

	err = wait.PollUntilContextTimeout(ctx, poll, time.Duration(deployTimeout)*time.Minute, true, func(_ context.Context) (bool, error) {
		ns, err := f.ClientSet.CoreV1().Namespaces().
			Get(ctx, "elastic-system", metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return true, nil
			}
			return false, nil
		}
		framework.Logf("namespace elastic-system is %s", ns.Status.Phase)
		return false, nil
	})
	if err != nil {
		framework.Logf("delete namespace elastic-system error %v", err)
	}
}

var _ = Describe("ElasticSearch", func() {
	f := framework.NewDefaultFramework("es")
	f.NamespacePodSecurityEnforceLevel = api.LevelPrivileged

	Context("app", func() {
		BeforeEach(func() {
			installEs("manifest/eck/crds.yaml",
				"manifest/eck/operator.yaml",
				"manifest/eck/elasticsearch.yaml", f)
		})
		AfterEach(func() {
			uninstallEs("manifest/eck/crds.yaml",
				"manifest/eck/operator.yaml",
				"manifest/eck/elasticsearch.yaml", f)
		})
		It("should be able to run ElasticSearch using ceph rbd plugin", Label("es"), func() {
			secret, err := f.ClientSet.CoreV1().Secrets(f.UniqueName).Get(context.Background(), "elasticsearch-sample-es-elastic-user", metav1.GetOptions{})
			if err != nil {
				framework.Failf("get elasticsearch-sample-es-elastic-user secret error %v", err)
			}
			pass := secret.Data["elastic"]
			// pass, _ = base64.StdEncoding.DecodeString(string(pass))

			svc, err := f.ClientSet.CoreV1().
				Services(f.UniqueName).
				Get(context.Background(), "elasticsearch-sample-es-http", metav1.GetOptions{})
			if err != nil {
				framework.Failf("get elasticsearch-sample-es-http svc error %v", err)
			}

			podList, err := f.ClientSet.CoreV1().Pods(f.UniqueName).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				framework.Failf("get elasticsearch pod error %v", err)
			}
			if len(podList.Items) == 0 {
				framework.Failf("elasticsearch pod not exist error %v", err)
			}
			nodeIP := podList.Items[0].Status.HostIP

			url := fmt.Sprintf("https://%s:%d", nodeIP, svc.Spec.Ports[0].NodePort)
			ctx := context.Background()
			err = wait.PollUntilContextTimeout(ctx, poll, time.Duration(deployTimeout)*time.Minute, true, func(_ context.Context) (bool, error) {
				out, err := exec.Command("curl", "-u", "elastic:"+string(pass), "-k", url).CombinedOutput()
				if err != nil {
					return false, nil
				}
				if strings.Contains(string(out), "You Know, for Search") {
					return true, nil
				}
				return false, nil
			})
			if err != nil {
				framework.Failf("Access elasticsearch svc endpoint %s error %v", url, err)
			}

		})
	})
})
