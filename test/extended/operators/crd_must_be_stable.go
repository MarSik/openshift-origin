package operators

import (
	"context"
	"fmt"
	"strings"

	"github.com/openshift/origin/pkg/test/ginkgo/result"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/sets"

	exutil "github.com/openshift/origin/test/extended/util"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// legacyCRDSsWithUnstableVersions is a list of CRD names that were accessible-by-default when this test was created.
// No new ones should be added, because accessible-by-default APIs are guaranteed backwards compatible for customer use and
// upgradeable about forever.  If your API is ready to do this, then it's ready to be a stable version (v1), not alpha or beta.
// TODO let's get all these promoted to v1 with zero changes since we aren't going to break people and deprecate the old.
var legacyCRDSsWithUnstableVersions = map[string]sets.String{
	"consoleplugins.console.openshift.io":                             sets.NewString("v1alpha1"),
	"imagecontentsourcepolicies.operator.openshift.io":                sets.NewString("v1alpha1"),
	"machineconfignodes.machineconfiguration.openshift.io":            sets.NewString("v1alpha1"),
	"performanceprofiles.performance.openshift.io":                    sets.NewString("v1alpha1"),
	"podnetworkconnectivitychecks.controlplane.operator.openshift.io": sets.NewString("v1alpha1"),

	"helmchartrepositories.helm.openshift.io":        sets.NewString("v1beta1"),
	"machineautoscalers.autoscaling.openshift.io":    sets.NewString("v1beta1"),
	"machinehealthchecks.machine.openshift.io":       sets.NewString("v1beta1"),
	"machines.machine.openshift.io":                  sets.NewString("v1beta1"),
	"machinesets.machine.openshift.io":               sets.NewString("v1beta1"),
	"projecthelmchartrepositories.helm.openshift.io": sets.NewString("v1beta1"),
}

var _ = g.Describe("[sig-arch][Early]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("crd-check")

	g.Describe("APIs for openshift.io", func() {
		g.It("must have stable versions", func() {
			crdClient := apiextensionsclientset.NewForConfigOrDie(oc.AdminConfig())
			crdList, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().List(context.Background(), metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			failures := []string{}
			stableVersions := sets.NewString("v1", "v2", "v3", "v4", "v5") // if you're past v5, you're not stable or David is retired
			for _, crd := range crdList.Items {
				if !strings.HasSuffix(crd.Spec.Group, ".openshift.io") {
					continue
				}

				for _, versionSpec := range crd.Spec.Versions {
					// existing violations are enumerated and restricted to prevent further slipping
					if legacyViolatingVersions, ok := legacyCRDSsWithUnstableVersions[crd.Name]; ok && legacyViolatingVersions.Has(versionSpec.Name) {
						continue
					}

					if !stableVersions.Has(versionSpec.Name) {
						failures = append(failures,
							fmt.Sprintf("crd/%v has an unstable version %q that is accessible-by-default. All CRDs accessible by default must be stable (v1, v2, etc) with guaranteed compatibility and upgradeability ~forever.",
								crd.Name, versionSpec.Name))
					}
				}
			}

			if len(failures) > 0 {
				result.Flakef(strings.Join(failures, "\n"))
			}
		})
	})
})
