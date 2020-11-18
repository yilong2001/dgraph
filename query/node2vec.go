/*
* 参数说明：
      rval: 迭代次数
      lval: L 个 adj node
      pval: 1/p => d(tx) == 0, 回到上一次访问的顶点的概率参数
      qval: 1/q => d(tx) == 2, 到下一个顶点的概率参数，下一个顶点和上一个顶点没有直接边
          : 1   => d(tx) == 1，到下一个顶点的概率参数，下一个顶点与上一个顶点有直接边
* 
*/
package query

import (
	//"container/heap"
	"context"
	"fmt"
	"math/rand"
	//"math"
	//"sync"
	"sort"

	"github.com/golang/glog"
	//"github.com/dgraph-io/dgraph/algo"
	"github.com/dgraph-io/dgraph/protos/pb"
	//"github.com/dgraph-io/dgraph/types"
	//"github.com/dgraph-io/dgraph/types/facets"
	//"github.com/dgraph-io/dgraph/x"
	//"github.com/pkg/errors"
)

func unionUidSet(slice1, slice2 []int64) []int64 {
	m := make(map[int64]int)
	for _, v := range slice1 {
		m[v]++
	}
 
	for _, v := range slice2 {
		times, _ := m[v]
		if times == 0 {
			slice1 = append(slice1, v)
		}
	}
	return slice1
}
 
func intersectUidSet(slice1, slice2 []int64) []int64 {
	m := make(map[int64]int)
	nn := make([]int64, 0)
	for _, v := range slice1 {
		m[v]++
	}
 
	for _, v := range slice2 {
		times, _ := m[v]
		if times == 1 {
			nn = append(nn, v)
		}
	}

	return nn
}
 
func differenceUidSet(slice1, slice2 []int64) []int64 {
	m := make(map[int64]int)
	nn := make([]int64, 0)
	inter := intersectUidSet(slice1, slice2)
	for _, v := range inter {
		m[v]++
	}
 
	for _, value := range slice1 {
		times, _ := m[value]
		if times == 0 {
			nn = append(nn, value)
		}
	}
	return nn
}

type UidList struct {
	Uids []uint64
}

type UidThreshod struct {
	Uid         uint64
	Threshod    int
}

type UidProbList struct {
	UidThs []UidThreshod
}

type Node2VecNbr struct {
	QVal float32
	PVal float32
	LVal int
	RVal int 

	//key-uid has neighbors of value-uid
	// t -> v -> (x, prob)
	EdgeProbs map[uint64]map[uint64]*UidProbList

	//key-uid is neighbor of value-uid 
	Walks  map[uint64]*UidList
}

func NewNode2VecNbr(qval, pval float32, lval, rval int) *Node2VecNbr {
	return &Node2VecNbr{
			Walks: make(map[uint64]*UidList),
			EdgeProbs: make(map[uint64]map[uint64]*UidProbList),
			QVal: qval,
			PVal: pval,
			LVal: lval,
			RVal: rval}
}

func (nvn *Node2VecNbr) Display() {
	glog.Infof("\nNode2VecNbr EdgeProbs : \n %q \n", nvn.EdgeProbs)
	glog.Infof("\nNode2VecNbr Walks : \n %q \n", nvn.Walks)
}

