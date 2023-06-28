package main

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type DynamicController struct {
	clientset       dynamic.DynamicClient
	certLister      cache.GenericLister
	certCacheSynced cache.InformerSynced
	queue           workqueue.RateLimitingInterface
	certResource    schema.GroupVersionResource
}

func newDynamicController(clientset dynamic.DynamicClient, certInformer informers.GenericInformer, certResource schema.GroupVersionResource) *DynamicController {
	c := &DynamicController{
		clientset:       clientset,
		certLister:      certInformer.Lister(),
		certCacheSynced: certInformer.Informer().HasSynced,
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "secret-syncer"),
		certResource:    certResource,
	}

	certInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: c.handleCertAdd,
			// DeleteFunc: c.handleCertDel,
		})

	return c
}

func (c *DynamicController) run(ch <-chan struct{}) {
	if !cache.WaitForCacheSync(ch, c.certCacheSynced) {
		fmt.Println("waiting for cache to be synced")
	}

	wait.Until(c.worker, 1*time.Second, ch)

	<-ch
}

func (c *DynamicController) worker() {
	for c.processNextWorkItem() {

	}
}

func (c *DynamicController) processNextWorkItem() bool {
	item, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Forget(item)
	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		fmt.Printf("error getting key from cache %s\n", err)
		return false
	}
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		fmt.Printf("error splitting key into namespace + name %s\n", err)
		return false
	}

	err = c.syncSecret(ns, name)
	if err != nil {
		//  retry
		fmt.Printf("error syncing cert: %s", err.Error())
		return false
	}

	return true
}

func (c *DynamicController) syncSecret(ns, name string) error {
	// ctx := context.Background()

	fmt.Println("certificate ns: ", ns)
	fmt.Println("certificate name: ", name)

	// obj, err := c.clientset.Resource(c.certResource).List(context.Background(), v1.ListOptions{})

	obj, err := c.clientset.Resource(c.certResource).Namespace(ns).Get(context.TODO(), name, v1.GetOptions{}, "")
	if err != nil {
		fmt.Println("abcdefg!")
		return err
	}

	fmt.Println("object get name", obj)

	// get the secret from the informer, not calling the apiserver again
	cert, err := c.certLister.ByNamespace("sandbox").List(labels.Everything())
	if err != nil {
		fmt.Println("certlisteur")
		return err
	}

	fmt.Println("helleu: ", cert)

	return nil
}

func (c *DynamicController) handleCertAdd(obj interface{}) {
	fmt.Println("handdleAdd called")
	c.queue.Add(obj)
}
