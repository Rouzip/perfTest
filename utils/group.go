package utils

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

const config = "/home/ubuntu/.kube/config"

// get test nginx container cgroup path
func GetTestCgroupPath(node string) ([]string, error) {
	k8sconfig, err := clientcmd.BuildConfigFromFlags("", config)
	if err != nil {
		klog.Fatal(err)
	}
	k8sClient, err := kubernetes.NewForConfig(k8sconfig)
	if err != nil {
		klog.Fatal(err)
	}

	pods, err := k8sClient.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + node,
	})

	for _, pod := range pods.Items {
		klog.Info(pod.Name)
	}

	return nil, nil
	// path := "/sys/fs/cgroup/kubepods.slice"
	// err = filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
	// 	if info.IsDir() && strings.Contains(path, "_") {
	// 		dirs = append(dirs, path)
	// 		return nil
	// 	}
	// 	return nil
	// })
	// if err != nil {
	// 	return nil, err
	// }

	// return dirs, nil
}