func (nvn *Node2VecNbr) InitTransitionProbs(sg *SubGraph) {
	if sg == nil {
		glog.Errorf(" InitTransitionProbs, but sg == nil")
		return
	}

	if sg.SrcUIDs == nil {
		glog.Errorf(" InitTransitionProbs, but sg.SrcUIDs == nil ")
		return
	}

	if len(sg.uidMatrix) < len(sg.SrcUIDs.Uids) {
		glog.Errorf(" InitTransitionProbs, but uidMatrix len < SrcUIDs len ")
		return
	}

	// SrcUID one by one, uidMaxtrix has destUIDs
	for i:=0; i < len(sg.SrcUIDs.Uids); i++ {
		tkey := sg.SrcUIDs.Uids[i]
		for j := 0; j < len(sg.uidMatrix[i].Uids); j++ {
			vkey := sg.uidMatrix[i].Uids[j]

			// t -> v
			if _, ok := nvn.EdgeProbs[tkey]; !ok {
				nvn.EdgeProbs[tkey] = make(map[uint64]*UidProbList)
			}

			tkeyNbrMap, _ := nvn.EdgeProbs[tkey]
			tkeyNbrMap[vkey] = &UidProbList{UidThs: make([]UidThreshod, 0)} 
		}
	}

	for i:=0; i < len(sg.SrcUIDs.Uids); i++ {
		tkey := sg.SrcUIDs.Uids[i]
		tkeyNbrMap, _ := nvn.EdgeProbs[tkey]

		for j := 0; j < len(sg.uidMatrix[i].Uids); j++ {
			vkey := sg.uidMatrix[i].Uids[j]

			// t -> v -> x, get vkey's Nbrs to find common nbrs for tkey and vkey
			vkeyNbrMap, ok := nvn.EdgeProbs[vkey]
			if !ok || len(vkeyNbrMap) == 0 {
				glog.Errorf(" Deepwalk InitTransitionProbs, t->v, v not exist : %q, %q \n", tkey, vkey)
				continue
			}

			tvx, ok := tkeyNbrMap[vkey]
			if !ok || tvx == nil {
				glog.Errorf(" Deepwalk InitTransitionProbs, t->v->x, x not exist : %q, %q \n", tkey, vkey)
				continue
			}

			var probs []float32 = make([]float32, len(vkeyNbrMap) + 1)
			var normProbs []int = make([]int, len(vkeyNbrMap) + 1)

			k := 0
			for xkey, _ := range vkeyNbrMap {
				if _, exist := tkeyNbrMap[xkey]; exist {
					probs[k] = 1.0 / nvn.PVal
				} else {
					probs[k] = 1.0 / nvn.QVal
				}
				k++
				tvx.UidThs = append(tvx.UidThs, UidThreshod{xkey, 0})
			}
			// t -> v -> t, last pos is for tkey self
			tvx.UidThs = append(tvx.UidThs, UidThreshod{tkey, 0})

			probs[len(vkeyNbrMap)] = 1
			var sum float32 = 0
			for _, prob := range probs {
				sum += prob
			}
			
			for k:=0; k<len(probs); k++ {
				probs[k] = probs[k]/sum
			}

			var maxrange int = 0
			for k:=0; k<len(probs); k++ {
				normProbs[k] = maxrange
				maxrange += int(probs[k]*MAX_RANGE)
			}

			for k := 0; k < len(probs)-1; k++ {
				tvx.UidThs[k].Threshod = normProbs[k]
			}

			tvx.UidThs[len(probs)-1].Threshod = normProbs[len(probs)-1]
		}
	}
}

func (nvn *Node2VecNbr) DeepWalkOnce() {
	for tkey, _ := range nvn.EdgeProbs {
		var walks []uint64 = make([]uint64,0)
		vkeys := nvn.EdgeProbs[tkey]
		vkeyslen := len(vkeys)

		var needReselect bool = false
		walks = append(walks, tkey)

		for l:=1; l<nvn.LVal;  {
			if l == 1 || needReselect {
				tkeyid := rand.Intn(vkeyslen)
				tmp := 0

				// random select start node
				for vkey, _ := range vkeys {
					if tmp++; tmp >= tkeyid {
						walks = append(walks, vkey)
						break
					}
				}
				// from edge in tkey->startNode, start walk
				l++
				needReselect = false
			} else {
				vkeys, _ = nvn.EdgeProbs[walks[l-2]]
				xkeys, exist := vkeys[walks[l-1]]
				if exist {
					l++
					maxrange := rand.Intn(MAX_RANGE)

					if len(xkeys.UidThs) == 0 {
						glog.Errorf("DeepwalkOnce, %v, %v no nbrs \n", walks[l-3], walks[l-2])
						walks = append(walks, walks[l-2])
						continue
					}

					for i := 0; i<len(xkeys.UidThs); i++ {
						if xkeys.UidThs[i].Threshod >= maxrange {
							walks = append(walks, xkeys.UidThs[i].Uid)
							break
						}
					}
					if xkeys.UidThs[len(xkeys.UidThs)-1].Threshod < maxrange {
						walks = append(walks, xkeys.UidThs[len(xkeys.UidThs)-1].Uid)
					}
				} else {
					needReselect = true
				}
			}
		}

		//glog.Infof(" walks : %+v \n", walks)
		if nvlist, ok := nvn.Walks[tkey]; !ok {
			nvn.Walks[tkey] = &UidList{Uids: walks}
		} else {
			nvlist.Uids = append(nvlist.Uids, walks...)
		}
	}
}

func (nvn *Node2VecNbr) DeepWalk() {
	for r:=0; r<nvn.RVal; r++ {
		nvn.DeepWalkOnce()
    }
}

