package main

import (
	"log"

	"github.com/liyuxuan89/k8s-operator-demo/11/pkg"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// 1. config
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		inClusterConfig, err := rest.InClusterConfig()
		if err != nil {
			log.Fatalln("can't get config")
		}
		config = inClusterConfig
	}

	// 2. client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalln("can't create client")
	}

	// 3. informer
	factory := informers.NewSharedInformerFactory(clientset, 0)
	serviceInformer := factory.Core().V1().Services()
	ingressInformer := factory.Networking().V1().Ingresses()

	// 4. add event handler
	controller := pkg.NewController(clientset, serviceInformer, ingressInformer)

	// 5. start informer
	stopCh := make(chan struct{})
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)
	controller.Run(stopCh)
}
