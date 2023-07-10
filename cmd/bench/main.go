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

// run collector for 100 containers
func main() {
	var wg sync.WaitGroup
	wg.Add(1)
	ctx := utils.SetUpContext()
	go func() {
		<-ctx.Done()
		wg.Done()
	}()

	go wait.Until(func() {
		// node name
		pods, err := utils.GetTestPods(os.Args[1])
		if err != nil {
			klog.Fatal(err)
		}
		collector, err := pod.GeneratePodCollector("goraw", pods)
		if err != nil {
			klog.Fatal(err)
		}
		// collect perf CPI for 10s
		time.Sleep(time.Second * 10)
		// collector
		collector.Profile()
	}, 60*time.Second, ctx.Done())
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":8080", nil)
	wg.Wait()
}
