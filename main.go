package main

import (
	"context"
	"log"
	"os"

	"github.com/CyCoreSystems/kube-bgp/nodes"
	"github.com/rotisserie/eris"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var configFile = "/etc/kube-bgp/kube-bgp.yaml"
var outputFile = "/etc/gobgp/gobgp.conf"

// Router is an eBGP router to which we whould peer
var Router struct {
	// Address is the address of the router
	Address string `yaml:"address"`

	// ASN is the Autonomous Service Number of the router.
	// This is optional, and if not supplied, the system ASN will be used.
	ASN string `yaml:"asn"`

	// PeerNodes is the list of Node names which should peer with this Router
	PeerNodes []string `yaml:"peerNodes"`
}

// Peer describes an iBGP peer with which we should exchange routes.
var Peer struct {
	// Address is the address of the iBGP peer
	Address string `yaml:"address"`

	// Name is the kubernetes Node name of the iBGP peer
	Name string `yaml:"name"`
}

// KubeBGPConfig describes the configuration structure of Kube-BGP
type KubeBGPConfig struct {
	// ASN is the Autonomous Service Number of the iBGP network
	ASN string `yaml:"asn"`

	// RouterID is the BGP routerID to be used for this node.
	// This is not normally manually supplied by the user, but is calculated from the environment.
	// If supplied, the supplied value will override any // auto-calculated one.
	RouterID string `yaml:"routerID"`

	// Routers is the list of eBGP routers to which we should reflect routes.
	// This is optional.
	Routers []Router `yaml:"routers"`

	// Peers is the list of iBGP peers between which we should exchange routes.
	// This should not be supplied by the user.
	// It will be automatically calculated based on the Nodes in the cluster.
	Peers []Peer `yaml:"-"`
}

func main() {
	ctx := context.Background()

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		log.Fatalln("NODE_NAME must be set")
	}

	cfg, err := loadConfig(configFile)
	if err != nil {
		log.Fatalln("failed to read configuration:", err)
	}

	kubeconfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalln("failed to acquire kubernetes config:", err)
	}

	clientset, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		log.Fatalln("failed to create the kubernetes clientset:", err)
	}

	nodeWatcher, err := nodes.NewWatcher(ctx, clientset)
	if err != nil {
		log.Fatalln("failed to create node watcher:", err)
	}

	// Run once to begin
	if err := export(nodeName, cfg, nodeWatcher.Nodes()); err != nil {
		log.Fatalln("failed to export config:", err)
	}

	// Notify gobgp of updated config.
	// Because we cannot guarantee gobgp is up yet, this command should be allowed to fail.
	notify(outputFile) // nolint: errcheck

	for ctx.Err() == nil {
		<-nodeWatcher.Changes()

		if err := export(nodeName, cfg, nodeWatcher.Nodes()); err != nil {
			log.Fatalln("failed to export config:", err)
		}

		if err := notify(outputFile); err != nil {
			log.Println("failed to notify gobgp of updated config:", err)
		}
	}
}

func loadConfig(filename string) (*KubeBGPConfig, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to open config file %s", filename)
	}

	cfg := new(KubeBGPConfig)
	if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
		return nil, eris.Wrap(err, "failed to decode config file")
	}

	return cfg, nil
}

var configTemplateString = `
[global.config]
router-id = {{ .RouterID }}
as = {{ .ASN }}

{{ if .IsReflector }}
{{ for _, r := .Routers }}
[[neighbors]]
  [neighbors.config]
    neighbor-address = "{{ r.Address }}"
	 peer-as = {{ r.ASN }}
{{ end }}
{{ end }}
`

func export(thisNode string, cfg *KubeBGPConfig, nodeList []v1.Node) error {
	return eris.New("TODO: export unimplemented")
}
