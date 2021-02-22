module github.com/jenkins-x-plugins/jx-test

require (
	github.com/Masterminds/sprig/v3 v3.2.0
	github.com/cpuguy83/go-md2man v1.0.10
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/jenkins-x/jx-helpers/v3 v3.0.81
	github.com/jenkins-x/jx-logging/v3 v3.0.3
	github.com/onsi/gomega v1.8.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	k8s.io/api v0.20.4 // indirect
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v11.0.0+incompatible
	sigs.k8s.io/yaml v1.2.0
)

replace (
	k8s.io/api => k8s.io/api v0.20.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.2
	k8s.io/client-go => k8s.io/client-go v0.20.2
)

go 1.15
