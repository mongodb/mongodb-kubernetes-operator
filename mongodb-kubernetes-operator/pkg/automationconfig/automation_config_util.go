package automationconfig

import (
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	"math"
	"github.com/blang/semver"
	"github.com/spf13/cast"
	"strings"
)

// getDnsForStatefulSet returns hostnames and names of pods in stateful set "set". This is a preferred way of getting hostnames
// it must be always used if it's possible to read the statefulset from Kubernetes
func getDnsForStatefulSet(set appsv1.StatefulSet, clusterName string) ([]string, []string) {
	return getDNSNames(set.Name, set.Spec.ServiceName, set.Namespace, clusterName, int(*set.Spec.Replicas))
}

// calculateWiredTigerCache returns the cache that needs to be dedicated to mongodb engine.
// This was fixed in SERVER-16571 so we don't need to enable this for some latest version of mongodb (see the ticket)
func calculateWiredTigerCache(set appsv1.StatefulSet, version string) *float32 {
	shouldCalculate, err := versionMatchesRange(version, ">=4.0.0 <4.0.9 || <3.6.13")

	if err != nil || shouldCalculate {
		// Note, that if the limit is 0 then it's not specified in fact (unbounded)
		if memory := set.Spec.Template.Spec.Containers[0].Resources.Limits.Memory(); memory != nil && (*memory).Value() != 0 {
			// Value() returns size in bytes so we need to transform to Gigabytes
			wt := cast.ToFloat64((*memory).Value()) / 1000000000
			// https://docs.mongodb.com/manual/core/wiredtiger/#memory-use
			wt = math.Max((wt-1)*0.5, 0.256)
			// rounding fractional part to 3 digits
			rounded := float32(math.Floor(wt*1000) / 1000)
			return &rounded
		}
	}
	return nil
}

func versionMatchesRange(version, vRange string) (bool, error) {
	v, err := semver.Parse(version)
	if err != nil {
		return false, err
	}
	expectedRange, err := semver.ParseRange(vRange)
	if err != nil {
		return false, err
	}
	return expectedRange(v), nil
}

// TODO: The cluster domain is not known inside the Kubernetes cluster,
// so there is no API to obtain this name from the operator.
// * See: https://github.com/kubernetes/kubernetes/issues/44954
func getDnsTemplateFor(name, service, namespace, clusterDomain string) string {
	if clusterDomain == "" {
		clusterDomain = "cluster.local"
	}
	dnsTemplate := fmt.Sprintf("%s-{}.%s.%s.svc.%s", name, service, namespace, clusterDomain)
	return strings.Replace(dnsTemplate, "{}", "%d", 1)
}

// GetDnsNames returns hostnames and names of pods in stateful set, it's less preferable than "getDnsForStatefulSet" and
// should be used only in situations when statefulset doesn't exist any more (the main example is when the mongodb custom
// resource is being deleted - then the dependant statefulsets cannot be read any more as they get into Terminated state)
func getDNSNames(statefulSetName, service, namespace, clusterName string, replicas int) (hostnames, names []string) {
	mName := getDnsTemplateFor(statefulSetName, service, namespace, clusterName)
	hostnames = make([]string, replicas)
	names = make([]string, replicas)

	for i := 0; i < replicas; i++ {
		hostnames[i] = fmt.Sprintf(mName, i)
		names[i] = getPodName(statefulSetName, i)
	}

	return
}

func getPodName(name string, idx int) string {
	return fmt.Sprintf("%s-%d", name, idx)
}

func compareVersions(version1, version2 string) (int, error) {
	v1, err := semver.Make(version1)
	if err != nil {
		return 0, err
	}
	v2, err := semver.Make(version2)
	if err != nil {
		return 0, err
	}
	return v1.Compare(v2), nil
}

