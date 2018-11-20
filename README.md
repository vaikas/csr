# Knative`Cloud Scheduler Source` CRD.

## Overview

This repository implements an Event Source for [Knative Eventing](http://github.com/knative/eventing)
defined with a CustomResourceDefinition (CRD). This Event Source represents
[Google Cloud Scheduler](https://cloud.google.com/scheduler/). Point is to demonstrate an Event Source that
does not live in the [Knative Eventing Sources](http://github.com/knative/eventing-sources) that can be
independently maintained, deployed and so forth.

This particular example demonstrates how to perform basic operations such as:

* Create a Cloud Scheduler Job when a Source is created
* Delete a Job when that Source is deleted
* Update a Job when the Source spec changes

## Details

Actual implementation contacts the Cloud Scheduler API and creates a Job
as specified in the CloudSechedulerSource CRD Spec. Upon success a Knative service is created
to receive calls from the Cloud Scheduler and will then forward them to the Channel.


## Purpose

Provide an Event Source that allows subscribing to Cloud Scheduler and processing them
in Knative.

Another purpose is to serve as an example of how to build an Event Source using a
[Warm Image[(https://github.com/mattmoor/warm-image) as a starting point.

## Prerequisites

1. Create a
   [Google Cloud project](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
   and install the `gcloud` CLI and run `gcloud auth login`. This sample will
   use a mix of `gcloud` and `kubectl` commands. The rest of the sample assumes
   that you've set the `$PROJECT_ID` environment variable to your Google Cloud
   project id, and also set your project ID as default using
   `gcloud config set project $PROJECT_ID`.

1. Setup [Knative Serving](https://github.com/knative/docs/blob/master/install)

1. Setup [Knative Eventing](https://github.com/knative/docs/tree/master/eventing)
   using the `release.yaml` file. This example does not require GCP.

## Create a GCP Service Account and a corresponding secret in Kubernetes

1. Create a
   [GCP Service Account](https://console.cloud.google.com/iam-admin/serviceaccounts/project).
   This sample creates one service account for both registration and receiving
   messages, but you can also create a separate service account for receiving
   messages if you want additional privilege separation.

   1. Create a new service account named `csr-source` with the following
      command:
      ```shell
      gcloud iam service-accounts create csr-source
      ```
   1. Give that Service Account the  Editor' role on your GCP project:
      ```shell
      gcloud projects add-iam-policy-binding $PROJECT_ID \
        --member=serviceAccount:csr-source@$PROJECT_ID.iam.gserviceaccount.com \
        --role roles/cloudscheduler.admin
      ```
   1. Download a new JSON private key for that Service Account. **Be sure not to
      check this key into source control!**
      ```shell
      gcloud iam service-accounts keys create csr-source.json \
        --iam-account=csr-source@$PROJECT_ID.iam.gserviceaccount.com
      ```
   1. Create a secret on the kubernetes cluster for the downloaded key. You need
      to store this key in `key.json` in a secret named `gcppubsub-source-key`

      ```shell
      kubectl -n cloudschedulersource-system create secret generic cloudschedulersource-key --from-file=key.json=csr-source.json
      ```

      The name `cloudschedulersource-key` and `key.json` are pre-configured values
      in the controller which manages your Cloud Scheduler sources.


## Install Cloud Scheduler Source

```shell
ko apply -f ./config
```


## Create a channel the events are sent to
```shell
kubectl apply -f ./channel.yaml
```

## Create a consumer for the events
```shell
ko apply -f ./subscription.yaml
```

## Wire Cloud Scheduler Events to the function 
First replace MY_GCP_PROJECT with your project id in example-csr.yaml, then create it.
```shell
kubectl apply -f ./example-csr.yaml
```

## Check that the Cloud Scheduler Job was created
```shell
gcloud beta scheduler jobs list
```

Then wait a couple of minutes and you should see events in your message dumper.

## Check that scheduler invoked the function
Note this might take couple of minutes after the creation while the Scheduler
gets going
```shell
kubectl -l 'serving.knative.dev/service=message-dumper' logs -c user-container
```
And you should see an entry like this there
```shell
vaikas@penguin:~/projects/go/src/github.com/vaikas-google/csr$ kubectl -l 'serving.knative.dev/service=message-dumper' logs -c user-container
2018/11/20 16:01:18 Message Dumper received a message: POST / HTTP/1.1
Host: message-dumper.default.svc.cluster.local
Accept-Encoding: gzip
Ce-Cloudeventsversion: 0.1
Ce-Eventid: 6b8d8507-de08-968a-a4d8-0b155151e632
Ce-Eventtime: 2018-11-20T16:01:08.817652632Z
Ce-Eventtype: GoogleCloudScheduler
Ce-Source: GCPCloudScheduler
Content-Length: 546
Content-Type: application/json
User-Agent: Go-http-client/1.1
X-B3-Parentspanid: 3d7bf0225241edaf
X-B3-Sampled: 1
X-B3-Spanid: 3fc86e7e9ebaad1f
X-B3-Traceid: ca9174f7729ea465
X-Envoy-Expected-Rq-Timeout-Ms: 60000
X-Envoy-Internal: true
X-Forwarded-For: 127.0.0.1, 127.0.0.1
X-Forwarded-Proto: http
X-Request-Id: d484098c-553d-97b7-ae11-014fca1c543f

"UE9TVCAvIEhUVFAvMS4xDQpIb3N0OiBzY2hlZHVsZXItdGVzdC5kZWZhdWx0LmFpa2FzLm9yZw0KQWNjZXB0LUVuY29kaW5nOiBnemlwLGRlZmxhdGUsYnINCkNvbnRlbnQtTGVuZ3RoOiAwDQpVc2VyLUFnZW50OiBHb29nbGUtQ2xvdWQtU2NoZWR1bGVyDQpYLUIzLVNhbXBsZWQ6IDENClgtQjMtU3BhbmlkOiBjM2ZhM2JjNTRkNDMyNDA2DQpYLUIzLVRyYWNlaWQ6IGMzZmEzYmM1NGQ0MzI0MDYNClgtRW52b3ktRXhwZWN0ZWQtUnEtVGltZW91dC1NczogNjAwMDANClgtRW52b3ktSW50ZXJuYWw6IHRydWUNClgtRm9yd2FyZGVkLUZvcjogMTAuMzYuMi4xLCAxMjcuMC4wLjENClgtRm9yd2FyZGVkLVByb3RvOiBodHRwDQpYLVJlcXVlc3QtSWQ6IDZiOGQ4NTA3LWRlMDgtOTY4YS1hNGQ4LTBiMTU1MTUxZTYzMg0KDQo="
```


### Uninstall

Simply use the same command you used to install, but with `ko delete` instead of `ko apply`.

## Usage

### Specification

The specification for a scheduler job looks like:
```yaml
apiVersion: sources.aikas.org/v1alpha1
kind: CloudSchedulerSource
metadata:
  name: scheduler-test
spec:
  googleCloudProject: quantum-reducer-434
  location: us-central1
  schedule: "every 1 mins"
  body: "{test does this work}"
  sink:
    apiVersion: eventing.knative.dev/v1alpha1
    kind: Channel
    name: scheduler-demo
```

### Creation

With the above in `foo.yaml`, you would create the Cloud Scheduler Job with:
```shell
kubectl create -f foo.yaml
```

### Listing

You can see what Cloud Scheduler Jobs have been created:
```shell
$ kubectl get cloudschedulersources
NAME             AGE
scheduler-test   4m
```

### Updating

You can's upgrade `foo.yaml` jobs yet because the reconciler doesn't work yet :(
But if it did, you'd do:
```shell
kubectl replace -f foo.yaml
```

### Removing

You can remove a Cloud Scheduler jobs via:
```shell
kubectl delete cloudschedulersources scheduler-test
```

