/*
Copyright 2024 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/konflux-ci/mintmaker/pkg/common"
	"github.com/konflux-ci/mintmaker/pkg/git/github"
	"github.com/konflux-ci/mintmaker/pkg/renovate"
)

const (
	githubAppPrivateKey = `-----BEGIN CERTIFICATE-----
MIIC+TCCAeGgAwIBAgIUbJ76r3NS4xiHwVQVbkF5Wn+rTQEwDQYJKoZIhvcNAQEL
BQAwDDEKMAgGA1UEAwwBeDAeFw0yNDA1MTkxNDA4MzRaFw0yNTA1MTkxNDA4MzRa
MAwxCjAIBgNVBAMMAXgwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDE
B3jF49FCpsYYeaiT5ohBvvvzK98yeF8Wjpn5pywApYmH94Z+dT2H1p3swPYMvHQd
/rU5Pueu3vqQ+D+YC5zT2+6ET9i5xTXAxF2DPWSlTlk6sDfodo23dENXPpXufkuN
l0TiPf8+OmCAltbPDlToDsO1fSXXekgboZ+3b3aBGP2pKtGo7eeLlDnxB04R2dNT
WjsD0fVPu6C8NjxYTkWXkGZaJtObfuAhFHWejsgHQzvOZo9HX9qcAQxNCtVGSzxo
WmbE31otsa8HbVA5QO1uLIDZXnXB7mCrWWYFWuPlQTGAMzWKreZmRO1I3psL8uW5
xvxZUsCGfldvN5pnZQ+VAgMBAAGjUzBRMB0GA1UdDgQWBBSiUW/ZOYDk/bIZUsMI
RC2cXftJGzAfBgNVHSMEGDAWgBSiUW/ZOYDk/bIZUsMIRC2cXftJGzAPBgNVHRMB
Af8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQCu82fHytxAxAdzhA915wZ+btWK
Q9HTzvsvHVwUXntzaRarZjJcajBNgTmuObImJlUiMCho8hiYWZdoCWxe7V1jEjhG
97TusFlUyGocqeLdDeD9ZeDijkGgd/hPlcQD5apOjryZFY59hIGHtDIDEjbK2DR8
rC22ymJ1NSKhb1XGJCjefDwASomwpzlfOxoJ3JSs1TvGCN5zPpeLYIyfgeL5bv/A
4MyQo/2doDDlJGAPVpB/DPaEkwmjct0vXR7fyW7gGUlmg930+PJbhSDiGNRr/eyE
sm2pXujQp36d8DfP7ht0kzojSqY06+JxnIYzNQ3zpd6gxQ+afF5p3PXO/6Wo
-----END CERTIFICATE-----`
	RenovateConfigMapName = "renovate-config"
)

var _ = Describe("DependencyUpdateCheck Controller", func() {

	var (
		pacSecretKey         = types.NamespacedName{Name: PipelinesAsCodeGitHubAppSecretName, Namespace: MintMakerNamespaceName}
		renovateConfigMapKey = types.NamespacedName{Name: RenovateConfigMapName, Namespace: MintMakerNamespaceName}
		defaultNS            = "default"
	)

	Context("Test Renovate jobs creation", func() {

		_ = BeforeEach(func() {
			createNamespace(MintMakerNamespaceName)
			pacSecretData := map[string]string{
				"github-application-id": "12345",
				"github-private-key":    githubAppPrivateKey,
			}
			createSecret(pacSecretKey, pacSecretData)
			renovateConfigMapData := map[string]string{
				"renovate.json": `{"forkProcessing": "enabled"}`,
			}
			createConfigMap(renovateConfigMapKey, renovateConfigMapData)
		})

		_ = AfterEach(func() {
			deleteJobs(MintMakerNamespaceName)
			os.Unsetenv(renovate.InstallationsPerJobEnvName)
			deleteSecret(pacSecretKey)
			deleteConfigMap(renovateConfigMapKey)
		})

		It("should trigger job for GitHub component", func() {
			installedRepositoryUrls := []string{
				"https://github.com/konfluxtest/repo",
			}
			github.GetAllAppInstallations = func(appIdStr string, privateKeyPem []byte) ([]github.ApplicationInstallation, string, error) {
				repositories := generateRepositories(installedRepositoryUrls)
				return []github.ApplicationInstallation{generateInstallation(repositories)}, "slug", nil
			}

			// Create a component with GitHub repository
			createNamespace(defaultNS)
			componentKey := types.NamespacedName{Namespace: defaultNS, Name: "test-component"}
			createComponent(componentKey, "test-application", "https://github.com/konfluxtest/repo", "main", "/")

			// Create a DependencyUpdateCheck CR in "mintmaker" namespace
			dependencyUpdateCheckKey := types.NamespacedName{Namespace: MintMakerNamespaceName, Name: "dependencyupdatecheck-sample"}
			createDependencyUpdateCheck(dependencyUpdateCheckKey, false)

			Eventually(listJobs).WithArguments(MintMakerNamespaceName).WithTimeout(timeout).Should(HaveLen(1))

			deleteComponent(componentKey)
			deleteDependencyUpdateCheck(dependencyUpdateCheckKey)
		})

		It("should trigger job for GitLab component", func() {
			installedRepositoryUrls := []string{
				"https://gitlab.com/konfluxtest/repo",
			}
			github.GetAllAppInstallations = func(appIdStr string, privateKeyPem []byte) ([]github.ApplicationInstallation, string, error) {
				repositories := generateRepositories(installedRepositoryUrls)
				return []github.ApplicationInstallation{generateInstallation(repositories)}, "slug", nil
			}

			// Create a component with GitLab repository
			createNamespace(defaultNS)
			componentKey := types.NamespacedName{Namespace: defaultNS, Name: "test-component"}
			createComponent(componentKey, "test-application", "https://gitlab.com/konfluxtest/repo", "main", "/")

			renovateSecretKey := types.NamespacedName{Name: "testsecret", Namespace: defaultNS}
			renovateSecretData := map[string]string{"password": "glp_token"}
			createSCMSecret(renovateSecretKey, renovateSecretData, corev1.SecretTypeBasicAuth, map[string]string{})

			// Create a DependencyUpdateCheck CR in "mintmaker" namespace
			dependencyUpdateCheckKey := types.NamespacedName{Namespace: MintMakerNamespaceName, Name: "dependencyupdatecheck-sample"}
			createDependencyUpdateCheck(dependencyUpdateCheckKey, false)
			Eventually(listJobs).WithArguments(MintMakerNamespaceName).WithTimeout(timeout).Should(HaveLen(1))

			deleteComponent(componentKey)
			deleteDependencyUpdateCheck(dependencyUpdateCheckKey)
			deleteSecret(renovateSecretKey)
		})

		It("should not trigger job for DependencyUpdateCheck CR which has been processed before", func() {
			installedRepositoryUrls := []string{
				"https://github.com/konfluxtest/repo",
			}
			github.GetAllAppInstallations = func(appIdStr string, privateKeyPem []byte) ([]github.ApplicationInstallation, string, error) {
				repositories := generateRepositories(installedRepositoryUrls)
				return []github.ApplicationInstallation{generateInstallation(repositories)}, "slug", nil
			}

			// Create a component with GitHub repository
			createNamespace(defaultNS)
			componentKey := types.NamespacedName{Namespace: defaultNS, Name: "test-component"}
			createComponent(componentKey, "test-application", "https://github.com/konfluxtest/repo", "main", "/")

			// Create a DependencyUpdateCheck CR in "mintmaker" namespace
			dependencyUpdateCheckKey := types.NamespacedName{Namespace: MintMakerNamespaceName, Name: "dependencyupdatecheck-sample"}
			createDependencyUpdateCheck(dependencyUpdateCheckKey, true)

			Eventually(listJobs).WithArguments(MintMakerNamespaceName).WithTimeout(timeout).Should(HaveLen(0))

			deleteComponent(componentKey)
			deleteDependencyUpdateCheck(dependencyUpdateCheckKey)
		})

		It("should not trigger job for DependencyUpdateCheck CR that is not from mintmaker namespace", func() {
			installedRepositoryUrls := []string{
				"https://github.com/konfluxtest/repo",
			}
			github.GetAllAppInstallations = func(appIdStr string, privateKeyPem []byte) ([]github.ApplicationInstallation, string, error) {
				repositories := generateRepositories(installedRepositoryUrls)
				return []github.ApplicationInstallation{generateInstallation(repositories)}, "slug", nil
			}

			// Create a component with GitHub repository
			createNamespace(defaultNS)
			componentKey := types.NamespacedName{Namespace: defaultNS, Name: "test-component"}
			createComponent(componentKey, "test-application", "https://github.com/konfluxtest/repo", "main", "/")

			// Create a DependencyUpdateCheck CR in "default" namespace
			dependencyUpdateCheckKey := types.NamespacedName{Namespace: defaultNS, Name: "dependencyupdatecheck-sample"}
			createDependencyUpdateCheck(dependencyUpdateCheckKey, false)

			Eventually(listJobs).WithArguments(MintMakerNamespaceName).WithTimeout(timeout).Should(HaveLen(0))

			deleteComponent(componentKey)
			deleteDependencyUpdateCheck(dependencyUpdateCheckKey)
		})

		It("should not trigger job for component has mintmaker disabled annotation", func() {
			installedRepositoryUrls := []string{
				"https://github.com/konfluxtest/repo",
			}
			github.GetAllAppInstallations = func(appIdStr string, privateKeyPem []byte) ([]github.ApplicationInstallation, string, error) {
				repositories := generateRepositories(installedRepositoryUrls)
				return []github.ApplicationInstallation{generateInstallation(repositories)}, "slug", nil
			}

			// Create a component with GitHub repository
			createNamespace(defaultNS)
			componentKey := types.NamespacedName{Namespace: defaultNS, Name: "test-component"}
			createComponent(componentKey, "test-application", "https://github.com/konfluxtest/repo", "main", "/")
			disableComponentMintmaker(componentKey)

			// Create a DependencyUpdateCheck CR in "mintmaker" namespace
			dependencyUpdateCheckKey := types.NamespacedName{Namespace: MintMakerNamespaceName, Name: "dependencyupdatecheck-sample"}
			createDependencyUpdateCheck(dependencyUpdateCheckKey, false)

			Eventually(listJobs).WithArguments(MintMakerNamespaceName).WithTimeout(timeout).Should(HaveLen(0))

			deleteComponent(componentKey)
			deleteDependencyUpdateCheck(dependencyUpdateCheckKey)
		})
	})
})
