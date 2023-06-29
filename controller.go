package main

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	cmInformerV1 "github.com/cert-manager/cert-manager/pkg/client/informers/externalversions/certmanager/v1"
	cmListerV1 "github.com/cert-manager/cert-manager/pkg/client/listers/certmanager/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type CMController struct {
	clientset       versioned.Clientset
	certLister      cmListerV1.CertificateLister
	certCacheSynced cache.InformerSynced
	queue           workqueue.RateLimitingInterface
}

func NewCMController(clientset versioned.Clientset, cmInformer cmInformerV1.CertificateInformer) *CMController {
	c := &CMController{
		clientset:       clientset,
		certLister:      cmInformer.Lister(),
		certCacheSynced: cmInformer.Informer().HasSynced,
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "certificate-controller"),
	}

	cmInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: c.handleCertAdd,
			// DeleteFunc: c.handleCertDel,
		})

	return c
}

func (c *CMController) Run(ch <-chan struct{}) {
	if !cache.WaitForCacheSync(ch, c.certCacheSynced) {
		fmt.Println("waiting for cache to be synced")
	}

	wait.Until(c.worker, 1*time.Second, ch)

	<-ch
}

func (c *CMController) worker() {
	for c.processNextWorkItem() {

	}
}

func (c *CMController) processNextWorkItem() bool {
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

	err = c.syncHandler(ns, name)
	if err != nil {
		//  retry
		fmt.Printf("error syncing cert: %s", err.Error())
		return false
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two.
func (c *CMController) syncHandler(ns, name string) error {
	ctx := context.Background()
	// allnamespaces: v1.NamespaceAll

	fmt.Println("")
	fmt.Println("################")

	fmt.Println("certificate ns: ", ns)
	fmt.Println("certificate name: ", name)

	// Calling the kubeAPI, retrieving this certificate
	// cert, err := c.clientset.CertmanagerV1().Certificates(ns).Get(ctx, name, metav1.GetOptions{})
	// if err != nil {
	// 	fmt.Printf("something went wrong getting certificate: %s\n", err)
	// 	return err
	// }

	// for _, v := range cert.Status.Conditions {
	// 	if v.Status != "True" {
	// 		fmt.Println("status is not TRUE")
	// 		fmt.Println(cert.GetName())
	// 		fmt.Println("cert Message is: ", v.Message)
	// 	}

	// }

	// Calling the certificate object from informers cache
	crt, err := c.certLister.Certificates(ns).Get(name)
	if err != nil {
		// The certificate resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			fmt.Printf("did not find cert %s / %s", ns, name)
			return nil
		}
		return err
	}

	for _, v := range crt.Status.Conditions {
		if v.Reason != "Ready" {
			fmt.Printf("deleting certificate: %s / %s", ns, name)
		}

		if v.Status != "True" {
			fmt.Println("status is not TRUE")
			fmt.Println(crt.GetName())
			fmt.Println("cert Message is: ", v.Message)

			// take some time checking if a cert needs to be deleted,
			// sometimes a cert needs a few seconds to come

			c.clientset.CertmanagerV1().Certificates(ns).Delete(ctx, name, metav1.DeleteOptions{})
			if err != nil {
				fmt.Println("something went wrong deleting secret")
				return err
			}
			fmt.Printf("deleted certificate: %s / %s", ns, name)

		}
	}

	fmt.Printf("cert from cache: %s \n ", crt.GetName())
	fmt.Println(crt.Status)

	return nil
}

func (c *CMController) handleCertAdd(obj interface{}) {
	fmt.Println("handleCertAdd called")
	c.queue.Add(obj)
}
