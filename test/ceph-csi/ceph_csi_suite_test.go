package ceph_csi

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/config"
)

func setDefaultKubeconfig() {
	_, exists := os.LookupEnv("KUBECONFIG")
	if !exists {
		defaultKubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		os.Setenv("KUBECONFIG", defaultKubeconfig)
	}
}

func init() {
	log.SetOutput(GinkgoWriter)
	setDefaultKubeconfig()

	config.CopyFlags(config.Flags, flag.CommandLine)
	framework.RegisterCommonFlags(flag.CommandLine)
	framework.RegisterClusterFlags(flag.CommandLine)
	testing.Init()
	flag.Parse()
	framework.AfterReadingAllFlags(&framework.TestContext)
}

func TestCephCsi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CephCsi Suite")
}
