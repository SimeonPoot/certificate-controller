package main

import (
	"log"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	log.Print("Shared Informer app started")
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Panic(err.Error())
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Panic(err.Error())
	}

	certResource := schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}

	ch := make(chan struct{})
	client.Resource(certResource)
	dynInformers := dynamicinformer.NewDynamicSharedInformerFactory(client, 10*time.Minute)

	c := newDynamicController(*client, dynInformers.ForResource(certResource), certResource)
	dynInformers.Start(ch)
	c.run(ch)

	// result, err := client.Resource(certResource).Namespace(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	// if err != nil {
	// 	panic(err)
	// }

	// fmt.Println(result)

}
