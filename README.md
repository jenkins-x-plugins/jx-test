# jx-test

[![Documentation](https://godoc.org/github.com/jenkins-x/jx-test?status.svg)](https://pkg.go.dev/mod/github.com/jenkins-x/jx-test)
[![Go Report Card](https://goreportcard.com/badge/github.com/jenkins-x/jx-test)](https://goreportcard.com/report/github.com/jenkins-x/jx-test)
[![Releases](https://img.shields.io/github/release-pre/jenkins-x/jx-test.svg)](https://github.com/jenkins-x/jx-test/releases)
[![LICENSE](https://img.shields.io/github/license/jenkins-x/jx-test.svg)](https://github.com/jenkins-x/jx-test/blob/master/LICENSE)
[![Slack Status](https://img.shields.io/badge/slack-join_chat-white.svg?logo=slack&style=social)](https://slack.k8s.io/)

`jx-test` is a small command line tool working with Kubernetes based (system tests, integration tests, BDD tests etc) with Terraform and the [Terraform Operator](http://tf.isaaguilar.com/)

## Getting Started

Download the [jx-test binary](https://github.com/jenkins-x/jx-test/releases) for your operating system and add it to your `$PATH`.

Or you can use `jx test` directly in the [Jenkins X 3.x CLI](https://github.com/jenkins-x/jx-cli)


### Creating a Terraform System Test

First you need to create a `Terraform` resource which can use Go / Helm style templating of values.

e.g.

```yaml 
apiVersion: tf.isaaguilar.com/v1alpha1
kind: Terraform
spec:
  env:
  - name: TF_VAR_jx_git_url
    value: https://github.com/jenkins-x-bdd/cluster-{{ .Env.TF_VAR_cluster_name }}-dev.git
  - name: TF_VAR_jx_bot_username
    value: jenkins-x-bot-bdd
  - name: TF_VAR_jx_bot_token
    valueFrom:
      secretKeyRef:
        name: bdd-git
        key: password
{{- range $pkey, $pval := .Env }}
  - name: {{ $pkey }}
    value: {{ quote $pval }}
{{- end }}

  scmAuthMethods:
  - host: github.com
    git:
      https:
        requireProxy: false
        tokenSecretRef:
          name: bdd-git
          namespace: jx
          key: password

  terraformRunner: ghcr.io/jenkins-x/terraform-operator-gcp
  terraformVersion: 0.0.6

  terraformModule:
    address: https://github.com/jenkins-x-bdd/infra-{{ .Env.TF_VAR_cluster_name }}-dev

  customBackend: |-
    terraform {
      backend "kubernetes" {
        secret_suffix = "{{ .Name }}-state"
        namespace = "{{ .Namespace }}"
        in_cluster_config = true
      }
    }

  serviceAccount: tekton-bot

  applyOnCreate: true
  applyOnUpdate: true
  applyOnDelete: true
  ignoreDelete: false


  postrunScript: |-
    #!/usr/bin/env bash
    echo "Terraform is done!"

    echo "lets connect to the remote cluster"
    $(terraform output -raw connect)
    
    # TODO now do more tests...
```

You then run a Terraform based test by creating an instance of the resource passing in the template values:

```bash 

export TF_VAR_gcp_project=myproject
export TF_VAR_cluster_name=mycluster
jx test create --file tf.yaml
```

This command will:

* Create the `Terraform` resource for the `tf.yaml` file after rendering the template and substituting in any variables for the current pipeline, build number, PR and so forth.

* The [Terraform Operator](http://tf.isaaguilar.com/)  detects the `Terraform` and will create a `Job` to perform the `terraform apply` and then run any `postrunScript:` scripts

* The terminal will tail the output of this Job and pass/fail based on the Job
   

## Viewing active test


```bash 
kubectl get terraform 
```

or the more brief:

```bash 
kubectl get tf 
```

## Garbage collecting failed tests

Run the following command periodically:

```bash 
jx test gc
```

## Keeping failed tests

If a test fails and you need time to investigate you can label the Terraform resource to ensure it doesn't get garbage collected as follows

Run the following command periodically:

```bash 
kubectl label terraform mytest keep=yes
```
      
When you are ready to remove the test case resources do:


```bash 
kubectl delete terraform mytest
```




## Commands

See the [jx-test command reference](https://github.com/jenkins-x/jx-test/blob/master/docs/cmd/jx-test.md)