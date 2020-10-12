# eirini-secscanner
EiriniX extension for the cf 2020 summit lab


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