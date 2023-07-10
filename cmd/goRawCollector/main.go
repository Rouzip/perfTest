package main

import "github.com/Rouzip/goperf/pkg/utils"

// run collector for 10 containers

func main() {
	utils.GetTestCgroupPath("k8s-node-1")
}
