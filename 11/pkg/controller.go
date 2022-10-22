package pkg

import (
	"context"
	"log"
	"reflect"
	"time"

	v13 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreInformer "k8s.io/client-go/informers/core/v1"
	netInformer "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/kubernetes"
	coreLister "k8s.io/client-go/listers/core/v1"
	netLister "k8s.io/client-go/listers/networking/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	workerNum = 5
	maxRetry  = 10
)

type controller struct {
	client        kubernetes.Interface
	ingressLister netLister.IngressLister
	serviceLister coreLister.ServiceLister
	queue         workqueue.RateLimitingInterface
}

func NewController(client kubernetes.Interface, serviceInformer coreInformer.ServiceInformer, ingressInformer netInformer.IngressInformer) controller {
	c := controller{
		client:        client,
		ingressLister: ingressInformer.Lister(),
		serviceLister: serviceInformer.Lister(),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ingressManager"),
	}
	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addService,
		UpdateFunc: c.updateService,
	})
	ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: c.deleteIngress,
	})
	return c
}

func (c *controller) addService(obj interface{}) {
	c.enqueue(obj)
}

func (c *controller) updateService(oldObj interface{}, newObj interface{}) {
	// TODO: compare annotation
	if reflect.DeepEqual(oldObj, newObj) {
		return
	}

	c.enqueue(newObj)
}

func (c *controller) deleteIngress(obj interface{}) {
	// If deleted ingress's corresponding service exists with correct annotation,
	// then recontruct ingress.
	ingress := obj.(*v1.Ingress)
	ownerReference := v12.GetControllerOf(ingress)
	if ownerReference == nil {
		return
	}
	if ownerReference.Kind != "Service" {
		// GetControllerOf might not reture service
		return
	}
	c.queue.Add(ingress.Namespace + "/" + ingress.Name) // same as service key
}

func (c *controller) Run(stopCh chan struct{}) {
	for i := 0; i < workerNum; i++ {
		go wait.Until(c.worker, time.Minute, stopCh)
	}
	<-stopCh
}

func (c *controller) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
	}
	c.queue.Add(key)
}

func (c *controller) worker() {
	for c.processNextItem() {

	}
}

func (c *controller) processNextItem() bool {
	item, shutdow := c.queue.Get()
	if shutdow {
		return false
	}
	defer c.queue.Done(item)

	key := item.(string)
	log.Println(key)
	err := c.syncService(key)
	if err != nil {
		c.handleError(key, err)
	}
	return true
}

func (c *controller) syncService(item string) error {
	namespaceKey, name, err := cache.SplitMetaNamespaceKey(item)
	if err != nil {
		return err
	}

	service, err := c.serviceLister.Services(namespaceKey).Get(name)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	// add or delete
	_, ok := service.GetAnnotations()["ingress/http"]
	ingress, err := c.ingressLister.Ingresses(namespaceKey).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if ok && errors.IsNotFound(err) {
		// create ingress
		// (ingress don't exist but annotation exist)
		ig := c.constructIngress(service)
		_, err := c.client.NetworkingV1().Ingresses(namespaceKey).Create(context.TODO(), ig, v12.CreateOptions{})
		if err != nil {
			return err
		}
	} else if !ok && ingress != nil {
		// delete ingress
		// (annotation does not exist, delete existing ingress)
		err := c.client.NetworkingV1().Ingresses(namespaceKey).Delete(context.TODO(), name, v12.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) constructIngress(service *v13.Service) *v1.Ingress {
	ingress := v1.Ingress{}

	ingress.ObjectMeta.OwnerReferences = []v12.OwnerReference{
		*v12.NewControllerRef(service, v13.SchemeGroupVersion.WithKind("Service")),
	}

	ingress.Name = service.Name
	ingress.Namespace = service.Namespace
	pathType := v1.PathTypePrefix
	icn := "nginx"
	ingress.Spec = v1.IngressSpec{
		IngressClassName: &icn,
		Rules: []v1.IngressRule{
			{
				Host: "example.com",
				IngressRuleValue: v1.IngressRuleValue{
					HTTP: &v1.HTTPIngressRuleValue{
						Paths: []v1.HTTPIngressPath{
							{
								Path:     "/",
								PathType: &pathType,
								Backend: v1.IngressBackend{
									Service: &v1.IngressServiceBackend{
										Name: service.Name,
										Port: v1.ServiceBackendPort{
											Number: 80,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return &ingress
}

func (c *controller) handleError(key string, err error) {
	if c.queue.NumRequeues(key) <= maxRetry {
		c.queue.AddRateLimited(key)
		return
	}
	runtime.HandleError(err)
	c.queue.Forget(key)
}