func (nvn *Node2VecNbr) ToSubGraph(temp *SubGraph) *SubGraph {
	sg := &SubGraph{}
	sg.ReadTs = temp.ReadTs
	sg.Cache = temp.Cache
	sg.Attr = "vec"
	sg.UnknownAttr = temp.UnknownAttr
	//sg.Params = temp.Params
	//sg.counts = temp.counts

	sg.uidMatrix = make([]*pb.List, 0)
	sg.SrcUIDs = &pb.List{Uids: make([]uint64, 0)}
	sg.DestUIDs = &pb.List{Uids: make([]uint64, 0)}
	sg.valueMatrix = make([]*pb.ValueList, 0)
	sg.facetsMatrix = make([]*pb.FacetsList, 0)

	for key, vecs := range nvn.Walks {
		sg.SrcUIDs.Uids = append(sg.SrcUIDs.Uids, key)
		sg.uidMatrix = append(sg.uidMatrix, &pb.List{})
		sg.facetsMatrix = append(sg.facetsMatrix, &pb.FacetsList{})
		vl := &pb.ValueList{ Values: make([]*pb.TaskValue, 0) }
		for _, uid := range vecs.Uids {
			tv := &pb.TaskValue{Val: []byte(fmt.Sprintf("%v",uid)), ValType: pb.Posting_STRING}
			vl.Values = append(vl.Values, tv)
		}
		sg.valueMatrix = append(sg.valueMatrix, vl)
	}

	sort.Slice(sg.SrcUIDs.Uids, func(i, j int) bool { return sg.SrcUIDs.Uids[i] < sg.SrcUIDs.Uids[j]; } )

	return sg
}

const (
	MAX_RANGE = 10000
)

func node2Vec(ctx context.Context, sg *SubGraph) ([]*SubGraph, error) {
	nvn := NewNode2VecNbr(sg.Params.Node2VecArgs.QVal, sg.Params.Node2VecArgs.PVal, 
		sg.Params.Node2VecArgs.LVal, sg.Params.Node2VecArgs.RVal)

	DisplaySubGraph(sg)

	var exec []*SubGraph
	var err error
	for _, child := range sg.Children {
		child.SrcUIDs = sg.DestUIDs
		exec = append(exec, child)
	}

	dummy := &SubGraph{}
	rrch := make(chan error, len(exec))
	for _, subgraph := range exec {
		go ProcessGraph(ctx, subgraph, dummy, rrch)
	}

	for range exec {
		select {
		case err = <-rrch:
			if err != nil {
				glog.Errorf("node2vec process query error: %#v \n", err)
				return nil, nil
			}
		case <-ctx.Done():
			//
		}
	}

	for _, subgraph := range exec {
		nvn.InitTransitionProbs(subgraph)

		//DisplaySubGraph(subgraph)
	}

	nvn.DeepWalk()

	//nvn.Display()

	outsg, err := nvn.ToSubGraph(sg), nil

	parentsg := &SubGraph{}
	parentsg.ReadTs = sg.ReadTs
	parentsg.Cache = sg.Cache
	parentsg.Attr = ""
	parentsg.UnknownAttr = sg.UnknownAttr
	parentsg.Params = sg.Params
	parentsg.Params.Alias = "node2vec"

	parentsg.DestUIDs = outsg.SrcUIDs
	parentsg.uidMatrix = make([]*pb.List, 0)
	parentsg.uidMatrix = append(parentsg.uidMatrix, outsg.SrcUIDs)

	//parentsg.valueMatrix = sg.valueMatrix 
	//parentsg.DestUIDs = sg.DestUIDs

	parentsg.Children = make([]*SubGraph, 0)
	parentsg.Children = append(parentsg.Children, outsg)

	uidsg := &SubGraph{}
	uidsg.ReadTs = sg.ReadTs
	uidsg.Cache = sg.Cache
	uidsg.Attr = "uid"
	uidsg.SrcUIDs = outsg.SrcUIDs
	uidsg.uidMatrix = make([]*pb.List, 0)
	for _, _ = range outsg.SrcUIDs.Uids {
		uidsg.uidMatrix = append(uidsg.uidMatrix, &pb.List{})
	}

	parentsg.Children = append(parentsg.Children, uidsg)

	//DisplaySubGraph(outsg)
	//DisplaySubGraph(uidsg)

	return []*SubGraph{parentsg}, nil
}
