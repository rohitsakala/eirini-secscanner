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
package main

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


## 4) write the main.go

Let's now write a short `main.go` which just executes our extension:

```golang

package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	eirinix "code.cloudfoundry.org/eirinix"
	"go.uber.org/zap"
)

const operatorFingerprint = "eirini-secscanner"

var appVersion string = ""

func main() {

	eiriniNsEnvVar := os.Getenv("EIRINI_NAMESPACE")
	if eiriniNsEnvVar == "" {
		zaplog.Fatal("the EIRINI_NAMESPACE environment variable must be set")
	}

	webhookNsEnvVar := os.Getenv("EXTENSION_NAMESPACE")
	if webhookNsEnvVar == "" {
		zaplog.Fatal("the EXTENSION_NAMESPACE environment variable must be set")
	}

	portEnvVar := os.Getenv("PORT")
	if portEnvVar == "" {
		zaplog.Fatal("the PORT environment variable must be set")
	}
	port, err := strconv.Atoi(portEnvVar)
	if err != nil {
		zaplog.Fatalw("could not convert port to integer", "error", err, "port", portEnvVar)
	}

	serviceNameEnvVar := os.Getenv("SERVICE_NAME")
	if serviceNameEnvVar == "" {
		zaplog.Fatal("the SERVICE_NAME environment variable must be set")
	}

	filter := true

	ext := eirinix.NewManager(eirinix.ManagerOptions{
		Namespace:           eiriniNsEnvVar,
		Host:                "0.0.0.0",
		Port:                int32(port),
		FilterEiriniApps:    &filter,
		OperatorFingerprint: operatorFingerprint,
		ServiceName:         serviceNameEnvVar,
		WebhookNamespace:    webhookNsEnvVar,
	})

	ext.AddExtension(&Extension{})

	if err := ext.Start(); err != nil {
		fmt.Println("error starting eirinix manager", "error", err)
	}

}

```

First we collect options from the environment. This will allow us to tweak easily from the kubernetes deployment the various fields:
- We grab `EIRINI_NAMESPACE` from the environment, it's the namespace used by Eirini to push App
- `EXTENSION_NAMESPACE` is the namespace used by our extension
- `PORT` is the listening port where our extension is listening to
- `SERVICE_NAME` is the Kubernetes service name reserved to our extension. We will need a Kubernetes service resource created before starting our extension. It will be used by Kubernetes to contact our extension while mutating Eirini apps.

Mext we construct the EiriniX manager, which will run our extension under the hood, and will create all the necessary boilerplate resources to talk to Kubernetes:
```golang

filter := true

	ext := eirinix.NewManager(eirinix.ManagerOptions{
		Namespace:           eiriniNsEnvVar,
		Host:                "0.0.0.0",
		Port:                int32(port),
		FilterEiriniApps:    &filter,
		OperatorFingerprint: operatorFingerprint,
		ServiceName:         serviceNameEnvVar,
		WebhookNamespace:    webhookNsEnvVar,
	})
```

Here we just map the settings that we collected in environment variables, that we hand over to EiriniX. The ```OperatorFingerprint```  and ```FilterEiriniApps``` are used to set a fingerprint for our runtime and for filtering eirini apps only respectively.

## 5) Commit the code

Time to try things out!

Commmit and push the code done so far to github, a workflow will trigger automatically, which can be inspected in the "Actions" tab of the repository. 
Now, we should have a docker image, and we are ready to start our extension!


### Make the Docker image public

