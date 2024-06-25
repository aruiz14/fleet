package singlecluster_test

import (
	"fmt"
	"math/rand"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/matchers"
	"github.com/rancher/fleet/e2e/testenv"
	"github.com/rancher/fleet/e2e/testenv/kubectl"
)

var shards = []string{"shard1", "shard2", "shard3"}

var _ = Describe("Filtering events by shard", Label("sharding"), Ordered, func() {
	var (
		k               kubectl.Command
		gitrepoName     string
		r               = rand.New(rand.NewSource(GinkgoRandomSeed()))
		targetNamespace string
	)

	BeforeAll(func() {
		// No sharded controller should have reconciled any GitRepo until this point.
		for _, shard := range shards {
			logs, err := k.Namespace("cattle-fleet-system").Logs(
				"-l",
				"app=fleet-controller",
				"-l",
				fmt.Sprintf("fleet.cattle.io/shard-id=%s", shard),
				"--tail=-1",
			)
			Expect(err).ToNot(HaveOccurred())
			regexMatcher := matchers.MatchRegexpMatcher{Regexp: "Reconciling GitRepo.*"}
			hasReconciledGitRepos, err := regexMatcher.Match(logs)
			Expect(err).ToNot(HaveOccurred())
			Expect(hasReconciledGitRepos).To(BeFalse())
		}

		k = env.Kubectl.Namespace(env.Namespace)
	})

	for _, shard := range shards {
		When(fmt.Sprintf("deploying a gitrepo labeled with shard ID %s", shard), func() {
			JustBeforeEach(func() {
				targetNamespace = testenv.NewNamespaceName("target", r)
				gitrepoName = testenv.RandomFilename("sharding-test", r)

				err := testenv.ApplyTemplate(
					k,
					testenv.AssetPath("gitrepo/gitrepo_sharded.yaml"),
					struct {
						Name            string
						Repo            string
						Branch          string
						PollingInterval string
						TargetNamespace string
						ShardID         string
					}{
						gitrepoName,
						"https://github.com/rancher/fleet-test-data",
						"master",
						"15s",           // default
						targetNamespace, // to avoid conflicts with other tests
						shard,
					},
				)
				Expect(err).ToNot(HaveOccurred())
			})

			It(fmt.Sprintf("deploys the gitrepo via the controller labeled with shard ID %s", shard), func() {
				By("checking the configmap exists")
				Eventually(func() string {
					out, _ := k.Namespace(targetNamespace).Get("configmaps")
					return out
				}).Should(ContainSubstring("test-simple-chart-config"))

				for _, s := range shards {
					Eventually(func(g Gomega) {
						logs, err := k.Namespace("cattle-fleet-system").Logs(
							"-l",
							"app=fleet-controller",
							"-l",
							fmt.Sprintf("fleet.cattle.io/shard-id=%s", s),
							"--tail=100",
						)
						g.Expect(err).ToNot(HaveOccurred())
						regexMatcher := matchers.MatchRegexpMatcher{
							Regexp: fmt.Sprintf(`Reconciling GitRepo.*"name":"%s"`, gitrepoName),
						}
						hasReconciledGitRepo, err := regexMatcher.Match(logs)
						g.Expect(err).ToNot(HaveOccurred())
						if s == shard {
							g.Expect(hasReconciledGitRepo).To(BeTrueBecause(
								"GitRepo %q labeled with shard %q should have been"+
									" deployed by controller for shard %q in namespace %q",
								gitrepoName,
								shard,
								shard,
								env.Namespace,
							))
						} else {
							g.Expect(hasReconciledGitRepo).To(BeFalseBecause(
								"GitRepo %q labeled with shard %q should not have been"+
									" deployed by controller for shard %q",
								gitrepoName,
								shard,
								s,
							))
						}
					}).Should(Succeed())
				}
			})

			AfterEach(func() {
				_, _ = k.Delete("gitrepo", gitrepoName)
				_, _ = k.Delete("ns", targetNamespace, "--wait=false")
			})
		})
	}

	When("deploying a gitrepo labeled with an unknown shard ID", func() {
		JustBeforeEach(func() {
			targetNamespace = testenv.NewNamespaceName("target", r)
			gitrepoName = testenv.RandomFilename("sharding-test", r)

			err := testenv.ApplyTemplate(k, testenv.AssetPath("gitrepo/gitrepo_sharded.yaml"), struct {
				Name            string
				Repo            string
				Branch          string
				PollingInterval string
				TargetNamespace string
				ShardID         string
			}{
				gitrepoName,
				"https://github.com/rancher/fleet-test-data",
				"master",
				"15s",           // default
				targetNamespace, // to avoid conflicts with other tests
				"unknown",
			})
			Expect(err).ToNot(HaveOccurred())
		})

		It("does not deploy the gitrepo", func() {
			By("checking the configmap does not exist")
			Eventually(func() string {
				out, _ := k.Namespace(targetNamespace).Get("configmaps")
				return out
			}).ShouldNot(ContainSubstring("test-simple-chart-config"))

			for _, s := range shards {
				logs, err := k.Namespace("cattle-fleet-system").Logs(
					"-l",
					"app=fleet-controller",
					"-l",
					fmt.Sprintf("fleet.cattle.io/shard-id=%s", s),
					"--tail=100",
				)
				Expect(err).ToNot(HaveOccurred())
				regexMatcher := matchers.MatchRegexpMatcher{
					Regexp: fmt.Sprintf(
						`Reconciling GitRepo.*"GitRepo": {"name":"%s","namespace":"%s"}`,
						gitrepoName,
						env.Namespace,
					),
				}
				hasReconciledGitRepos, err := regexMatcher.Match(logs)
				Expect(err).ToNot(HaveOccurred())
				Expect(hasReconciledGitRepos).To(BeFalseBecause(
					"GitRepo labeled with shard %q should not have been deployed by"+
						" controller for shard %q",
					"unknown",
					s,
				))
			}
		})

		AfterEach(func() {
			_, _ = k.Delete("gitrepo", gitrepoName)
			_, _ = k.Delete("ns", targetNamespace, "--wait=false")
		})
	})
})
