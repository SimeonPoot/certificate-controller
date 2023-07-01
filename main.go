package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"k8s.io/client-go/tools/clientcmd"

	ctrl "github.com/simeonpoot/certificate-controller/pkg/controllers"

	"github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	cminformer "github.com/cert-manager/cert-manager/pkg/client/informers/externalversions"
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

	cmInformer := cminformer.NewSharedInformerFactory(cmCl, 10*time.Minute)
	cmz := ctrl.NewCMController(*cmCl, cmInformer.Certmanager().V1().Certificates())
	cmInformer.Start(ch)
	cmz.Run(ch)

}
