/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloudschedulersource

import (
	"bytes"
	"context"
	"fmt"
	"reflect"

	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/logging/logkey"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"

	"cloud.google.com/go/scheduler/apiv1beta1"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	servingclientset "github.com/knative/serving/pkg/client/clientset/versioned"
	servinginformers "github.com/knative/serving/pkg/client/informers/externalversions/serving/v1alpha1"
	"github.com/vaikas-google/csr/pkg/apis/cloudschedulersource/v1alpha1"
	clientset "github.com/vaikas-google/csr/pkg/client/clientset/versioned"
	cloudschedulersourcescheme "github.com/vaikas-google/csr/pkg/client/clientset/versioned/scheme"
	informers "github.com/vaikas-google/csr/pkg/client/informers/externalversions/cloudschedulersource/v1alpha1"
	listers "github.com/vaikas-google/csr/pkg/client/listers/cloudschedulersource/v1alpha1"
	"github.com/vaikas-google/csr/pkg/reconciler/cloudschedulersource/resources"
	schedulerpb "google.golang.org/genproto/googleapis/cloud/scheduler/v1beta1"
	"google.golang.org/grpc/codes"
	gstatus "google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
)

const (
	controllerAgentName = "cloudschedulersource-controller"
	finalizerName       = controllerAgentName
)

// Reconciler is the controller implementation for Cloudschedulersource resources
type Reconciler struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// cloudschedulersourceclientset is a clientset for our own API group
	cloudschedulersourceclientset clientset.Interface
	cloudschedulersourcesLister   listers.CloudSchedulerSourceLister

	// We use dynamic client for Duck type related stuff.
	dynamicClient dynamic.Interface

	// For dealing with Service.serving.knative.dev
	servingClient   servingclientset.Interface
	servingInformer servinginformers.ServiceInformer

	// Receive Adapter Image.
	raImage string

	// Sugared logger is easier to use but is not as performant as the
	// raw logger. In performance critical paths, call logger.Desugar()
	// and use the returned raw logger instead. In addition to the
	// performance benefits, raw logger also preserves type-safety at
	// the expense of slightly greater verbosity.
	Logger *zap.SugaredLogger
}

// Check that we implement the controller.Reconciler interface.
var _ controller.Reconciler = (*Reconciler)(nil)

func init() {
	// Add cloudschedulersource-controller types to the default Kubernetes Scheme so Events can be
	// logged for cloudschedulersource-controller types.
	cloudschedulersourcescheme.AddToScheme(scheme.Scheme)
}

// NewController returns a new cloudschedulersource controller
func NewController(
	logger *zap.SugaredLogger,
	kubeclientset kubernetes.Interface,
	dynamicClient dynamic.Interface,
	cloudschedulersourceclientset clientset.Interface,
	cloudschedulersourceInformer informers.CloudSchedulerSourceInformer,
	servingclientset servingclientset.Interface,
	servingsourceInformer servinginformers.ServiceInformer,
	raImage string,
) *controller.Impl {

	// Enrich the logs with controller name
	logger = logger.Named(controllerAgentName).With(zap.String(logkey.ControllerType, controllerAgentName))

	r := &Reconciler{
		kubeclientset:                 kubeclientset,
		dynamicClient:                 dynamicClient,
		cloudschedulersourceclientset: cloudschedulersourceclientset,
		cloudschedulersourcesLister:   cloudschedulersourceInformer.Lister(),
		servingClient:                 servingclientset,
		raImage:                       raImage,
		Logger:                        logger,
	}
	statsExporter, err := controller.NewStatsReporter(controllerAgentName)
	if nil != err {
		logger.Fatalf("Couldn't create stats exporter: %s", err)
	}
	impl := controller.NewImpl(r, logger, "CloudSchedulerSources", statsExporter)

	logger.Info("Setting up event handlers")

	// Set up an event handler for when CloudSchedulerSource resources change
	cloudschedulersourceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    impl.Enqueue,
		UpdateFunc: controller.PassNew(impl.Enqueue),
	})

	// Set up an event handler for when CloudSchedulerSource owned Service resources change.
	// Basically whenever a Service controlled by us is chaned, we want to know about it.
	servingsourceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    impl.EnqueueControllerOf,
		UpdateFunc: controller.PassNew(impl.EnqueueControllerOf),
		DeleteFunc: impl.EnqueueControllerOf,
	})

	return impl
}

