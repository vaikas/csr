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
	"context"
	"fmt"

	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/logging/logkey"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"cloud.google.com/go/scheduler/apiv1beta1"
	//	servingv1alpha1
	//	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"github.com/vaikas-google/csr/pkg/apis/cloudschedulersource/v1alpha1"
	clientset "github.com/vaikas-google/csr/pkg/client/clientset/versioned"
	cloudschedulersourcescheme "github.com/vaikas-google/csr/pkg/client/clientset/versioned/scheme"
	informers "github.com/vaikas-google/csr/pkg/client/informers/externalversions/cloudschedulersource/v1alpha1"
	listers "github.com/vaikas-google/csr/pkg/client/listers/cloudschedulersource/v1alpha1"
	"github.com/vaikas-google/csr/pkg/reconciler/cloudschedulersource/resources"
	schedulerpb "google.golang.org/genproto/googleapis/cloud/scheduler/v1beta1"
	"k8s.io/client-go/dynamic"
)

const controllerAgentName = "cloudschedulersource-controller"

// Reconciler is the controller implementation for Cloudschedulersource resources
type Reconciler struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// cloudschedulersourceclientset is a clientset for our own API group
	cloudschedulersourceclientset clientset.Interface

	cloudschedulersourcesLister listers.CloudSchedulerSourceLister

	dynamicClient dynamic.Interface
	client        client.Client

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
	//	servingclientset clientset.Interface,
	//	servingsourceInformer informers.CloudSchedulerSourceInformer,
	raImage string,
) *controller.Impl {

	// Enrich the logs with controller name
	logger = logger.Named(controllerAgentName).With(zap.String(logkey.ControllerType, controllerAgentName))

	r := &Reconciler{
		kubeclientset:                 kubeclientset,
		dynamicClient:                 dynamicClient,
		cloudschedulersourceclientset: cloudschedulersourceclientset,
		cloudschedulersourcesLister:   cloudschedulersourceInformer.Lister(),
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
	csr, err := c.cloudschedulersourcesLister.CloudSchedulerSources(namespace).Get(name)
	if errors.IsNotFound(err) {
		// The CloudSchedulerSource resource may no longer exist, in which case we stop processing.
		runtime.HandleError(fmt.Errorf("cloudschedulersource '%s' in work queue no longer exists", key))
		return nil
	} else if err != nil {
		return err
	}

	if err := c.reconcileCloudSchedulerSource(ctx, csr); err != nil {
		return err
	}

	return nil
}

//, "http://message-dumper.default.aikas.org/"

func (c *Reconciler) reconcileCloudSchedulerSource(ctx context.Context, csr *v1alpha1.CloudSchedulerSource) error {
	// First try to resolve the sink, and if not found mark as not resolved.
	uri, err := GetSinkURI(c.dynamicClient, csr.Spec.Sink, csr.Namespace)
	if err != nil {
		//		csr.Status.MarkNoSink("NotFound", "%s", err)
		c.Logger.Infof("Couldn't resolve Sink URI: %s", err)
		return err
	}

	c.Logger.Infof("Resolved Sink URI to %q", uri)
	csr.Status.SinkURI = uri

	ksvc := resources.MakeService(csr, c.raImage)

	c.Logger.Infof("Would create service %+v", ksvc)

	c.Logger.Infof("creating scheduler job")
	err = c.createJob(csr.Name, &csr.Spec, "http://message-dumper.default.aikas.org/")
	if err != nil {
		c.Logger.Infof("Failed to create scheduler job: %s", err)
		return err
	}
	return nil
}

func (c *Reconciler) createJob(name string, spec *v1alpha1.CloudSchedulerSourceSpec, target string) error {
	ctx := context.Background()
	csc, err := scheduler.NewCloudSchedulerClient(ctx)
	if err != nil {
		return err
	}

	timezone := "UTC"
	if spec.TimeZone != "" {
		timezone = spec.TimeZone
	}

	parent := fmt.Sprintf("projects/%s/locations/%s", spec.GoogleCloudProject, spec.Location)
	jobName := fmt.Sprintf("%s/jobs/%s", parent, name)
	req := &schedulerpb.CreateJobRequest{
		Parent: parent,
		Job: &schedulerpb.Job{
			Name:     jobName,
			Schedule: spec.Schedule,
			TimeZone: timezone,
			Target: &schedulerpb.Job_HttpTarget{
				HttpTarget: &schedulerpb.HttpTarget{
					Uri:        target,
					HttpMethod: schedulerpb.HttpMethod_POST,
				},
			},
		},
	}
	c.Logger.Infof("Creating job as: %+v", req)
	resp, err := csc.CreateJob(ctx, req)
	if err != nil {
		return err
	}
	// TODO: Use resp.
	c.Logger.Infof("Created job %+v", resp)
	return nil
}
