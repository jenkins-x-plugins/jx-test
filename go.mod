module github.com/jenkins-x-plugins/jx-test

require (
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/cpuguy83/go-md2man v1.0.10
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/jenkins-x/jx-helpers/v3 v3.0.116
	github.com/jenkins-x/jx-logging/v3 v3.0.6
	github.com/onsi/gomega v1.8.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	k8s.io/apimachinery v0.20.7
	k8s.io/client-go v11.0.0+incompatible
	sigs.k8s.io/structured-merge-diff/v4 v4.0.3 // indirect
	sigs.k8s.io/yaml v1.2.0
)

replace (
	// helm dependencies
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible

	k8s.io/api => k8s.io/api v0.20.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.2
	k8s.io/client-go => k8s.io/client-go v0.20.2
)

go 1.15
