package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	cmInformerV1 "github.com/cert-manager/cert-manager/pkg/client/informers/externalversions/certmanager/v1"
	cmListerV1 "github.com/cert-manager/cert-manager/pkg/client/listers/certmanager/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type CertControllerOptions struct {
	Trigger string
	DryRun  bool
}

type CertController struct {
	clientset       versioned.Clientset
	certLister      cmListerV1.CertificateLister
	certCacheSynced cache.InformerSynced
	queue           workqueue.RateLimitingInterface
	CertControllerOptions
	Lg *logrus.Logger
}

func NewCertController(clientset versioned.Clientset, cmInformer cmInformerV1.CertificateInformer, cOpts CertControllerOptions, log *logrus.Logger) *CertController {
	c := &CertController{
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

func (c *CertController) Run(ch <-chan struct{}) {
	if !cache.WaitForCacheSync(ch, c.certCacheSynced) {
		c.Lg.Info("waiting for cache to be synced")
	}

	wait.Until(c.worker, 1*time.Second, ch)

	<-ch
}

func (c *CertController) worker() {
	for c.processNextWorkItem() {

	}
}

func (c *CertController) processNextWorkItem() bool {
	item, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Forget(item)
	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		c.Lg.Errorf("error getting key from cache %s\n", err)
		return false
	}
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.Lg.Errorf("error splitting key into namespace + name %s\n", err)
		return false
	}

	err = c.syncHandler(ns, name)
	if err != nil {
		//  retry
		c.Lg.Errorf("error syncing cert: %s", err.Error())
		return false
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two.
func (c *CertController) syncHandler(ns, name string) error {
	ctx := context.Background()
	// allnamespaces: v1.NamespaceAll

	fmt.Println("")
	fmt.Println("################")

	fmt.Println("certificate ns: ", ns)
	fmt.Println("certificate name: ", name)

	// Calling the certificate object from informers cache
	crt, err := c.certLister.Certificates(ns).Get(name)
	if err != nil {
		// The certificate resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			c.Lg.Infof("did not find cert %s / %s", ns, name)
			return nil
		}
		return err
	}

	if crt.Status.Conditions == nil {
		// FIXME: BackOff retry? >> an option how much backoff ?
		c.Lg.Info("probably just created: ", crt.GetName())
		// return fmt.Errorf("nil, we need to retry")
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

func (c *CertController) handleCertAdd(obj interface{}) {
	c.Lg.Info("handleCertAdd called")
	c.queue.Add(obj)
}
