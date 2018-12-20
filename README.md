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

2. Configure [static IP](https://github.com/knative/docs/blob/master/serving/gke-assigning-static-ip-address.md)

1. Configure [custom dns](https://github.com/knative/docs/blob/master/serving/using-a-custom-domain.md)

1. Configure [outbound network access](https://github.com/knative/docs/blob/master/serving/outbound-network-access.md)

1. Setup [Knative Eventing](https://github.com/knative/docs/tree/master/eventing)
   using the `release.yaml` file. This example does not require GCP.

## Create a GCP Service Account and a corresponding secret in Kubernetes

1. Enable Google Cloud Scheduler API
      ```shell
      gcloud services enable cloudscheduler.googleapis.com
      ```

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
   1. Create a namespace for where the secret is created and where our controller will run

      ```shell
      kubectl create namespace cloudschedulersource-system
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
kubectl apply -f https://raw.githubusercontent.com/vaikas-google/csr/master/release.yaml
```

## Inspect the Cloud Scheduler Source

First list the available sources, you might have others available to you, but this is the one we'll be using
in this example

```shell
 kubectl get crds -l "eventing.knative.dev/source=true"
```

You should see something like this:
```shell
vaikas@penguin:~/projects/go/src/github.com/vaikas-google/csr$
NAME                                      AGE
cloudschedulersources.sources.aikas.org   29d
```

you can then get more details about it, for example what are the available configuration options for it:
```shell
kubectl get crds cloudschedulersources.sources.aikas.org -oyaml
```

And in particular the Spec section is of interest:
```shell
 validation:
    openAPIV3Schema:
      properties:
        apiVersion:
          type: string
        kind:
          type: string
        metadata:
          type: object
        spec:
          properties:
            body:
              description: Optional body to send in the event
              type: string
            googleCloudProject:
              description: Google Cloud Project ID to create the scheduler job in.
              type: string
            httpMethod:
              description: Optional HTTP method to use when delivering the event.
                If omitted, uses POST
              type: string
            location:
              description: 'Google Cloud Platform region to create the scheduler job
                in. For example: us-central1.'
              type: string
            schedule:
              description: 'Schedule in cron format. For example: ''* * * * *'' (once
                a minute), or human readable: ''every 1 mins'''
              type: string
            serviceAccountName:
              description: Service Account to run Receive Adapter as. If omitted,
                uses 'default'.
              type: string
            sink:
              type: object
            timezone:
              description: Optional timezone of the schedule. If omitted, uses UTC.
              type: string
          required:
          - googleCloudProject
          - location
          - schedule
```


## Create a Knative Service that will be invoked for each Scheduler job invocation

To verify the `Cloud Scheduler` is working, we will create a simple Knative
`Service` that dumps incoming messages to its log. The `service.yaml` file
defines this basic service.

```yaml
apiVersion: serving.knative.dev/v1alpha1
kind: Service
metadata:
  name: github-message-dumper
spec:
  runLatest:
    configuration:
      revisionTemplate:
        spec:
          container:
            image: gcr.io/knative-releases/github.com/knative/eventing-sources/cmd/message_dumper
```

Enter the following command to create the service from `service.yaml`:

```shell
kubectl --namespace default apply -f https://raw.githubusercontent.com/vaikas-google/csr/master/service.yaml
```


## Configure a Cloud Scheduler Source to send events directly to the service

The simplest way to consume events is to wire the Source directly into the consuming
function. The logical picture looks like this:

![Source Directly To Function](csr-1-1.png)

## Wire Cloud Scheduler Events to the function 
Create a Cloud Scheduler instance targeting your function with the following:
```shell
curl https://raw.githubusercontent.com/vaikas-google/csr/master/one-to-one-csr.yaml | \
sed "s/MY_GCP_PROJECT/$PROJECT_ID/g" | kubectl apply -f -
```

## Check that the Cloud Scheduler Source was created
```shell
kubectl get cloudschedulersources
```

And you should see something like this:
```shell
vaikas@penguin:~/projects/go/src/github.com/vaikas-google/csr$ kubectl get cloudschedulersources
NAME             AGE
scheduler-test   1m
```

## Check that the Cloud Scheduler Job was created
```shell
gcloud beta scheduler jobs list
```

You should see something like this:
```shell
vaikas@penguin:~/projects/go/src/github.com/vaikas-google/csr$ gcloud beta scheduler jobs list
ID             LOCATION     SCHEDULE (TZ)       TARGET_TYPE  STATE
filter-source  us-central1  every 1 mins (UTC)  HTTP         ENABLED
```

Then wait a couple of minutes and you should see events in your message dumper.

## Check that Cloud Scheduler invoked the function
Note this might take couple of minutes after the creation while the Cloud Scheduler
gets going
```shell
kubectl -l 'serving.knative.dev/service=message-dumper' logs -c user-container
```
And you should see an entry like this there
```shell
2018/12/20 00:23:00 Received Cloud Event Context as: {CloudEventsVersion:0.1 EventID:2cd5d2ed-d2d1-94a1-bee7-d542d7ab834e EventTime:2018-12-20 00:23:00.498638175 +0000 UTC EventType:GoogleCloudScheduler EventTypeVersion: SchemaURL: ContentType:application/json Source:GCPCloudScheduler Extensions:map[]}
2018/12/20 00:23:00 Received event data as: {"data": "test does this work"}
```

Where the first line is displaying the Cloud Events Context and the second line is the actual data line.

## Uninstall

```shell
kubectl delete cloudschedulersources scheduler-test
kubectl delete services.serving message-dumper
```

## Check that the Cloud Scheduler Job was deleted
```shell
gcloud beta scheduler jobs list
```

## More complex examples
* [Multiple functions working together](MULTIPLE_FUNCTIONS.md)


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

You can upgrade `foo.yaml` jobs by updating the spec. For example, say you
wanted to change the above job to send a different body, you'd update
the foo.yaml from above like so:

```yaml
apiVersion: sources.aikas.org/v1alpha1
kind: CloudSchedulerSource
metadata:
  name: scheduler-test
spec:
  googleCloudProject: quantum-reducer-434
  location: us-central1
  schedule: "every 1 mins"
  body: "{test does this work, hopefully this does too}"
  sink:
    apiVersion: eventing.knative.dev/v1alpha1
    kind: Channel
    name: scheduler-demo
```

And then update the spec.
```shell
kubectl replace -f foo.yaml
```

Of course you can also do this in place by using:
```shell
kubectl edit cloudschedulersources scheduler-test
```

And on the next run (or so) the body send to your function will
by changed to '{test does this work, hopefully this does too}'
instead of '{test does this work}' like before.

### Removing

You can remove a Cloud Scheduler jobs via:
```shell
kubectl delete cloudschedulersources scheduler-test
```

