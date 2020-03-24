// Copyright 2019-2020 Authors of Cilium
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nodemanager

import (
	"context"
	"reflect"

	"github.com/cilium/cilium/pkg/ipam"
	"github.com/cilium/cilium/pkg/k8s"
	"github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	clientset "github.com/cilium/cilium/pkg/k8s/client/clientset/versioned"
	"github.com/cilium/cilium/pkg/k8s/informer"
	k8sversion "github.com/cilium/cilium/pkg/k8s/version"
	"github.com/cilium/cilium/pkg/logging"
	"github.com/cilium/cilium/pkg/logging/logfields"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

var log = logging.DefaultLogger.WithField(logfields.LogSubsys, "cilium-operator/tasks/node_manager")

type NodeManager struct {
	*ipam.NodeManager
	clientSet clientset.Interface
}

func NewNodeManager(instancesAPI ipam.AllocationImplementation, clientSet clientset.Interface, metricsAPI ipam.MetricsAPI, parallelWorkers int64, releaseExcessIPs bool) (*NodeManager, error) {
	nodeManager := &NodeManager{
		clientSet: clientSet,
	}

	getterUpdater := &nodeGetterUpdater{nodeManager}

	ipamNodeManager, err := ipam.NewNodeManager(instancesAPI, getterUpdater, metricsAPI, parallelWorkers, releaseExcessIPs)
	if err != nil {
		return nil, err
	}

	nodeManager.NodeManager = ipamNodeManager
	return nodeManager, nil
}

func (nodeManager *NodeManager) StartSyncTask() {
	log.Info("Starting to synchronize CiliumNode custom resources...")

	// TODO: The operator is currently storing a full copy of the
	// CiliumNode resource, as the resource grows, we may want to consider
	// introducing a slim version of it.
	_, ciliumNodeInformer := informer.NewInformer(
		cache.NewListWatchFromClient(nodeManager.clientSet.CiliumV2().RESTClient(),
			"ciliumnodes", v1.NamespaceAll, fields.Everything()),
		&v2.CiliumNode{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if node, ok := obj.(*v2.CiliumNode); ok {
					// node is deep copied before it is stored in pkg/aws/eni
					nodeManager.Update(node)
				} else {
					log.Warningf("Unknown CiliumNode object type %s received: %+v", reflect.TypeOf(obj), obj)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				if node, ok := newObj.(*v2.CiliumNode); ok {
					// node is deep copied before it is stored in pkg/aws/eni
					nodeManager.Update(node)
				} else {
					log.Warningf("Unknown CiliumNode object type %s received: %+v", reflect.TypeOf(newObj), newObj)
				}
			},
			DeleteFunc: func(obj interface{}) {
				deletedObj, ok := obj.(cache.DeletedFinalStateUnknown)
				if ok {
					// Delete was not observed by the
					// watcher but is removed from
					// kube-apiserver. This is the last
					// known state and the object no longer
					// exists.
					if node, ok := deletedObj.Obj.(*v2.CiliumNode); ok {
						nodeManager.Delete(node.Name)
						return
					}
				} else if node, ok := obj.(*v2.CiliumNode); ok {
					nodeManager.Delete(node.Name)
					return
				}
				log.Warningf("Unknown CiliumNode object type %s received: %+v", reflect.TypeOf(obj), obj)
			},
		},
		k8s.ConvertToCiliumNode,
	)

	go ciliumNodeInformer.Run(wait.NeverStop)
}

type nodeGetterUpdater struct{ *NodeManager }

func (n *nodeGetterUpdater) Delete(name string) {
	if err := n.clientSet.CiliumV2().CiliumNodes().Delete(context.TODO(), name, metav1.DeleteOptions{}); err == nil {
		log.WithField("name", name).Info("Removed CiliumNode after receiving node deletion event")
	}
	n.NodeManager.Delete(name)
}

func (n *nodeGetterUpdater) Get(node string) (*v2.CiliumNode, error) {
	return n.clientSet.CiliumV2().CiliumNodes().Get(context.TODO(), node, metav1.GetOptions{})
}

func (n *nodeGetterUpdater) UpdateStatus(node, origNode *v2.CiliumNode) (*v2.CiliumNode, error) {
	// If k8s supports status as a sub-resource, then we need to update the status separately
	k8sCapabilities := k8sversion.Capabilities()
	switch {
	case k8sCapabilities.UpdateStatus:
		if !reflect.DeepEqual(origNode.Status, node.Status) {
			return n.clientSet.CiliumV2().CiliumNodes().UpdateStatus(context.TODO(), node, metav1.UpdateOptions{})
		}
	default:
		if !reflect.DeepEqual(origNode.Status, node.Status) {
			return n.clientSet.CiliumV2().CiliumNodes().Update(context.TODO(), node, metav1.UpdateOptions{})
		}
	}

	return nil, nil
}

func (n *nodeGetterUpdater) Update(node, origNode *v2.CiliumNode) (*v2.CiliumNode, error) {
	// If k8s supports status as a sub-resource, then we need to update the status separately
	k8sCapabilities := k8sversion.Capabilities()
	switch {
	case k8sCapabilities.UpdateStatus:
		if !reflect.DeepEqual(origNode.Spec, node.Spec) {
			return n.clientSet.CiliumV2().CiliumNodes().Update(context.TODO(), node, metav1.UpdateOptions{})
		}
	default:
		if !reflect.DeepEqual(origNode, node) {
			return n.clientSet.CiliumV2().CiliumNodes().Update(context.TODO(), node, metav1.UpdateOptions{})
		}
	}

	return nil, nil
}
