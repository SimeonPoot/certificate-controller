package controllers

import (
	"context"
	"errors"
	"strings"
	"time"

	errs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	cmInformerV1 "github.com/cert-manager/cert-manager/pkg/client/informers/externalversions/certmanager/v1"
	cmListerV1 "github.com/cert-manager/cert-manager/pkg/client/listers/certmanager/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type CertControllerOptions struct {
	Trigger string
	DryRun  bool
}

type Controller struct {
	clientset       versioned.Clientset
	certLister      cmListerV1.CertificateLister
	certCacheSynced cache.InformerSynced
	queue           workqueue.RateLimitingInterface
	CertControllerOptions
	Lg *logrus.Logger
}

func NewController(clientset versioned.Clientset, cmInformer cmInformerV1.CertificateInformer, cOpts CertControllerOptions, log *logrus.Logger) *Controller {
	c := &Controller{
		clientset:             clientset,
		certLister:            cmInformer.Lister(),
		certCacheSynced:       cmInformer.Informer().HasSynced,
		queue:                 workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "certificate-controller"),
		CertControllerOptions: cOpts,
		Lg:                    log,
	}

	cmInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: c.handleCertAdd,
			// DeleteFunc: c.handleCertDel,
		})

	return c
}

func (c *Controller) Run(ch <-chan struct{}) {
	if !cache.WaitForCacheSync(ch, c.certCacheSynced) {
		c.Lg.Info("waiting for cache to be synced")
	}

	wait.Until(c.worker, 1*time.Second, ch)

	<-ch
}

func (c *Controller) worker() {
	for c.processNextWorkItem() {

	}
}

func (c *Controller) processNextWorkItem() bool {
	// Wait until there is a new item in the working queue
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two pods with the same key are never processed in
	// parallel.
	defer c.queue.Done(key)

	// Invoke the method containing the business logic
	err := c.syncHandler(key)
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, key)
	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two.
func (c *Controller) syncHandler(obj interface{}) error {
	ctx := context.Background()
	// allnamespaces: v1.NamespaceAll
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.Lg.Errorf("error getting key from cache %s\n", err)
		return err
	}
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.Lg.Errorf("error splitting key into namespace + name %s\n", err)
		return err
	}

	// Calling the certificate object from informers cache
	crt, err := c.certLister.Certificates(ns).Get(name)
	if err != nil {
		// The certificate resource may no longer exist, in which case we stop
		// processing.
		if errs.IsNotFound(err) {
			c.Lg.Infof("did not find cert %s / %s", ns, name)
			return nil
		}
		return err
	}

	if crt.Status.Conditions == nil {
		// FIXME: BackOff retry? >> an option how much backoff ?
		c.Lg.Info("probably just created: ", crt.GetName())
		return errors.New("status is nil, retrying object")
	}

	for _, v := range crt.Status.Conditions {
		if v.Status != "True" {
			c.Lg.Infof("status is not True for cert %s, msg is: %s", crt.GetName(), v.Message)

			// take some time checking if a cert needs to be deleted,
			// sometimes a cert needs a few seconds to come

			if !c.DryRun && strings.Contains(v.Message, c.Trigger) {
				c.clientset.CertmanagerV1().Certificates(ns).Delete(ctx, name, metav1.DeleteOptions{})
				if err != nil {
					c.Lg.Errorf("something went wrong deleting certificate: %s/%s", ns, name)
					return err
				}
			}
			c.Lg.Infof("deleted certificate: %s/%s", ns, name)
		}
	}

	c.Lg.Info(crt.Status)

	return nil
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.queue.Forget(key)
		return
	}

	// This controller retries 5 times if something goes wrong. After that, it stops trying.
	if c.queue.NumRequeues(key) < 5 {
		c.Lg.Infof("Error syncing pod %v: %v", key, err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	runtime.HandleError(err)
	c.Lg.Infof("Dropping pod %q out of the queue: %v", key, err)
}

func (c *Controller) handleCertAdd(obj interface{}) {
	c.Lg.Info("handleCertAdd called")
	c.queue.Add(obj)
}
