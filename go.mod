module github.com/openshift/managed-node-metadata-operator

go 1.16

require (
	github.com/golang/mock v1.6.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/openshift/machine-api-operator v0.2.1-0.20211203013047-383c9b959b69
	k8s.io/api v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/client-go v0.23.0
	k8s.io/klog/v2 v2.30.0
	sigs.k8s.io/controller-runtime v0.11.1
)

require (
	golang.org/x/net v0.0.0-20211209124913-491a49abca63 // indirect
	k8s.io/utils v0.0.0-20211116205334-6203023598ed // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.1 // indirect
)

replace sigs.k8s.io/cluster-api-provider-aws => github.com/openshift/cluster-api-provider-aws v0.2.1-0.20210622023641-c69a3acaee27

replace sigs.k8s.io/cluster-api-provider-azure => github.com/openshift/cluster-api-provider-azure v0.1.0-alpha.3.0.20211202014309-184ccedc799e
