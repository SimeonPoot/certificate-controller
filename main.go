package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	cm "github.com/cert-manager/cert-manager/pkg/client/informers/externalversions"
)

func main() {
	log.Print("Shared Informer app started")
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Panic(err.Error())
	}

	ch := make(chan struct{})
	cmCl, err := versioned.NewForConfig(config)
	if err != nil {
		fmt.Println("error creating certmanager config", err)
	}

	cmInformer := cm.NewSharedInformerFactory(cmCl, 10*time.Minute)
	cmz := newCMController(*cmCl, cmInformer.Certmanager().V1().Certificates())
	cmInformer.Start(ch)
	cmz.run(ch)

	// result, err := client.Resource(certResource).Namespace(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	// if err != nil {
	// 	panic(err)
	// }

	// fmt.Println(result)

}