// Reconcile implements controller.Reconciler
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the CloudSchedulerSource resource with this namespace/name
	original, err := c.cloudschedulersourcesLister.CloudSchedulerSources(namespace).Get(name)
	if errors.IsNotFound(err) {
		// The CloudSchedulerSource resource may no longer exist, in which case we stop processing.
		runtime.HandleError(fmt.Errorf("cloudschedulersource '%s' in work queue no longer exists", key))
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	csr := original.DeepCopy()

	err = c.reconcileCloudSchedulerSource(ctx, csr)

	if equality.Semantic.DeepEqual(original.Status, csr.Status) &&
		equality.Semantic.DeepEqual(original.ObjectMeta, csr.ObjectMeta) {
		// If we didn't change anything (status or finalizers) then don't
		// call update.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err := c.update(csr); err != nil {
		c.Logger.Warn("Failed to update CloudSchedulerService status", zap.Error(err))
		return err
	}
	return err
}

func (c *Reconciler) reconcileCloudSchedulerSource(ctx context.Context, csr *v1alpha1.CloudSchedulerSource) error {
	// See if the source has been deleted.
	deletionTimestamp := csr.DeletionTimestamp

	// First try to resolve the sink, and if not found mark as not resolved.
	uri, err := GetSinkURI(c.dynamicClient, csr.Spec.Sink, csr.Namespace)
	if err != nil {
		// TODO: Update status appropriately
		//		csr.Status.MarkNoSink("NotFound", "%s", err)
		c.Logger.Infof("Couldn't resolve Sink URI: %s", err)
		if deletionTimestamp == nil {
			return err
		}
		// we don't care about the URI if we're deleting, so carry on...
		uri = ""
	}
	c.Logger.Infof("Resolved Sink URI to %q", uri)

	if deletionTimestamp != nil {
		err := c.deleteJob(csr)
		if err != nil {
			c.Logger.Infof("Unable to delete the Job: %s", err)
			return err
		}
		c.removeFinalizer(csr)
		return nil
	}

	c.addFinalizer(csr)

	csr.Status.SinkURI = uri

	// Make sure Service is in the state we expect it to be in.
	ksvc, err := c.reconcileService(csr)
	if err != nil {
		// TODO: Update status appropriately
		c.Logger.Infof("Failed to reconcile service: %s", err)
		return err
	}
	c.Logger.Infof("Reconciled service: %+v", ksvc)

	if ksvc.Status.Domain == "" {
		// TODO: Update status appropriately
		c.Logger.Infof("No domain configured for service, bailing...")
		return fmt.Errorf("no domain configured for service")
	}

	url := fmt.Sprintf("http://%s/", ksvc.Status.Domain)
	c.Logger.Infof("using %s as a cluster sink", url)

	job, err := c.reconcileJob(csr.Name, &csr.Spec, url)
	if err != nil {
		// TODO: Update status with this...
		c.Logger.Infof("Failed to reconcile Job: %s", err)
		return err
	}

	c.Logger.Infof("Reconciled job: %+v", job)
	csr.Status.Job = job.Name

	return nil
}

func (c *Reconciler) reconcileService(csr *v1alpha1.CloudSchedulerSource) (*servingv1alpha1.Service, error) {
	svcClient := c.servingClient.ServingV1alpha1().Services(csr.Namespace)
	existing, err := svcClient.Get(csr.Name, v1.GetOptions{})
	if err == nil {
		// TODO: Handle any updates...
		c.Logger.Infof("Found existing service: %+v", existing)
		return existing, nil
	}
	if errors.IsNotFound(err) {
		ksvc := resources.MakeService(csr, c.raImage)
		c.Logger.Infof("Creating service %+v", ksvc)
		return c.servingClient.ServingV1alpha1().Services(csr.Namespace).Create(ksvc)
	}
	return nil, err
}

func (c *Reconciler) reconcileJob(name string, spec *v1alpha1.CloudSchedulerSourceSpec, target string) (*schedulerpb.Job, error) {
	parent := fmt.Sprintf("projects/%s/locations/%s", spec.GoogleCloudProject, spec.Location)
	jobName := fmt.Sprintf("%s/jobs/%s", parent, name)

	c.Logger.Infof("Parent: %q Job: %q", parent, jobName)

	ctx := context.Background()
	csc, err := scheduler.NewCloudSchedulerClient(ctx)
	if err != nil {
		return nil, err
	}

	getReq := &schedulerpb.GetJobRequest{
		Name: jobName,
	}

	existing, err := csc.GetJob(ctx, getReq)
	if err == nil {
		c.Logger.Infof("Found existing job as: %+v", existing)

		existingHttpTarget := existing.GetHttpTarget()
		if existingHttpTarget == nil {
			return nil, fmt.Errorf("missing http target in the existing scheduler proto: %+v", existing)
		}

		updated := createJobProto(jobName, spec, target)
		updatedHttpTarget := updated.GetHttpTarget()
		if updatedHttpTarget == nil {
			return nil, fmt.Errorf("missing http target in the updated scheduler proto: %+v", updated)
		}
		if updated.Schedule != existing.Schedule ||
			updated.TimeZone != existing.TimeZone ||
			bytes.Compare(updatedHttpTarget.Body, existingHttpTarget.Body) != 0 ||
			updatedHttpTarget.HttpMethod != existingHttpTarget.HttpMethod {
			req := &schedulerpb.UpdateJobRequest{
				Job: updated,
			}
			c.Logger.Info("Updating Job spec with %+v", req)
			resp, err := csc.UpdateJob(ctx, req)
			if err != nil {
				return nil, err
			}
			return resp, nil
		}
		return existing, nil
	}

	if st, ok := gstatus.FromError(err); !ok {
		c.Logger.Infof("Unknown error from the cloud scheduler client: %s", err)
		return nil, err
	} else if st.Code() != codes.NotFound {
		return nil, err
	}

	req := &schedulerpb.CreateJobRequest{
		Parent: parent,
		Job:    createJobProto(jobName, spec, target),
	}

	c.Logger.Infof("Creating job as: %+v", req)
	resp, err := csc.CreateJob(ctx, req)
	if err != nil {
		return nil, err
	}
	// TODO: Use resp.
	c.Logger.Infof("Created job %+v", resp)
	return resp, nil
}

