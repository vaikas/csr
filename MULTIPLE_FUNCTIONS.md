# Multtiple Functions working together

## Overview

This example builds on the simple Source to Function model, with very simple functions
demonstrating how to wire multiple functions together so that simple functions can
be wired together for more complex tasks.

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

1. [Prerequisites from simple case](./README.md)
1. [Create a GCP Service Account and install Cloud Scheduler Source](/README.md)

## Creating a Filter function

For the first example, we're going to create a filtering function. Filtering function
is extremely simple and is only meant to demonstrate ability to filter things for
functions downstream.

![Source Directly To Function](csr-filter.png)


## Create a Knative Service (Filter function) that will be invoked for each Scheduler job invocation

To verify the `Cloud Scheduler` is working, we will create a simple Knative
`Service` that dumps incoming messages to its log. The `service.yaml` file
defines this basic service.

```yaml
apiVersion: serving.knative.dev/v1alpha1
kind: Service
metadata:
  name: filter
  namespace: default
spec:
  runLatest:
    configuration:
      revisionTemplate:
        spec:
          container:
            image: us.gcr.io/probable-summer-223122/filter-55ddf8a10bddee5b31712a5ad318a3a3@sha256:49f619aa72aa0ff10b9797cd8fb6365aa795c244eb0f32fdf9533d93b8e018ff
```

Enter the following command to create the service from `filter.yaml`:

```shell
kubectl --namespace default apply -f https://raw.githubusercontent.com/vaikas-google/csr/master/filter.yaml
```

## Create a Knative Service that will be invoked for each job invocation that passes the filter

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


## Create a Channel resource for receiving events for filtering.
In order to be able to provide a Fanout for Events, or to be able to subscribe to output
of Functions (or return values), you need to wire them into channels. First we create
a channel that looks like this:
```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: Channel
metadata:
  name: scheduler-demo
spec:
  provisioner:
    apiVersion: eventing.knative.dev/v1alpha1
    kind: ClusterChannelProvisioner
    name: in-memory-channel
```

```shell
kubectl --namespace default apply -f https://raw.githubusercontent.com/vaikas-google/csr/master/channel.yaml
```

## Create a Channel resource receiving filtered events
We create another channel that receives filtered events from the Filter function.
```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: Channel
metadata:
  name: filtered
spec:
  provisioner:
    apiVersion: eventing.knative.dev/v1alpha1
    kind: ClusterChannelProvisioner
    name: in-memory-channel
```

```shell
kubectl --namespace default apply -f https://raw.githubusercontent.com/vaikas-google/csr/master/filtered-channel.yaml
```

## Configure a Cloud Scheduler Source to send events to the incoming channel we just created

```yaml
apiVersion: sources.aikas.org/v1alpha1
kind: CloudSchedulerSource
metadata:
  name: filter-source
spec:
  googleCloudProject: MY_GCP_PROJECT
  location: us-central1
  schedule: "every 1 mins"
  body: "{test does this work}"
  sink:
    apiVersion: eventing.knative.dev/v1alpha1
    kind: Channel
    name: scheduler-demo
```

```shell
curl https://raw.githubusercontent.com/vaikas-google/csr/master/filter-source-csr.yaml | \
sed "s/MY_GCP_PROJECT/$PROJECT_ID/g" | kubectl apply -f -
```

## Create the subscriptions

We create two subscriptions, one that wires the incoming channel `scheduler-demo` into our `Filter` function
with any results going into `filtered` channel, and a second subscription that wires a `message-dumper` to
receive Filtered events.
```yaml
# Subscription from the Cloud Scheduler Sources's output Channel to the Filter function.

apiVersion: eventing.knative.dev/v1alpha1
kind: Subscription
metadata:
  name: cloud-scheduler-source-sample
  namespace: default
spec:
  channel:
    apiVersion: eventing.knative.dev/v1alpha1
    kind: Channel
    name: scheduler-demo
  subscriber:
    ref:
      apiVersion: serving.knative.dev/v1alpha1
      kind: Service
      name: filter
  reply:
    channel:
      apiVersion: eventing.knative.dev/v1alpha1
      kind: Channel
      name: filtered
---

# Subscription from filtered channel to the message dumper function
apiVersion: eventing.knative.dev/v1alpha1
kind: Subscription
metadata:
  name: cloud-scheduler-filtered
  namespace: default
spec:
  channel:
    apiVersion: eventing.knative.dev/v1alpha1
    kind: Channel
    name: filtered
  subscriber:
    ref:
      apiVersion: serving.knative.dev/v1alpha1
      kind: Service
      name: message-dumper
```

```shell
kubectl --namespace default apply -f https://raw.githubusercontent.com/vaikas-google/csr/master/filter-subscription.yaml
```


## Wire Cloud Scheduler Events to the function 
Create a Cloud Scheduler instance targeting your function with the following:
```shell
curl https://raw.githubusercontent.com/vaikas-google/csr/master/one-to-one-csr.yaml | \
sed "s/MY_GCP_PROJECT/$PROJECT_ID/g" | kubectl apply -f -
```

