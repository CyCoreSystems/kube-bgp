package nodes

import (
	"context"
	"log"
	"time"

	"github.com/rotisserie/eris"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// MaximumCheckIntervalSeconds is the maximum amount to time to wait before forcing an update check
var MaximumCheckIntervalSeconds = 60

func getClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, eris.Wrap(err, "failed to acquire kubernetes config")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, eris.Wrap(err, "failed to create the kubernetes clientset")
	}

	return clientset, nil
}

// Watcher defines the interface for a Node Watcher
type Watcher interface {

	// Changes waits for a change to the Node set to occur
	Changes() <-chan struct{}

	// Nodes returns the current list of Nodes
	Nodes() []v1.Node

	// Close shuts down the Watcher
	Close()
}

type watcher struct {
	cancel    context.CancelFunc
	clientSet *kubernetes.Clientset
	nodeList  []v1.Node
	sigChan   chan struct{}
}

func (w *watcher) run(ctx context.Context) {
	for {
		if err := w.watchOnce(ctx); err != nil {
			log.Println(err)

			// Prevent runaway short loop.
			// TODO: handle this better
			time.Sleep(time.Second)
		}

		changed, err := w.updateList(ctx)
		if err != nil {
			log.Println("failed to update node list:", err)
			continue
		}

		if changed {
			w.sigChan <- struct{}{}
		}
	}
}

func (w *watcher) watchOnce(ctx context.Context) error {
	wtch, err := w.clientSet.CoreV1().Nodes().Watch(metav1.ListOptions{})
	if err != nil {
		return eris.Wrap(err, "failed to create node watcher")
	}
	defer wtch.Stop()

	select {
	case <-ctx.Done():
	case <-time.After(time.Duration(MaximumCheckIntervalSeconds) * time.Second):
	case <-wtch.ResultChan():
	}

	return nil
}

func (w *watcher) Changes() <-chan struct{} {
	return w.sigChan
}

func (w *watcher) Nodes() []v1.Node {
	return w.nodeList
}

func (w *watcher) Close() {
	w.cancel()
}

func (w *watcher) updateList(ctx context.Context) (changed bool, err error) {
	newList, err := w.clientSet.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return false, eris.Wrap(err, "failed to obtain list of nodes")
	}

	if len(newList.Items) != len(w.nodeList) {
		w.nodeList = newList.Items
		return true, nil
	}

	for _, newNode := range newList.Items {
		var newNodeFound bool

		for _, oldNode := range w.nodeList {
			if oldNode.Name == newNode.Name {
				newNodeFound = true

				if addressesDiffer(newNode.Status.Addresses, oldNode.Status.Addresses) {
					return true, nil
				}

				break // nodes are the same
			}
		}

		if !newNodeFound {
			return true, nil
		}
	}

	return false, nil
}

func addressesDiffer(a, b []v1.NodeAddress) bool {
	if len(a) != len(b) {
		return true
	}

	for _, aAddr := range a {
		var addrFound bool

		for _, bAddr := range b {
			if aAddr == bAddr {
				addrFound = true
				break
			}
		}

		if !addrFound {
			return true
		}
	}

	return false
}

// NewWatcher returns a new Nodes watcher which signals whenever the set of Nodes or the IPs of existing Nodes change
func NewWatcher(ctx context.Context, clientSet *kubernetes.Clientset) (Watcher, error) {
	clientSet, err := getClient()
	if err != nil {
		return nil, eris.Wrap(err, "failed to create client")
	}

	localCtx, cancel := context.WithCancel(ctx)

	w := &watcher{
		cancel:    cancel,
		clientSet: clientSet,
		sigChan:   make(chan struct{}, 1),
	}

	go w.run(localCtx)

	return w, nil
}
