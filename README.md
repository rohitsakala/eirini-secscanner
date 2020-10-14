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
- Github account / Git experience


## 1) Setup your go environment

Be sure you have an environment where you can build golang source code, see [here for golang installation](https://golang.org/doc/install), and check that your environment can compile the [go hello world program](https://golang.org/doc/tutorial/getting-started). Note the tutorial needs an environment with `Go >=1.14`.

## 2) Preparation

### 2.1) Setup go.mod for our project

First of all, create a new folder, and init it with your project path:

```$ go mod init github.com/user/eirini-secscanner```

At this point, we can run ```go get code.cloudfoundry.org/eirinix``` at the top level, so it gets added to go.mod.

You should have a go.mod similar to this one:

```golang
module github.com/[USER]/eirini-secscanner # NOTE: Replace [USER] with your username here

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

### 2.2) Prepare GitHub repository

For easy of use, we will use GitHub to store our extension with git, and we will use github actions to build the docker image of our extension. In this way, we can later deploy our extension with `kubectl` in our cluster. 

Create a GitHub account if you don't have one yet, create a new repository and [create a Personal Access Token (PAT)](https://docs.github.com/en/free-pro-team@latest/github/authenticating-to-github/creating-a-personal-access-token) in GitHub with the [appropriate permissions](https://docs.github.com/en/free-pro-team@latest/packages/getting-started-with-github-container-registry/migrating-to-github-container-registry-for-docker-images#authenticating-with-the-container-registry) and add a secret in the repository, called `CR_PAT` with the PAT key. For sake of semplicity, we will assume that our repository is called `eirini-secscanner`.

Clone the repository, and create a `.github` folder, inside create a new `workflows` folder with a yaml file `docker.yaml` with the following content:

*.github/workflows/docker.yaml*: 
```yaml
name: Publish Docker image
on:
  push:
jobs:
  push_to_registry:
    name: Push Docker image to GitHub Packages
    runs-on: ubuntu-latest
    steps:
      - name: Check out the repo
        uses: actions/checkout@v2
      -
        name: Set up QEMU
        uses: docker/setup-qemu-action@v1
      -
        name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v1
      -
        name: Login to GitHub Container Registry
        uses: docker/login-action@v1
        with:
          registry: ghcr.io
          username: ${{ github.repository_owner }}
          password: ${{ secrets.CR_PAT }}
      -
        name: Build and push
        id: docker_build
        uses: docker/build-push-action@v2
        with:
          push: true
          tags: ghcr.io/[USER]/eirini-secscanner:latest # NOTE: Replace [USER] with your username here
      -
        name: Image digest
        run: echo ${{ steps.docker_build.outputs.digest }}
```

The GitHub Action will build and push a fresh docker image to the GitHub container registry, that we can later on use it in our cluster to run our extension. 
The Image should be accessible to a url similar to this: `ghcr.io/user/eirini-secscanner:latest`

## 3)  Extension logic

Before jumping in creating our `main.go`, let's focus on our extension logic. EiriniX has support for different kind of extensions, which allows to interact with Eirini applications, or staging pods in different ways:

- MutatingWebhooks -  by having an active component which patches Eirini apps before starting them
- Watcher - a component that just watch and gets notified if new Eirini apps are pushed
- Reconcilers - a component that constantly reconciles a desired state for an Eirini app

An Eirini App workload is represented by a ```StatefulSet```, which then it becomes a pod running in the Eirini namespace. 

Before the app is started, Eirini runs a staging job which builds the image used to start the app.

For our Security scanner (secscanner) makes sense to use a *MutatingWebhook*, we will try to patch the Eirini runtime pod and inject an InitContainer with [trivy](https://github.com/aquasecurity/trivy#embed-in-dockerfile) preventing to starting it in case has security vulnerability.

Since we want [trivy](https://github.com/aquasecurity/trivy#embed-in-dockerfile) to run as a first action, and check if the filesystem of our app is secure enough, we will have to run the InitContainer with the same image which is used for the Eirini app.

So our extension will also have to retrieve the image of the Eirini app - and use that one to run the security scanner.

### Anatomy of an Extension

[EiriniX Extensions](https://github.com/cloudfoundry-incubator/eirinix#write-your-extension) which are *MutatingWebhooks* are expected to provide a *Handle* method which receives a request from the Kubernetes API. The request contains
the pod definition that we want to mutate, so our extension will start by defining a struct:


```golang
package secscanner

type Extension struct{}
```

Our extension needs a `Handle` method, so we can write:

```golang
func (ext *Extension) Handle(ctx context.Context, eiriniManager eirinix.Manager, pod *corev1.Pod, req admission.Request) admission.Response {

	if pod == nil {
		return admission.Errored(http.StatusBadRequest, errors.New("No pod could be decoded from the request"))
  }

	return eiriniManager.PatchFromPod(req, pod)
}

```

Note we need to add a bunch of imports, as our new `Handle` method receives structures from other packages:

-  ```ctx``` is the request context, that can be used for background operations. 
- ```eiriniManager``` is EiriniX, it's an instance of the current execution. 
- ```pod``` is the Pod that needs to be patched - in our case will be our Eirini Extension
- ```req``` is the raw admission request, might be useful for furhter inspection, but we won't use it in our case
- ```eiriniManager.PatchFromPod(req, pod)``` is computing the diff between the raw request and the pod. It's used to return the actual difference we are introducing in the pod

As it stands our extension is not much useful, let's make it add a new init container:


```golang

func (ext *Extension) Handle(ctx context.Context, eiriniManager eirinix.Manager, pod *corev1.Pod, req admission.Request) admission.Response {

	if pod == nil {
		return admission.Errored(http.StatusBadRequest, errors.New("No pod could be decoded from the request"))
	}
	podCopy := pod.DeepCopy()

	secscanner := v1.Container{
		Name:            "secscanner",
		Image:           "busybox",
		Args:            []string{"echo 'fancy'"},
		Command:         []string{"/bin/sh", "-c"},
		ImagePullPolicy: v1.PullAlways,
		Env:             []v1.EnvVar{},
	}

	podCopy.Spec.InitContainers = append(podCopy.Spec.InitContainers, secscanner)

	return eiriniManager.PatchFromPod(req, podCopy)
}
```

We have added a bunch of things, let's go over it one by one:

- ```podCopy := pod.DeepCopy()``` creates a copy of the pod, to operate over a copy instead of a real pointer
- ```secscanner....``` it's our `InitContainer` definition. It contains the `Name`, `Image`, and `Args` fields along with `Commands`. As for now it doesn't do anything useful, but it's a start point so we can experience with our extension.
- ```podCopy.Spec.InitContainers = append(podCopy.Spec.InitContainers, secscanner)``` is appending the InitContainer to the list of the containers in ```podCopy```
- ```return eiriniManager.PatchFromPod(req, podCopy)``` returns the diff patch from the request to the podCopy
