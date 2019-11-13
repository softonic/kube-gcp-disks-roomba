package main

import (
	"flag"
	"fmt"
	"log"

	"regexp"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"k8s.io/api/core/v1"
	_ "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
)

type pvc struct {
	zone       string
	volumeName string
	pvcName    string
	namespace  string
}

func main() {
	ctx := context.Background()

	c, err := google.DefaultClient(ctx, compute.CloudPlatformScope)
	if err != nil {
		log.Fatal(err)
	}

	computeService, err := compute.New(c)
	if err != nil {
		log.Fatal(err)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	api := clientset.CoreV1()

	ns := ""

	// initial list
	pvcs, err := api.PersistentVolumeClaims(ns).List(metav1.ListOptions{})
	if err != nil {
		log.Fatal(err)
	}

	stan := getPVCs(pvcs)

	if len(stan) == 0 {
		fmt.Println("There is no pvc candidate to be removed")
	}


	// Project ID for this request.
	project := flag.String("project", "foo", "a string")
	zones := []string{}

	flag.Parse()

	zones = flag.Args()

	p := pvc{}
	candidate := make(map[string]pvc)
	for _, z := range zones {
		req := computeService.Disks.List(*project, z)
		if err := req.Pages(ctx, func(page *compute.DiskList) error {
			for _, disk := range page.Items {
				if disk.Users == nil {
					// this means that the pvc is disk is not in use
					if disk.SourceSnapshot == "" {
						reVolumeName := regexp.MustCompile(`.*kubernetes.io\/created-for\/pv\/name":"([a-zA-Z0-9\-]+)",`)
						rePvc := regexp.MustCompile(`.*kubernetes.io\/created-for\/pvc\/name":"([a-zA-Z0-9\-]+)",`)
						reNamespace := regexp.MustCompile(`.*kubernetes.io\/created-for\/pvc\/namespace":"([a-zA-Z0-9\-]+)"`)
						p.volumeName = disk.Name
						p.zone = z
						volumeName := reVolumeName.FindStringSubmatch(disk.Description)[1]
						p.pvcName = rePvc.FindStringSubmatch(disk.Description)[1]
						p.namespace = reNamespace.FindStringSubmatch(disk.Description)[1]
						candidate[volumeName] = p
					} else {
						re := regexp.MustCompile(`.*-(pvc-.*)`)
						p.volumeName = disk.Name
						p.zone = z
						volumeName := "moved-" + re.FindStringSubmatch(disk.Name)[1]
						candidate[volumeName] = p
					}
				}
			}
			return nil
		}); err != nil {
			log.Fatal(err)
		}
	}

	// Join map candidate and list of standard pvcs and output the resulting final disks to be removed

	for _, k := range stan {
		if val, ok := candidate[k]; ok {
			fmt.Println("Deleting disk:", val.volumeName)

			resp, err := computeService.Disks.Delete(*project, val.zone, val.volumeName).Context(ctx).Do()
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%#v\n", resp)

			// and here I can remove the pv
			fmt.Println("We can delete the pvc with the name", val.pvcName)
			fmt.Println("The namespace is:", val.namespace)

			work := deletePVCs(clientset, val.pvcName, val.namespace)
			fmt.Println(work)
		}
	}

}

// List all the standard pvcs

func getPVCs(pvcs *v1.PersistentVolumeClaimList) []string {
	standard := []string{}
	if len(pvcs.Items) == 0 {
		log.Println("No claims found")
		return standard
	}

	for _, pvc := range pvcs.Items {
		if *pvc.Spec.StorageClassName == "standard" {
			standard = append(standard, pvc.Spec.VolumeName)
		}
	}
	return standard

}

func deletePVCs(clientset *kubernetes.Clientset, pvc string, namespace string) bool {

	api := clientset.CoreV1()

	err := api.PersistentVolumeClaims(namespace).Delete(pvc, nil)
	if err != nil {
		fmt.Println("Error deleting PVC %s\n", pvc)
	} else {
		fmt.Println("Deleting PVC %s\n", pvc)
	}

	return true

}
