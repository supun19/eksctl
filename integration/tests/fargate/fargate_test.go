//go:build integration
// +build integration

package fargate

import (
	"fmt"
	"strings"
	"testing"
	"time"

	harness "github.com/dlespiau/kube-test-harness"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/weaveworks/eksctl/integration/runner"
	"github.com/weaveworks/eksctl/integration/tests"
	"github.com/weaveworks/eksctl/integration/utilities/kube"
	"github.com/weaveworks/eksctl/pkg/testutils"
)

var params *tests.Params

func init() {
	// Call testing.Init() prior to tests.NewParams(), as otherwise -test.* will not be recognised. See also: https://golang.org/doc/go1.13#testing
	testing.Init()
	params = tests.NewParams("fgate")
}

func TestFargate(t *testing.T) {
	testutils.RegisterAndRun(t)
}

var _ = Describe("(Integration) Fargate", func() {

	deleteCluster := func(clusterName string) {
		cmd := params.EksctlDeleteCmd.WithArgs(
			"cluster", clusterName,
			"--verbose", "4",
		)
		Expect(cmd).To(RunSuccessfully())
	}

	type fargateTest struct {
		clusterName string
		kubeTest    *harness.Test
	}

	setup := func(ft *fargateTest, createArgs ...string) {
		prefix := strings.ReplaceAll("fgate-"+strings.ReplaceAll(strings.Join(createArgs, ""), "-", ""), "--", "-")
		ft.clusterName = params.NewClusterName(prefix)
		args := []string{
			"cluster",
			"--name", ft.clusterName,
			"--verbose", "4",
			"--kubeconfig", params.KubeconfigPath,
			"--nodes", "1",
			"--timeout", "1h10m",
		}

		args = append(args, createArgs...)
		cmd := params.EksctlCreateCmd.WithArgs(args...)
		Expect(cmd).To(RunSuccessfully())

		var err error
		ft.kubeTest, err = kube.NewTest(params.KubeconfigPath)
		Expect(err).NotTo(HaveOccurred())
	}

	testDefaultFargateProfile := func(clusterName string, kubeTest *harness.Test) {
		By("having a default Fargate profile")
		cmd := params.EksctlGetCmd.WithArgs(
			"fargateprofile",
			"--cluster", clusterName,
			"--verbose", "4",
		)
		Expect(cmd).To(RunSuccessfullyWithOutputString(ContainSubstring("fp-default")))

		By("scheduling pods matching the default profile onto Fargate")
		d := kubeTest.CreateDeploymentFromFile("default", "../../data/podinfo.yaml")
		kubeTest.WaitForDeploymentReady(d, 10*time.Minute)

		pods := kubeTest.ListPodsFromDeployment(d)
		Expect(len(pods.Items)).To(Equal(2))
		for _, pod := range pods.Items {
			Expect(pod.Spec.NodeName).To(HavePrefix("fargate-"))
		}
		cmd = params.EksctlDeleteCmd.WithArgs(
			"fargateprofile",
			"--cluster", clusterName,
			"--name", "fp-default",
			"--wait",
			"--verbose", "4",
		)
		Expect(cmd).To(RunSuccessfully())
	}

	testCreateFargateProfile := func(clusterName string, kubeTest *harness.Test) {
		By("creating a new Fargate profile")
		profileName := "profile-1"
		cmd := params.EksctlCreateCmd.WithArgs(
			"fargateprofile",
			"--cluster", clusterName,
			"--name", profileName,
			"--namespace", kubeTest.Namespace,
			"--labels", "run-on=fargate",
			"--tags", "env=integration",
			"--verbose", "4",
		)
		Expect(cmd).To(RunSuccessfullyWithOutputString(ContainSubstring(profileName)))

		By("scheduling pods matching the selector onto Fargate")
		d := kubeTest.LoadDeployment("../../data/podinfo.yaml")
		d.Spec.Template.Labels["run-on"] = "fargate"

		kubeTest.CreateDeployment(kubeTest.Namespace, d)
		kubeTest.WaitForDeploymentReady(d, 10*time.Minute)
		pods := kubeTest.ListPodsFromDeployment(d)
		Expect(len(pods.Items)).To(Equal(2))
		for _, pod := range pods.Items {
			Expect(pod.Spec.NodeName).To(HavePrefix("fargate-"))
		}

		By(fmt.Sprintf("deleting Fargate profile: %q", profileName))
		cmd = params.EksctlDeleteCmd.WithArgs(
			"fargateprofile",
			"--cluster", clusterName,
			"--name", profileName,
			"--wait",
			"--verbose", "4",
		)
		Expect(cmd).To(RunSuccessfully())
	}

	Context("Creating a cluster with --fargate", func() {
		ft := &fargateTest{}

		BeforeEach(func() {
			setup(ft, "--fargate")
		})

		It("should support Fargate", func() {
			testDefaultFargateProfile(ft.clusterName, ft.kubeTest)
			testCreateFargateProfile(ft.clusterName, ft.kubeTest)
		})

		AfterEach(func() {
			deleteCluster(ft.clusterName)
		})
	})

	Context("Creating a cluster with --fargate and --managed", func() {
		ft := &fargateTest{}

		BeforeEach(func() {
			setup(ft, "--fargate", "--managed")
		})

		It("should support Fargate", func() {
			testDefaultFargateProfile(ft.clusterName, ft.kubeTest)
			testCreateFargateProfile(ft.clusterName, ft.kubeTest)
		})

		AfterEach(func() {
			deleteCluster(ft.clusterName)
		})
	})

	Context("Creating a cluster without --fargate", func() {
		ft := &fargateTest{}

		BeforeEach(func() {
			setup(ft)
		})

		It("should allow creation of new Fargate profiles", func() {
			testCreateFargateProfile(ft.clusterName, ft.kubeTest)
		})

		AfterEach(func() {
			deleteCluster(ft.clusterName)
		})
	})
})

var _ = AfterSuite(func() {
	params.DeleteClusters()
})