After GH Action has been executed and the docker image of your extension has been pushed, change its permission setting to public in the [package settings page](https://docs.github.com/en/free-pro-team@latest/packages/managing-container-images-with-github-container-registry/configuring-access-control-and-visibility-for-container-images#configuring-visibility-of-container-images-for-your-personal-account)

## 6) Kube apply, first cluster tests

We need at this point to start our extension, so we will create a file which represent our deployment for kubernetes:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: eirini-secscanner
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: eirini-secscanner
  namespace: eirini-secscanner
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: eirini-secscanner-webhook
rules:
- apiGroups:
  - admissionregistration.k8s.io
  resources:
  - validatingwebhookconfigurations
  - mutatingwebhookconfigurations
  verbs:
  - create  
  - delete
  - update
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: eirini-secscanner-secrets
rules:
- apiGroups:
  - ""
  resources:
  - secrets
  verbs:
  - get
  - create
  - delete
  - list
  - update
  - watch
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: eirini-secscanner
rules:
- apiGroups:
  - ""
  resources:
  - namespaces
  verbs:
  - get
  - list
  - update
  - watch

- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - patch
  - update

- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - delete
  - get
  - list
  - update
  - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: watch-eirini-1
  namespace: eirini
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: eirini-secscanner
subjects:
- kind: ServiceAccount
  name: eirini-secscanner
  namespace: eirini-secscanner
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: secrets
  namespace: eirini-secscanner
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: eirini-secscanner-secrets
subjects:
- kind: ServiceAccount
  name: eirini-secscanner
  namespace: eirini-secscanner
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: webhook
  namespace: eirini-secscanner
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: eirini-secscanner-webhook
subjects:
- kind: ServiceAccount
  name: eirini-secscanner
  namespace: eirini-secscanner
---
apiVersion: v1
kind: Service
metadata:
  name: eirini-secscanner
  namespace: eirini-secscanner
spec:
  type: ClusterIP
  selector:
    name: eirini-secscanner
  ports:
  - protocol: TCP
    name: https
    port: 8080
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: eirini-secscanner
  namespace: eirini-secscanner
spec:
  replicas: 1
  selector:
    matchLabels:
      name: eirini-secscanner
  template:
    metadata:
      labels:
        name: eirini-secscanner
    spec:
      serviceAccountName: eirini-secscanner
      containers:
        - name: eirini-secscanner
          imagePullPolicy: Always
          image: "ghcr.io/mudler/eirini-secscanner:latest"
          env:
            - name: EIRINI_NAMESPACE
              value: "eirini"
            - name: EXTENSION_NAMESPACE
              value: "eirini-secscanner"
            - name: PORT
              value: "8080"
            - name: SERVICE_NAME
              value: "eirini-secscanner"
            - name: SEVERITY
              value: "CRITICAL"
```

Mind to replace `"ghcr.io/[USER]/eirini-secscanner:latest"` with your image, and then apply the yaml with `kubectl`. Our component will be now on the `eirini-secscanner` namespace, intercepting Eirini Apps.

## 7) Extension logic, part two.

We have tried our extension, but doesn't do anything useful - yet. So let's implement what we was aheading for - a secscanner.

This time,  we will inject a container, but the container will have the image of the running App, so we will try to scan the pod that we have intercepted, and we will try to find the container that Eirini created to start our application.


```golang

func (ext *Extension) Handle(ctx context.Context, eiriniManager eirinix.Manager, pod *corev1.Pod, req admission.Request) admission.Response {

	if pod == nil {
		return admission.Errored(http.StatusBadRequest, errors.New("No pod could be decoded from the request"))
	}
	podCopy := pod.DeepCopy()

	var image string
	for i := range podCopy.Spec.Containers {
		c := &podCopy.Spec.Containers[i]
		switch c.Name {
		case "opi":
			image = c.Image
		}
  }
  ....
```

Now we are looping `podCopy` Containers, and we are finding for a container which is named after `opi` - that's by convention the container named by Eirini running your app. We will grab the image string and we store it to `image`.

Knowing the correct image, now we can Inject our container:

```golang


	secscanner := v1.Container{
		Name:            "secscanner",
		Image:           image,
		Args:            []string{`mkdir bin && curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/master/contrib/install.sh | sh -s -- -b bin && bin/trivy filesystem --exit-code 1 --no-progress /`},
		Command:         []string{"/bin/sh", "-c"},
		ImagePullPolicy: v1.PullAlways,
		Env:             []v1.EnvVar{},
	}
```

As we would like also to be able to run our extension with replicas, in full HA mode, we will adapt our code to be idempotent, so it doesn't try to inject an init container each time. Before injecting the container, we can add:

```golang


	// Stop if a secscanner was already injected
	for i := range podCopy.Spec.InitContainers {
		c := &podCopy.Spec.InitContainers[i]
		if c.Name == "secscanner" {
			return eiriniManager.PatchFromPod(req, podCopy)
		}
	}


```
To return an empty patch , so we don't patch the pod twice (or more).

Now our extension should look something like: 

```golang

func trivyInject(severity string) string {
	return fmt.Sprintf("curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/master/contrib/install.sh | sh -s -- -b tmp && tmp/trivy filesystem --severity '%s' --exit-code 1 --no-progress /", severity)
}
// Extension is the secscanner extension which injects a initcontainer which checks for vulnerability in the container image
type Extension struct{}

func (ext *Extension) Handle(ctx context.Context, eiriniManager eirinix.Manager, pod *corev1.Pod, req admission.Request) admission.Response {

	if pod == nil {
		return admission.Errored(http.StatusBadRequest, errors.New("No pod could be decoded from the request"))
	}
	podCopy := pod.DeepCopy()

	// Stop if a secscanner was already injected
	for i := range podCopy.Spec.InitContainers {
		c := &podCopy.Spec.InitContainers[i]
		if c.Name == "secscanner" {
			return eiriniManager.PatchFromPod(req, podCopy)
		}
	}

	var image string
	for i := range podCopy.Spec.Containers {
		c := &podCopy.Spec.Containers[i]
		switch c.Name {
		case "opi":
			image = c.Image
		}
	}

	secscanner := v1.Container{
		Name:            "secscanner",
		Image:           image,
		Args:            []string{trivyInject("CRITICAL")},
		Command:         []string{"/bin/sh", "-c"},
		ImagePullPolicy: v1.PullAlways,
		Env:             []v1.EnvVar{},
	}

	podCopy.Spec.InitContainers = append(podCopy.Spec.InitContainers, secscanner)

	return eiriniManager.PatchFromPod(req, podCopy)
}


```

We have just moved the bash commmand construction to its own function `trivyInject` so it can take a severity as an option.

Let's commit the code and push it, to have a new image built by GitHub.

Have a look at the complete source code, `extension.go` in this repository.

Kill and delete the extension pod and push an application to see what happens