## Check that the Cloud Scheduler Job was created
```shell
gcloud beta scheduler jobs list
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
vaikas@penguin:~/projects/go/src/github.com/vaikas-google/csr$ kubectl -l 'serving.knative.dev/service=message-dumper' logs -c user-container
2018/12/07 22:05:00 Message Dumper received a message: POST / HTTP/1.1
Host: message-dumper.default.svc.cluster.local
Transfer-Encoding: chunked
Accept-Encoding: gzip
Ce-Cloudeventsversion: 0.1
Ce-Eventid: 5ec4c08e-bab7-9748-af40-ac9a49dcc2d4
Ce-Eventtime: 2018-12-07T22:05:00.492448449Z
Ce-Eventtype: GoogleCloudScheduler
Ce-Source: GCPCloudScheduler
Content-Type: application/json
User-Agent: Go-http-client/1.1
X-B3-Sampled: 1
X-B3-Spanid: bff04c5522a8dab8
X-B3-Traceid: bff04c5522a8dab8
X-Forwarded-For: 127.0.0.1
X-Forwarded-Proto: http
X-Request-Id: 0412e5bc-9d53-9b5e-be66-8450e32221a4

276
"UE9TVCAvIEhUVFAvMS4xDQpIb3N0OiBzY2hlZHVsZXItdGVzdC5kZWZhdWx0LmFpa2FzLm9yZw0KQWNjZXB0LUVuY29kaW5nOiBnemlwLGRlZmxhdGUsYnINCkNvbnRlbnQtTGVuZ3RoOiAyMQ0KQ29udGVudC1UeXBlOiBhcHBsaWNhdGlvbi9vY3RldC1zdHJlYW0NClVzZXItQWdlbnQ6IEdvb2dsZS1DbG91ZC1TY2hlZHVsZXINClgtQjMtU2FtcGxlZDogMQ0KWC1CMy1TcGFuaWQ6IDU5MjNiZGNiOGFjNGI2ZmMNClgtQjMtVHJhY2VpZDogNTkyM2JkY2I4YWM0YjZmYw0KWC1FbnZveS1FeHBlY3RlZC1ScS1UaW1lb3V0LU1zOiA2MDAwMA0KWC1FbnZveS1JbnRlcm5hbDogdHJ1ZQ0KWC1Gb3J3YXJkZWQtRm9yOiAxMC4yNDAuMC4xNiwgMTI3LjAuMC4xDQpYLUZvcndhcmRlZC1Qcm90bzogaHR0cA0KWC1SZXF1ZXN0LUlkOiA1ZWM0YzA4ZS1iYWI3LTk3NDgtYWY0MC1hYzlhNDlkY2MyZDQNCg0Ke3Rlc3QgZG9lcyB0aGlzIHdvcmt9"
0
```

Where the last line is the base64 decoded message, you can cut&paste that line and feed it through base64 tool (the ^d below means
hit CTRL-d):
```shell
base64 -d
UE9TVCAvIEhUVFAvMS4xDQpIb3N0OiBzY2hlZHVsZXItdGVzdC5kZWZhdWx0LmFpa2FzLm9yZw0KQWNjZXB0LUVuY29kaW5nOiBnemlwLGRlZmxhdGUsYnINCkNvbnRlbnQtTGVuZ3RoOiAyMQ0KQ29udGVudC1UeXBlOiBhcHBsaWNhdGlvbi9vY3RldC1zdHJlYW0NClVzZXItQWdlbnQ6IEdvb2dsZS1DbG91ZC1TY2hlZHVsZXINClgtQjMtU2FtcGxlZDogMQ0KWC1CMy1TcGFuaWQ6IDU5MjNiZGNiOGFjNGI2ZmMNClgtQjMtVHJhY2VpZDogNTkyM2JkY2I4YWM0YjZmYw0KWC1FbnZveS1FeHBlY3RlZC1ScS1UaW1lb3V0LU1zOiA2MDAwMA0KWC1FbnZveS1JbnRlcm5hbDogdHJ1ZQ0KWC1Gb3J3YXJkZWQtRm9yOiAxMC4yNDAuMC4xNiwgMTI3LjAuMC4xDQpYLUZvcndhcmRlZC1Qcm90bzogaHR0cA0KWC1SZXF1ZXN0LUlkOiA1ZWM0YzA4ZS1iYWI3LTk3NDgtYWY0MC1hYzlhNDlkY2MyZDQNCg0Ke3Rlc3QgZG9lcyB0aGlzIHdvcmt9
^d
POST / HTTP/1.1
Host: scheduler-test.default.aikas.org
Accept-Encoding: gzip,deflate,br
Content-Length: 21
Content-Type: application/octet-stream
User-Agent: Google-Cloud-Scheduler
X-B3-Sampled: 1
X-B3-Spanid: 5923bdcb8ac4b6fc
X-B3-Traceid: 5923bdcb8ac4b6fc
X-Envoy-Expected-Rq-Timeout-Ms: 60000
X-Envoy-Internal: true
X-Forwarded-For: 10.240.0.16, 127.0.0.1
X-Forwarded-Proto: http
X-Request-Id: 5ec4c08e-bab7-9748-af40-ac9a49dcc2d4

{test does this work}
```

## Uninstall

```shell
kubectl delete cloudschedulersources scheduler-test
```

## More complex examples
* [Multiple functions working together]{MULTIPLE_FUNCTIONS.md}

## Check that the Cloud Scheduler Job was deleted
```shell
gcloud beta scheduler jobs list
```

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

But if it did, you'd do:
```shell
kubectl replace -f foo.yaml
```

And on the next run (or so) the body send to your function will
by changed to '{test does this work, hopefully this does too}'
instead of '{test does this work}' like before.

### Removing

You can remove a Cloud Scheduler jobs via:
```shell
kubectl delete cloudschedulersources scheduler-test
```


