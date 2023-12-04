package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var CPUNUM int

// declare a slice of events to collect
var EventsToCollect = []string{"instructions", "cycles", "LONGEST_LAT_CACHE.MISS"}

var KubeConfig string

func SetKubeConfig(value string) {
	KubeConfig = value
}

func init() {
	CPUNUM = runtime.NumCPU()
}

type Unit struct {
	Container string
	Pod       string
	Namespace string
}

type Result struct {
	Cycles       uint64
	Instructions uint64
}

type CPICollector interface {
	Collect() error
	Close() error
}

// get all the pods from the namespace
// input: node_name, namespace
// output: pods, error
func GetPods(node string, namespace string) ([]*v1.Pod, error) {
	k8sconfig, err := clientcmd.BuildConfigFromFlags("", KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from flags: %v", err)
	}
	k8sClient, err := kubernetes.NewForConfig(k8sconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %v", err)
	}

	pods, err := k8sClient.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + node,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %v", err)
	}

	resPods := []*v1.Pod{}
	for _, pod := range pods.Items {
		resPods = append(resPods, pod.DeepCopy())
	}

	return resPods, nil
}

// get test nginx container cgroup path
func GetTestPods(node string) ([]*v1.Pod, error) {
	k8sconfig, err := clientcmd.BuildConfigFromFlags("", KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from flags: %v", err)
	}
	k8sClient, err := kubernetes.NewForConfig(k8sconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %v", err)
	}

	pods, err := k8sClient.CoreV1().Pods("nginx").List(context.Background(), metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + node,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %v", err)
	}

	resPods := []*v1.Pod{}
	for _, pod := range pods.Items {
		resPods = append(resPods, pod.DeepCopy())
	}

	return resPods, nil
}

// just test for containerd
func CGroupFd(pod *v1.Pod, container *v1.ContainerStatus) (*os.File, error) {
	k8sPath := "/sys/fs/cgroup/kubepods.slice/"
	uid := strings.ReplaceAll(string(pod.UID), "-", "_")
	path := filepath.Join(k8sPath, "kubepods-pod"+uid+".slice", ContainerId(container))
	f, err := os.OpenFile(path, os.O_RDONLY, os.ModeDir)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func ContainerId(container *v1.ContainerStatus) string {
	hashs := strings.Split(container.ContainerID, "://")
	return fmt.Sprintf("cri-containerd-%s.scope", hashs[1])
}
