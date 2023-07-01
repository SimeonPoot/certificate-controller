package main

import (
	"flag"
	"os"
	"time"

	"k8s.io/client-go/tools/clientcmd"

	ctrl "github.com/simeonpoot/certificate-controller/pkg/controllers"
	"github.com/sirupsen/logrus"

	"github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	cminformer "github.com/cert-manager/cert-manager/pkg/client/informers/externalversions"
)

func main() {
	var kubeconfig string
	flag.StringVar(&kubeconfig, "kubeconfig", kubeconfig, "kubeconfig file")

	logLevel := flag.String("log-level", "debug", "logLevels allowed [debug,info,warn,error,fatal].")
	dryRun := flag.Bool("dryrun", false, "true / false, when set true it will only log the certs to be deleted")
	trigger := flag.String("trigger", "", "the error to trigger the deletion in the Status.Condition.Message ")

	flag.Parse()
	log := newLogger(*logLevel)

	log.Info("Shared Informer app started")
	log.Infof("running app with trigger: %s, dryrun-mode: %v", *trigger, *dryRun)

	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatal(err.Error())
	}

	ch := make(chan struct{})
	clientset, err := versioned.NewForConfig(config)
	if err != nil {
		log.Fatalf("error creating certmanager config: %v", err)
	}

	// build CertControllerOptions
	cOpts := ctrl.CertControllerOptions{
		DryRun:  *dryRun,
		Trigger: *trigger,
	}

	// TODO: resync every 10minutes (this doesn't work!)
	cmInformer := cminformer.NewSharedInformerFactory(clientset, 10*time.Minute)
	cController := ctrl.NewCertController(*clientset, cmInformer.Certmanager().V1().Certificates(), cOpts, log)
	cmInformer.Start(ch)
	cController.Run(ch)

}

func newLogger(logLevel string) *logrus.Logger {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	lvl, err := logrus.ParseLevel(logLevel)
	if err != nil {
		log.Errorf("error creating logger: %v", err)
	}
	log.SetLevel(lvl)

	return log
}
