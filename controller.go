package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	cmInformerV1 "github.com/cert-manager/cert-manager/pkg/client/informers/externalversions/certmanager/v1"
	cmListerV1 "github.com/cert-manager/cert-manager/pkg/client/listers/certmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func newCMController(clientset versioned.Clientset, cmInformer cmInformerV1.CertificateInformer) *CMController {
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

func (c *CMController) run(ch <-chan struct{}) {
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

	err = c.syncCertificate(ns, name)
	if err != nil {
		//  retry
		fmt.Printf("error syncing cert: %s", err.Error())
		return false
	}

	return true
}

func (c *CMController) syncCertificate(ns, name string) error {
	ctx := context.Background()
	// allnamespaces: v1.NamespaceAll

	fmt.Println("certificate ns: ", ns)
	fmt.Println("certificate name: ", name)

	cert, err := c.clientset.CertmanagerV1().Certificates(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("something went wrong getting certificate: %s\n", err)
		return err
	}

	fmt.Println("cert ############: ", cert.Status.Conditions)
	fmt.Println("Cerrrritificate: ", cert.GetName())

	for _, v := range cert.Status.Conditions {
		fmt.Println("cert status is: ", v.Status)
		fmt.Println("cert reason is: ", v.Reason)
		fmt.Println("cert type is: ", v.Type)
		fmt.Println("cert Message is: ", v.Message)
	}

	crt, err := c.certLister.Certificates(ns).Get(name)
	if err != nil {
		fmt.Printf("error getting list %s \n", err)
		return err
	}

	fmt.Printf("cert from cache: %s \n ", crt.GetName())

	return nil
}

func (c *CMController) handleCertAdd(obj interface{}) {
	fmt.Println("handleCertAdd called")
	c.queue.Add(obj)
}
