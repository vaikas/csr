# Kubernetes `Cloud Scheduler Source` CRD.

## Overview

This repository implements an Event Source for (Knative Eventing)[http://github.com/knative/eventing]
defined with a CustomResourceDefinition (CRD). This Event Source represents
(Google Cloud Scheduler)[https://cloud.google.com/scheduler/]. Point is to demonstrate an Event Source that
does not live in the (Knative Eventing Sources)[http://github.com/knative/eventing-sources] that can be
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
(Kubernetes Sample Controller)[https://github.com/kubernetes/sample-controller] as a starting point.

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


### Uninstall

Simply use the same command you used to install, but with `ko delete` instead of `ko apply`.

## Usage

### Specification

The specification for an image to "warm up" looks like:
```yaml
apiVersion: mattmoor.io/v2
kind: WarmImage
metadata:
  name: example-warmimage
spec:
  image: gcr.io/google-appengine/debian8:latest
  # Optionally:
  # imagePullSecrets: 
  # - name: foo
```

### Creation

With the above in `foo.yaml`, you would install the image with:
```shell
kubectl create -f foo.yaml
```

### Listing

You can see what images are "warm" via:
```shell
$ kubectl get warmimages
NAME                KIND
example-warmimage   WarmImage.v2.mattmoor.io
```

### Updating

You can upgrade `foo.yaml` to `debian9` and run:
```shell
kubectl replace -f foo.yaml
```

### Removing

You can remove a warmed image via:
```shell
kubectl delete warmimage example-warmimage
```