func createJobProto(jobName string, spec *v1alpha1.CloudSchedulerSourceSpec, target string) *schedulerpb.Job {
	// If no timezone specified, use UTC
	timezone := "UTC"
	if spec.TimeZone != "" {
		timezone = spec.TimeZone
	}

	// For method, default to POST, otherwise use what's specified and look up the value for it.
	HttpMethod := schedulerpb.HttpMethod_POST
	if spec.HTTPMethod != "" {
		if m, ok := schedulerpb.HttpMethod_value[spec.HTTPMethod]; ok {
			HttpMethod = schedulerpb.HttpMethod(m)
		}
	}

	httpTarget := &schedulerpb.Job_HttpTarget{
		HttpTarget: &schedulerpb.HttpTarget{
			Uri:        target,
			HttpMethod: HttpMethod,
		},
	}
	if spec.Body != "" {
		httpTarget.HttpTarget.Body = []byte(spec.Body)
	}

	job := &schedulerpb.Job{
		Name:     jobName,
		Schedule: spec.Schedule,
		TimeZone: timezone,
		Target:   httpTarget,
	}
	return job
}

func (c *Reconciler) deleteJob(csr *v1alpha1.CloudSchedulerSource) error {
	parent := fmt.Sprintf("projects/%s/locations/%s", csr.Spec.GoogleCloudProject, csr.Spec.Location)
	jobName := fmt.Sprintf("%s/jobs/%s", parent, csr.Name)

	c.Logger.Infof("Parent: %q Job: %q", parent, jobName)

	ctx := context.Background()
	csc, err := scheduler.NewCloudSchedulerClient(ctx)
	if err != nil {
		return err
	}

	deleteReq := &schedulerpb.DeleteJobRequest{
		Name: jobName,
	}

	c.Logger.Infof("Deleting job as: %q", jobName)
	err = csc.DeleteJob(ctx, deleteReq)
	if err == nil {
		c.Logger.Infof("Deleted job: %+v", jobName)
		return nil
	}

	if st, ok := gstatus.FromError(err); !ok {
		c.Logger.Infof("Unknown error from the cloud scheduler client: %s", err)
		return err
	} else if st.Code() != codes.NotFound {
		return err
	}
	return nil
}

func (c *Reconciler) addFinalizer(csr *v1alpha1.CloudSchedulerSource) {
	finalizers := sets.NewString(csr.Finalizers...)
	finalizers.Insert(finalizerName)
	csr.Finalizers = finalizers.List()
}

func (c *Reconciler) removeFinalizer(csr *v1alpha1.CloudSchedulerSource) {
	finalizers := sets.NewString(csr.Finalizers...)
	finalizers.Delete(finalizerName)
	csr.Finalizers = finalizers.List()
}

func (c *Reconciler) update(desired *v1alpha1.CloudSchedulerSource) (*v1alpha1.CloudSchedulerSource, error) {
	csr, err := c.cloudschedulersourcesLister.CloudSchedulerSources(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}
	// Check if there is anything to update.
	if !reflect.DeepEqual(csr.Status, desired.Status) || !reflect.DeepEqual(csr.ObjectMeta, desired.ObjectMeta) {
		// Don't modify the informers copy
		existing := csr.DeepCopy()
		existing.Status = desired.Status
		existing.Finalizers = desired.Finalizers
		client := c.cloudschedulersourceclientset.SourcesV1alpha1().CloudSchedulerSources(desired.Namespace)
		// TODO: for CRD there's no updatestatus, so use normal update.
		return client.Update(existing)
	}
	return csr, nil
}
