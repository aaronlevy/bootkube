// +build ignore

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"text/template"
	"time"
)

var tmpl = template.Must(template.New("templates.go").Parse(`package internal

// This file was generated by templates_gen.go. DO NOT EDIT by hand.

var (
{{ range $i, $var := .Vars }}	{{ $var.Name }} = _{{ $var.Name }}
{{ end }}
)

var (
{{ range $i, $var := .Vars }}	_{{ $var.Name }} = {{ $var.Data  }}{{ end }}
)
`))

//go:generate go run templates_gen.go
//go:generate gofmt -w internal/templates.go

var files = []struct {
	Filename string
	VarName  string
}{
	{"kubeconfig.yaml", "KubeConfigTemplate"},
	{"kubelet.yaml", "KubeletTemplate"},
	{"kube-apiserver.yaml", "APIServerTemplate"},
	{"kube-controller-manager.yaml", "ControllerManagerTemplate"},
	{"kube-scheduler.yaml", "SchedulerTemplate"},
	{"kube-proxy.yaml", "ProxyTemplate"},
	{"kube-dns-rc.yaml", "DNSRcTemplate"},
	{"kube-dns-svc.yaml", "DNSSvcTemplate"},
	{"kube-system-ns.yaml", "SystemNSTemplate"},
	{"kube-flannel.yaml", "KubeFlannelTemplate"},
	{"kube-flannel-cm.yaml", "KubeFlannelCmTemplate"},
}

type Data struct {
	Vars []Var
	Now  time.Time
}

type Var struct {
	Name string
	Data string
}

func toGoByteSlice(sli []byte) string {
	buff := new(bytes.Buffer)
	fmt.Fprintf(buff, "[]byte{\n")
	for i, b := range sli {
		if i%10 == 0 {
			fmt.Fprintf(buff, "\t%#x,", b)
		} else {
			fmt.Fprintf(buff, " %#x,", b)
		}
		if (i+1)%10 == 0 {
			fmt.Fprintln(buff)
		}
	}
	fmt.Fprintf(buff, "\n}\n")
	return buff.String()
}

func main() {
	tmpls := make([]Var, len(files))
	for i, file := range files {
		path := filepath.Join("templates", file.Filename)
		data, err := ioutil.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}
		tmpls[i] = Var{file.VarName, toGoByteSlice(data)}
	}

	f, err := os.OpenFile("internal/templates.go", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	data := Data{tmpls, time.Now().UTC()}
	if err := tmpl.Execute(f, data); err != nil {
		log.Fatal("Failed to render template:", err)
	}

}
