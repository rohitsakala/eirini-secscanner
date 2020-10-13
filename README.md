# eirini-secscanner
EiriniX extension for the cf 2020 summit lab (content should go eventually to https://github.com/cloudfoundry/summit-hands-on-labs)


In this lab, we will write an extension for Eirini with EiriniX.

Our extension is a security-oriented one. We want to prevent from being pushed Eirini apps that doesn't pass a vulnerability scan.

For example, we could run [trivy](https://github.com/aquasecurity/trivy#embed-in-dockerfile) in an `InitContainer` and make the pod crash before starting it up if tests fails.

## Target Audience

This lab is targeted towards the audience who would like to use Cloud Foundry for packaging and deploying applications and Kubernetes as the underlying infrastructure for orchestration of the containers.

## Learning Objectives

 - Build an Eirini extension with EiriniX
 - Deploy the extension to your kubernetes cluster

## Prerequisites
- A machine with golang installed, were we will build our extension
- A KubeCF Cluster deployed with Eirini
- Students must have basic knowledge of Cloud Foundry and Kubernetes.


## 1) Setup your go environment

Be sure you have an environment where you can build golang source code, see [here for golang installation](https://golang.org/doc/install), and check that your environment can compile the [go hello world program](https://golang.org/doc/tutorial/getting-started). Note the tutorial needs an environment with `Go >=1.14`.

## 2) Setup go.mod for our project

First of all, create a new folder, and init it with your project path:

```$ go mod init github.com/user/eirini-secscanner```

At this point, we can run ```go get code.cloudfoundry.org/eirinix``` at the top level, so it gets added to go.mod.

You should have a go.mod similar to this one:

```golang
module github.com/user/eirini-secscanner

require (
	code.cloudfoundry.org/eirinix v0.3.1-0.20200908072226-2c03042398ea
	go.uber.org/zap v1.15.0
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	sigs.k8s.io/controller-runtime v0.6.2
)

go 1.14
```

## 3)  Extension logic

Before jumping in creating our `main.go`, let's focus on our extension logic. EiriniX has support for different kind of extensions, which allows to interact with Eirini applications, or staging pods in different ways:

- MutatingWebhooks -  by having an active component which patches Eirini apps before starting them
- Watcher - a component that just watch and gets notified if new Eirini apps are pushed
- Reconcilers - a component that constantly reconciles a desired state for an Eirini app

An Eirini App workload is represented by a statefulset, which then it becomes a pod living in the Eirini namespace. 

Before the app is started, Eirini runs a staging joob which builds the image used to start the app.

For our Security scanner (secscanner) makes sense to use a *MutatingWebhook*, we will try to patch the Eirini runtime pod and inject an InitContainer with [trivy](https://github.com/aquasecurity/trivy#embed-in-dockerfile) preventing to starting it in case has security vulnerability.

Since we want [trivy](https://github.com/aquasecurity/trivy#embed-in-dockerfile) to run as a first action, and check if the filesystem of our app is secure enough, we will have to run the InitContainer with the same image which is used for the Eirini app.

So our extension will also have to retrieve the image of the Eirini app - and use that one to run the security scanner.

