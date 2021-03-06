package main

import (
	"flag"
	"fmt"
	"log"

	"regexp"

	"github.com/ashwanthkumar/slack-go-webhook"
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

// structs:
type pvc struct {
	zone       string
	volumeName string
	pvcName    string
	namespace  string
}

func main() {

	// implicit uses Application Default Credentials to authenticate
	ctx := context.Background()
	c, err := google.DefaultClient(ctx, compute.CloudPlatformScope)
	if err != nil {
		log.Fatal(err)
	}
	computeService, err := compute.New(c)
	if err != nil {
		log.Fatal(err)
	}

	// client-go uses the Service Account token mounted inside the Pod when the rest.InClusterConfig() is used.

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	ns := ""
	// access the API to list PVCs
	pvcs, err := clientset.CoreV1().PersistentVolumeClaims(ns).List(metav1.ListOptions{})
	if err != nil {
		log.Fatal(err)
	}

	// returns array of pvcs with storage class = standard
	stan := getPVCs(pvcs)

	if len(stan) == 0 {
		fmt.Println("There is no pvc candidate to be removed")
	}

	// Flags and arguments

	projectID := flag.String("project", "foo", "a string")
	slackurl := flag.String("slackurl", "bar", "a string")
	zones := []string{}
	flag.Parse()
	zones = flag.Args()

	protocol := "https://"

	url := protocol + *slackurl

	// iterate zones passed via args and fill the map candidate with the disks that are not in use

	p := pvc{}
	candidate := make(map[string]pvc)
	for _, z := range zones {
		req := computeService.Disks.List(*projectID, z)
		if err := req.Pages(ctx, func(page *compute.DiskList) error {
			for _, disk := range page.Items {
				if disk.Users == nil {

					// this means that the pvc is disk is not in use

					if disk.SourceSnapshot == "" {
						// regexp to get the following data

						reVolumeName := regexp.MustCompile(`.*kubernetes.io\/created-for\/pv\/name":"([a-zA-Z0-9\-]+)",`)
						rePvc := regexp.MustCompile(`.*kubernetes.io\/created-for\/pvc\/name":"([a-zA-Z0-9\-]+)",`)
						reNamespace := regexp.MustCompile(`.*kubernetes.io\/created-for\/pvc\/namespace":"([a-zA-Z0-9\-]+)"`)
						p.volumeName = disk.Name
						p.zone = z
						volumeName := reVolumeName.FindStringSubmatch(disk.Description)[1]
						p.pvcName = rePvc.FindStringSubmatch(disk.Description)[1]
						p.namespace = reNamespace.FindStringSubmatch(disk.Description)[1]

						// fill the map with the key volumeName and the value of a pvc struct type
						candidate[volumeName] = p

					} else {
						fmt.Println("this is snapshot", disk.Name)
						re := regexp.MustCompile(`.*-(pvc-.*)`)
						p.volumeName = disk.Name
						p.zone = z
						if m := re.FindStringSubmatch(disk.Name); m != nil {
							volumeName := "moved-" + m[1]
							candidate[volumeName] = p
						}
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

			resp, err := computeService.Disks.Delete(*projectID, val.zone, val.volumeName).Context(ctx).Do()
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

	for _, val := range candidate {

		attachment1 := slack.Attachment{}
		attachment1.AddField(slack.Field{Title: "PVCName", Value: val.pvcName})
		attachment1.AddField(slack.Field{Title: "VolumeName", Value: val.volumeName})
		attachment1.AddField(slack.Field{Title: "Namespace", Value: val.namespace})
		attachment1.AddAction(slack.Action{Type: "button", Text: "Check in the console", Url: "https://console.cloud.google.com/compute/disks?project=kubertonic", Style: "primary"})
		payload := slack.Payload{
			Text:        "This is a message that reports one disk is not being used in GCP. Do you really need it? check please",
			Username:    "robot",
			Channel:     "#disk-usage-kubernetes",
			IconEmoji:   ":gcp-disks-maintenance:",
			Attachments: []slack.Attachment{attachment1},
		}
		err := slack.Send(url, "", payload)
		if len(err) > 0 {
			fmt.Printf("error: %s\n", err)
		}

	}

	for key, val := range candidate {

		// Si no existe el pv , eliminar el disco que no está en uso
		// If neither PV nor PVC  exist, remove the disk not in used
		noPvcExists(ctx, clientset, key, val, computeService, *projectID)

	}

}

func noPvcExists(ctx context.Context, clientset *kubernetes.Clientset, pv string, c pvc, computeService *compute.Service, projectID string) {

	var err error
	// si el pv no existe en el namespace, se puede borrar el disco ( key)
	api := clientset.CoreV1()
	//var  pvscc v1.PersistentVolumeClaim

	_, err = api.PersistentVolumes().Get(pv, metav1.GetOptions{})
	if err != nil {
		resp, err := computeService.Disks.Delete(projectID, c.zone, c.volumeName).Context(ctx).Do()
		fmt.Printf("%#v\n", resp)
		if err != nil {
			log.Fatal(err)
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

// delete the pvcs associated with the disk removed
func deletePVCs(clientset *kubernetes.Clientset, pvc string, namespace string) bool {

	api := clientset.CoreV1()

	err := api.PersistentVolumeClaims(namespace).Delete(pvc, nil)
	if err != nil {
		fmt.Printf("Error deleting PVC %s\n", pvc)
	} else {
		fmt.Printf("Deleting PVC %s\n", pvc)
	}

	return true

}
