package main

import (
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Rouzip/goperf/pkg/pod"
	"github.com/Rouzip/goperf/pkg/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

const namespace = "default"

// run collector for 100 containers
func main() {
	var wg sync.WaitGroup
	wg.Add(1)
	ctx := utils.SetUpContext()
	go func() {
		<-ctx.Done()
		wg.Done()
	}()

	// if the os.Args[2] doesn't exist, exit and report error
	if len(os.Args) < 3 {
		klog.Fatal("Please input the node name and the path of kubeconfig.")
	}

	// set the kubeconfig
	utils.SetKubeConfig(os.Args[2])

	go wait.Until(func() {
		// node name
		pods, err := utils.GetPods(os.Args[1], namespace)

		if err != nil {
			klog.Fatal(err)
		}
		// print the count of pods
		klog.Infof("There are %d pods in the default namespace.", len(pods))
		collector, err := pod.GeneratePodCollector("libpfm4", pods)
		if err != nil {
			klog.Fatal(err)
		}
		// collect perf CPI for 10s
		time.Sleep(time.Second * 5)
		// collector
		collector.Profile()
	}, 10*time.Second, ctx.Done())
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":8080", nil)
	wg.Wait()
}
