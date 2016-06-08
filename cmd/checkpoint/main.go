package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api/v1"
)

var manifestBytes = []byte(`
{
"apiVersion": "v1",
"kind": "Pod",
"metadata": {
  "name": "temp-apiserver",
  "namespace": "kube-system"
},
"spec": {}
}
`)

var (
	tempAPIServer = []byte("temp-apiserver")
	kubeAPIServer = []byte("kube-apiserver")
)

const (
	kubeletAPIPodsURL = "http://localhost:10255/pods"
	ignorePath        = "/srv/kubernetes/manifests/apiserver.json"
	activePath        = "/etc/kubernetes/manifests/apiserver.json"
)

func main() {
	log.Println("begin apiserver checkpointing...")
	run()
}

func run() {
	var manifest v1.Pod
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		log.Fatal(err)
	}
	for {
		rawPods := getPods()
		switch {
		case bothAPIServersRunning(rawPods):
			log.Println("both temp and kube apiserver running, removing temp apiserver")
			// Both the self-hosted API Server and the temp API Server are running.
			// Remove the temp API Server manifest from the config dir so that the
			// kubelet will stop it.
			if err := os.Remove(activePath); err != nil {
				log.Println(err)
			}
		case kubeSystemAPIServerRunning(rawPods):
			log.Println("kube-apiserver found, creating temp-apiserver manifest")
			// The self-hosted API Server is running. Let's snapshot the pod,
			// clean it up a bit, and then save it to the ignore path for
			// later use.
			var podList v1.PodList
			var apiPod v1.Pod
			if err := json.Unmarshal(rawPods, &podList); err != nil {
				log.Fatal(err)
			}
			for _, p := range podList.Items {
				if strings.Contains(p.Name, string(kubeAPIServer)) {
					apiPod = p
					break
				}
			}
			cleanVolumes(&apiPod)
			modifyInsecurePort(&apiPod)
			manifest.Spec = apiPod.Spec
			m, err := json.Marshal(manifest)
			if err != nil {
				log.Fatal(err)
			}
			if err := ioutil.WriteFile(ignorePath, m, 0644); err != nil {
				log.Fatal(err)
			}
			log.Printf("finished creating temp-apiserver manifest at %s\n", ignorePath)

		default:
			log.Println("no apiserver running, installing temp apiserver static manifest")
			b, err := ioutil.ReadFile(ignorePath)
			if err != nil {
				log.Println(err)
				continue
			}
			if err := ioutil.WriteFile(activePath, b, 0644); err != nil {
				log.Println(err)
			}
		}
		time.Sleep(60 * time.Second)
	}
}

func getPods() []byte {
	var pods []byte
	res, err := http.Get(kubeletAPIPodsURL)
	if err != nil {
		log.Println(err)
		return pods
	}
	pods, err = ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Println(err)
	}
	return pods
}

func bothAPIServersRunning(pods []byte) bool {
	return bytes.Contains(pods, tempAPIServer) && bytes.Contains(pods, kubeAPIServer)
}

func kubeSystemAPIServerRunning(pods []byte) bool {
	return bytes.Contains(pods, kubeAPIServer)
}

// cleanVolumes will sanitize the list of volumes and volume mounts
// to remove the default service account token.
func cleanVolumes(p *v1.Pod) {
	volumes := make([]v1.Volume, 0, len(p.Spec.Volumes))
	volumeMounts := make([]v1.VolumeMount, 0, len(p.Spec.Volumes))
	for _, v := range p.Spec.Volumes {
		if !strings.Contains(v.Name, "default") {
			volumes = append(volumes, v)
		}
	}
	for _, vm := range p.Spec.Containers[0].VolumeMounts {
		if !strings.Contains(vm.Name, "default") {
			volumeMounts = append(volumeMounts, vm)
		}
	}
	p.Spec.Volumes = volumes
	p.Spec.Containers[0].VolumeMounts = volumeMounts
}

func modifyInsecurePort(p *v1.Pod) {
	cmds := p.Spec.Containers[0].Command
	for i, c := range cmds {
		if strings.Contains(c, "insecure-port") {
			cmds[i] = strings.Replace(c, "8080", "8081", 1)
			break
		}
	}
}
