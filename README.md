# jx-test

[![Documentation](https://godoc.org/github.com/jenkins-x/jx-test?status.svg)](https://pkg.go.dev/mod/github.com/jenkins-x/jx-test)
[![Go Report Card](https://goreportcard.com/badge/github.com/jenkins-x/jx-test)](https://goreportcard.com/report/github.com/jenkins-x/jx-test)
[![Releases](https://img.shields.io/github/release-pre/jenkins-x/jx-test.svg)](https://github.com/jenkins-x/jx-test/releases)
[![LICENSE](https://img.shields.io/github/license/jenkins-x/jx-test.svg)](https://github.com/jenkins-x/jx-test/blob/master/LICENSE)
[![Slack Status](https://img.shields.io/badge/slack-join_chat-white.svg?logo=slack&style=social)](https://slack.k8s.io/)

`jx-test` is a small command line tool working with Kubernetes tests (system tests, integration tests, BDD tests etc)

## Getting Started

Download the [jx-test binary](https://github.com/jenkins-x/jx-test/releases) for your operating system and add it to your `$PATH`.

Or you can use `jx test` directly in the [Jenkins X 3.x CLI](https://github.com/jenkins-x/jx-cli)


### Installing the `TestRun` CRD

To be able to use the [jx-test commands](https://github.com/jenkins-x/jx-test/blob/master/docs/cmd/jx-test.md) you will need to install the `TestRun` CRD in your kubernetes cluster...

```bash 
kubectl apply -f https://raw.githubusercontent.com/jenkins-x/jx-cli/master/crds/test-crd.yaml
```

## Commands

See the [jx-test command reference](https://github.com/jenkins-x/jx-test/blob/master/docs/cmd/jx-test.md)