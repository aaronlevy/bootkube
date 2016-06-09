package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_2"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
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

const (
	kubeletAPIPodsURL = "http://localhost:10255/pods"
	ignorePath        = "/srv/kubernetes/manifests"
	activePath        = "/etc/kubernetes/manifests"
	manifestFilename  = "apiserver.json"
	kubeconfigPath    = "/etc/kubernetes/kubeconfig"
	tlsPath           = "/etc/kubernetes/tls"
)

var (
	tempAPIServer = []byte("temp-apiserver")
	kubeAPIServer = []byte("kube-apiserver")
	activeFile    = filepath.Join(activePath, manifestFilename)
	ignoreFile    = filepath.Join(ignorePath, manifestFilename)
	secureAPIAddr = fmt.Sprintf("https://%s:%s", os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT_HTTPS"))
)

func main() {
	log.Println("begin apiserver checkpointing...")
	run()
}

func run() {
	var tempAPIServerManifest v1.Pod
	if err := json.Unmarshal(manifestBytes, &tempAPIServerManifest); err != nil {
		log.Fatal(err)
	}
	client := newAPIClient()
	for {
		var podList v1.PodList
		if err := json.Unmarshal(getPodsFromKubeletAPI(), &podList); err != nil {
			log.Fatal(err)
		}
		switch {
		case bothAPIServersRunning(podList):
			log.Println("both temp and kube apiserver running, removing temp apiserver")
			// Both the self-hosted API Server and the temp API Server are running.
			// Remove the temp API Server manifest from the config dir so that the
			// kubelet will stop it.
			if err := os.Remove(activeFile); err != nil {
				log.Println(err)
			}
		case kubeSystemAPIServerRunning(podList):
			log.Println("kube-apiserver found, creating temp-apiserver manifest")
			// The self-hosted API Server is running. Let's snapshot the pod,
			// clean it up a bit, and then save it to the ignore path for
			// later use.
			tempAPIServerManifest.Spec = parseAPIPodSpec(podList)
			convertSecretsToVolumeMounts(client, &tempAPIServerManifest.Spec)
			writeManifest(tempAPIServerManifest)
			log.Printf("finished creating temp-apiserver manifest at %s\n", ignoreFile)

		default:
			log.Println("no apiserver running, installing temp apiserver static manifest")
			b, err := ioutil.ReadFile(ignoreFile)
			if err != nil {
				log.Println(err)
			} else {
				if err := ioutil.WriteFile(activeFile, b, 0644); err != nil {
					log.Println(err)
				}
			}
		}
		time.Sleep(60 * time.Second)
	}
}

func getPodsFromKubeletAPI() []byte {
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

func bothAPIServersRunning(pods v1.PodList) bool {
	var kubeAPISeen, tempAPISeen bool
	for _, p := range pods.Items {
		switch {
		case isKubeAPI(p):
			kubeAPISeen = true
		case isTempAPI(p):
			tempAPISeen = true
		}
		if kubeAPISeen && tempAPISeen {
			return true
		}
	}
	return false
}

func kubeSystemAPIServerRunning(pods v1.PodList) bool {
	for _, p := range pods.Items {
		if isKubeAPI(p) {
			return true
		}
	}
	return false
}

func isKubeAPI(pod v1.Pod) bool {
	return strings.Contains(pod.Name, "kube-apiserver") && pod.Namespace == api.NamespaceSystem
}

func isTempAPI(pod v1.Pod) bool {
	return strings.Contains(pod.Name, "temp-apiserver") && pod.Namespace == api.NamespaceSystem
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

// writeManifest will write the manifest to the ignore path.
// It first writes the file to a temp file, and then atomically moves it into
// the actual ignore path and correct file name.
func writeManifest(manifest v1.Pod) {
	m, err := json.Marshal(manifest)
	if err != nil {
		log.Fatal(err)
	}
	writeAndAtomicCopy(m, ignoreFile)
}

func parseAPIPodSpec(podList v1.PodList) v1.PodSpec {
	var apiPod v1.Pod
	for _, p := range podList.Items {
		if strings.Contains(p.Name, string(kubeAPIServer)) {
			apiPod = p
			break
		}
	}
	cleanVolumes(&apiPod)
	modifyInsecurePort(&apiPod)
	return apiPod.Spec
}

func newAPIClient() clientset.Interface {
	kubeConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&clientcmd.ClientConfigLoadingRules{
		ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		log.Fatal(err)
	}
	client, err := clientset.NewForConfig(&restclient.Config{
		Host: secureAPIAddr,
		TLSClientConfig: restclient.TLSClientConfig{
			CertData: kubeConfig.CertData,
			KeyData:  kubeConfig.KeyData,
			CAData:   kubeConfig.CAData,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	return client
}

func convertSecretsToVolumeMounts(client clientset.Interface, spec *v1.PodSpec) {
	log.Println("converting secrets to volume mounts")
	for i := range spec.Volumes {
		v := &spec.Volumes[i]
		if v.Name == "secrets" && v.Secret != nil {
			v.HostPath = &v1.HostPathVolumeSource{
				Path: tlsPath,
			}
			copySecretsToDisk(client, v.Secret.SecretName)
			v.Secret = nil
			break
		}
	}
}

func copySecretsToDisk(client clientset.Interface, name string) {
	log.Println("copying secrets to disk")
	if err := os.MkdirAll(tlsPath, 0755); err != nil {
		log.Fatal(err)
	}
	log.Printf("created directory %s", tlsPath)
	s, err := client.Core().Secrets(api.NamespaceSystem).Get(name)
	if err != nil {
		log.Fatal(err)
	}
	for name, value := range s.Data {
		writeAndAtomicCopy(value, filepath.Join(tlsPath, name))
	}
}

func writeAndAtomicCopy(data []byte, path string) {
	// First write a "temp" file.
	tmpfile := filepath.Join(filepath.Dir(path), "."+filepath.Base(path))
	if err := ioutil.WriteFile(tmpfile, data, 0644); err != nil {
		log.Fatal(err)
	}
	// Finally, copy that file to the correct location.
	if err := os.Rename(tmpfile, path); err != nil {
		log.Fatal(err)
	}
}
