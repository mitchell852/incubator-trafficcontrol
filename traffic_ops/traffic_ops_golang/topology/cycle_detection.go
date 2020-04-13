package topology

import (
	"github.com/apache/trafficcontrol/lib/go-tc"
	"math"
)

type TarjanNode struct {
	*tc.TopologyNode
	Index   *int
	LowLink *int
	OnStack *bool
}

type NodeStack []*TarjanNode
type Graph []*TarjanNode
type Component []*tc.TopologyNode

type Tarjan struct {
	Graph      *Graph
	Stack      *NodeStack
	Components *[]*Component
	Index      int
}

func (stack *NodeStack) push(node *TarjanNode) {
	*stack = append(append([]*TarjanNode{}, (*stack)...), node)
}

func (stack *NodeStack) pop() *TarjanNode {
	length := len(*stack)
	node := (*stack)[length-1]
	*stack = (*stack)[:length-1]
	return node
}

func tarjan(nodes *[]*tc.TopologyNode) *[]*[]*tc.TopologyNode {
	structs := Tarjan{
		Graph:      &Graph{},
		Stack:      &NodeStack{},
		Components: &[]*Component{},
		Index:      0,
	}
	for _, node := range *nodes {
		tarjanNode := TarjanNode{TopologyNode: node, LowLink: new(int)}
		*tarjanNode.LowLink = 500
		*structs.Graph = append(*structs.Graph, &tarjanNode)
	}
	structs.Stack = &NodeStack{}
	structs.Index = 0
	for _, vertex := range *structs.Graph {
		if vertex.Index == nil {
			structs.strongConnect(vertex)
		}
	}
	components := &[]*[]*tc.TopologyNode{}
	for _, component := range *structs.Components {
		componentArray := new([]*tc.TopologyNode)
		for _, node := range *component {
			*componentArray = append(*componentArray, node)
		}
		*components = append(*components, componentArray)
	}
	return components
}

func (structs *Tarjan) nextComponent() *Component {
	component := &Component{}
	*structs.Components = append(*structs.Components, component)
	return component
}

func (structs *Tarjan) strongConnect(node *TarjanNode) {
	stack := structs.Stack
	node.Index = new(int)
	*node.Index = structs.Index
	node.LowLink = new(int)
	*node.LowLink = structs.Index
	structs.Index++
	stack.push(node)
	node.OnStack = new(bool)
	*node.OnStack = true

	for _, parentIndex := range node.Parents {
		parent := (*structs.Graph)[parentIndex]
		if parent.Index == nil {
			structs.strongConnect(parent)
			*(*parent).LowLink = int(math.Min(float64(*node.LowLink), float64(*parent.LowLink)))
		} else if *parent.OnStack {
			*node.LowLink = int(math.Min(float64(*node.LowLink), float64(*parent.Index)))
		}
	}

	if *node.LowLink == *node.Index {
		component := structs.nextComponent()
		var otherNode *TarjanNode = nil
		for node != otherNode {
			otherNode = stack.pop()
			*otherNode.OnStack = false
			*component = append(*component, otherNode.TopologyNode)
		}
	}
}
