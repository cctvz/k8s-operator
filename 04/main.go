package main

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// config
	// read from local config file (the same as kubectl)
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		panic(err)
	}
	config.GroupVersion = &v1.SchemeGroupVersion
	config.NegotiatedSerializer = scheme.Codecs
	config.APIPath = "/api"

	// client
	restClient, err := rest.RESTClientFor(config)
	if err != nil {
		panic(err)
	}

	// query
	pod := v1.Pod{}
	err = restClient.Get().Namespace("kube-system").Resource("pods").Name("etcd-minikube").Do(context.TODO()).Into(&pod)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(pod.Name)
	}
}
