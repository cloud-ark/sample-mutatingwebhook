Kubernetes MutatingWebhook Example with v1 version of AdmissionRegistration API
===============================================================================

This is an example of creating Kubernetes MutatingWebhook with v1 version of
the AdmissionRegistration API - admissionregistration.k8s.io/v1

The mutating webhook endpoint is registered using a self-signed CA certificate.

The webhook Pod is created as a set of two containers - an init container and a hook container.
The init container creates the self-signed CA, server key/certificate, 
registers the mutatingwebhookconfiguration object, and creates a Secret
object with the server key/certificate.
The hook container mounts this Secret as a Volume and serves the webhook endpoint.

Check mutatingwebhookconfiguration.yaml to see the Kubernetes resources and actions
that will be intercepted by this mutating webhook.


Background:
------------

At CloudARK, our [KubePlus Operator](https://github.com/cloud-ark/kubeplus) depends
on a working MutatingWebhook setup for its operation. When we started building KubePlus,
the AdmissionRegistration API was still in ```v1beta1``` version. We built our webhook
using the spec properties and features available in that version. This worked till Kubernetes
versions < 1.22 were still around in public cloud providers. Lately though, we started
observing that public cloud providers have moved to Kubernetes versions 1.22 and above.
These versions do not serve the v1beta1 version of the AdmissionRegistration API anymore.
So we had to migrate our webhook to use the v1 version of AdmissionRegistration API.

The path to reach there was not straightforward. When we had built the original webhook,
we had depended on the excellent example available at [1]. So our first approach was to
try to modify that to use the v1 API instead of the v1beta1 API. However, we ran into several
problems in this approach. The v1 registration API has made certain fields in the CSR object
compulsory. One of them is the signerName. We tried using ```kubernetes.io/kubelet-serving```,
```kubernetes.io/kube-apiserver-client```. But both of these did not work. Turns out the simplest
approach is to create a self-signed CA and use it's certificate to sign the key of the webhook server. 
Through referring to [2, 3,4] and through trial and error we were able to create this certificate.


Steps to test:
--------------
1. Create a Minikube cluster with Kubernetes version >= v1.22.0 
```
minikube start
eval $(minikube docker-env)
```

2. Check that the cluster has admissionregistration.k8s.io/v1 endpoint
```
kubectl api-versions
```

3. Install Golang >= 1.12 

4. Set Environment Variables
```
GO111MODULE=on
GOPATH=/home/vagrant/go
GOOS=linux
``` 

5. Build the executable
```
go get github.com/googleapis/gnostic@v0.4.0
go build .
```

6. Build containers
```
docker build -t mwh -f Dockerfile.mwh .
docker build -t mwh-setup .
```

7. Deploy the Webhook
```
kubectl create -f deploy-mwh.yaml 
until kubectl get pods | grep test-mwh-deployment | grep Running; do echo "Waiting for MutatingWebhook Pod to become ready"; sleep 1; done
```

8. Test creating a namespace
```
kubectl create ns ns1
```

9. Verify that the create request was intercepted
```
kubectl get pods | grep test-mwh-deployment | awk '{print $1}' | xargs kubectl logs -c crd-hook
```

10. Cleanup 
```
kubectl delete mutatingwebhookconfigurations test-mwh
kubectl delete -f deploy-mwh.yaml
```

Tested on:
----------
1. Minikube - Server version v1.21.0, July 28, 2022


Contributions:
--------------
If you try the code, we would love to hear from you. Please consider opening a PR with the information
about the configuration that you tested on. We would like to improve the list of Tested on platforms.


References:
------------
[1] https://github.com/morvencao/kube-mutating-webhook-tutorial/blob/master/deployment/webhook-create-signed-cert.sh
[2] https://github.com/morvencao/kube-sidecar-injector
[3] https://www.funkypenguin.co.nz/blog/self-signed-certificate-on-mutating-webhook-requires-double-encryption/
[4] https://github.com/kubernetes/kubernetes/issues/61171

