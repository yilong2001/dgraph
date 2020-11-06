/*
 * Copyright 2018 Dgraph Labs, Inc. and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package codec

import (
	"sort"

	"github.com/RoaringBitmap/roaring"
	"github.com/dgraph-io/dgraph/protos/pb"
	"github.com/dgraph-io/dgraph/x"
	"github.com/dgraph-io/ristretto/z"
)

type seekPos int

const (
	// SeekStart is used with Seek() to search relative to the Uid, returning it in the results.
	SeekStart seekPos = iota
	// SeekCurrent to Seek() a Uid using it as offset, not as part of the results.
	SeekCurrent
)

var (
	numMsb     uint8  = 48 // Number of most significant bits that are used as bases for UidBlocks
	msbBitMask uint64 = ((1 << numMsb) - 1) << (64 - numMsb)
	lsbBitMask uint64 = ^msbBitMask
)

func FreePack(pack *pb.UidPack) {
	if pack == nil {
		return
	}
	if pack.AllocRef == 0 {
		return
	}
	alloc := z.AllocatorFrom(pack.AllocRef)
	alloc.Release()
}

type ListMap struct {
	bitmaps map[uint64]*roaring.Bitmap
}

func NewListMap(pack *pb.UidPack) *ListMap {
	lm := &ListMap{
		bitmaps: make(map[uint64]*roaring.Bitmap),
	}
	if pack != nil {
		for _, block := range pack.Blocks {
			bitmap := roaring.New()
			x.Check2(bitmap.FromBuffer(block.Deltas))
			lm.bitmaps[block.Base] = bitmap
			x.AssertTrue(block.Base&lsbBitMask == 0)
		}
	}
	return lm
}

func (lm *ListMap) ToUids() []uint64 {
	lmi := lm.NewIterator()
	uids := make([]uint64, 64)
	var result []uint64
	for sz := lmi.Next(uids); sz > 0; {
		result = append(result, uids[:sz]...)
	}
	return result
}

func FromListXXX(list *pb.List) *ListMap {
	lm := NewListMap(nil)
	lm.AddMany(list.Uids)
	return lm
}

func (lm *ListMap) IsEmpty() bool {
	for _, bitmap := range lm.bitmaps {
		if !bitmap.IsEmpty() {
			return false
		}
	}
	return true
}

func (lm *ListMap) NumUids() uint64 {
	var result uint64
	for _, bitmap := range lm.bitmaps {
		result += bitmap.GetCardinality()
	}
	return result
}

type ListMapIterator struct {
	bases   []uint64
	bitmaps map[uint64]*roaring.Bitmap
	curIdx  int
	itr     roaring.ManyIntIterable
	many    []uint32
}

func (lm *ListMap) NewIterator() *ListMapIterator {
	lmi := &ListMapIterator{}
	for base := range lm.bitmaps {
		lmi.bases = append(lmi.bases, base)
	}
	sort.Slice(lmi.bases, func(i, j int) bool {
		return lmi.bases[i] < lmi.bases[j]
	})
	if len(lmi.bases) == 0 {
		return nil
	}
	base := lmi.bases[0]
	if bitmap, ok := lmi.bitmaps[base]; ok {
		lmi.itr = bitmap.ManyIterator()
	}
	return lmi
}

func (lmi *ListMapIterator) Next(uids []uint64) int {
	if lmi == nil {
		return 0
	}
	if lmi.curIdx >= len(lmi.bases) {
		return 0
	}

	// Adjust size of lmi.many.
	if len(uids) > cap(lmi.many) {
		lmi.many = make([]uint32, 0, len(uids))
	}
	lmi.many = lmi.many[:len(uids)]

	base := lmi.bases[lmi.curIdx]
	fill := func() int {
		if lmi.itr == nil {
			return 0
		}
		out := lmi.itr.NextMany(lmi.many)
		for i := 0; i < out; i++ {
			// NOTE that we can not set the uids slice via append, etc. That would not get reflected
			// back to the caller. All we can do is to set the internal elements of the given slice.
			uids[i] = base | uint64(lmi.many[i])
		}
		return out
	}

	for lmi.curIdx < len(lmi.bases) {
		if sz := fill(); sz > 0 {
			return sz
		}
		lmi.itr = nil
		lmi.curIdx++
		base = lmi.bases[lmi.curIdx]
		if bitmap, ok := lmi.bitmaps[base]; ok {
			lmi.itr = bitmap.ManyIterator()
		}
	}
	return 0
}

func (lm *ListMap) ToPack() *pb.UidPack {
	pack := &pb.UidPack{
		NumUids: lm.NumUids(),
	}
	for base, bitmap := range lm.bitmaps {
		data, err := bitmap.ToBytes()
		x.Check(err)
		block := &pb.UidBlock{
			Base:   base,
			Deltas: data,
		}
		pack.Blocks = append(pack.Blocks, block)
	}
	sort.Slice(pack.Blocks, func(i, j int) bool {
		return pack.Blocks[i].Base < pack.Blocks[j].Base
	})
	return pack
}

func (lm *ListMap) AddOne(uid uint64) {
	base := uid & msbBitMask
	bitmap, ok := lm.bitmaps[base]
	if !ok {
		bitmap = roaring.New()
		lm.bitmaps[base] = bitmap
	}
	bitmap.Add(uint32(uid & lsbBitMask))
}

func (lm *ListMap) RemoveOne(uid uint64) {
	base := uid & msbBitMask
	if bitmap, ok := lm.bitmaps[base]; ok {
		bitmap.Remove(uint32(uid & lsbBitMask))
	}
}

func (lm *ListMap) AddMany(uids []uint64) {
	for _, uid := range uids {
		lm.AddOne(uid)
	}
}

func PackOfOne(uid uint64) *pb.UidPack {
	lm := NewListMap(nil)
	lm.AddOne(uid)
	return lm.ToPack()
}

func (lm *ListMap) Add(block *pb.UidBlock) error {
	// TODO: Shouldn't we be adding this block in case it doesn't already exist?
	dst, ok := lm.bitmaps[block.Base]
	if !ok {
		return nil
	}
	src := roaring.New()
	if _, err := src.FromBuffer(block.Deltas); err != nil {
		return err
	}
	dst.Or(src)
	return nil
}

func (lm *ListMap) Intersect(a2 *ListMap) {
	if a2 == nil || len(a2.bitmaps) == 0 {
		// a2 might be empty. In that case, just ignore.
		return
	}
	for base, bitmap := range lm.bitmaps {
		if a2Map, ok := a2.bitmaps[base]; !ok {
			// a2 does not have this base. So, remove.
			delete(lm.bitmaps, base)
		} else {
			bitmap.And(a2Map)
		}
	}
}

func (lm *ListMap) Merge(a2 *ListMap) {
	if a2 == nil || len(a2.bitmaps) == 0 {
		// a2 might be empty. In that case, just ignore.
		return
	}
	for a2base, a2map := range a2.bitmaps {
		if bitmap, ok := lm.bitmaps[a2base]; ok {
			bitmap.Or(a2map)
		} else {
			// lm does not have this bitmap. So, add.
			lm.bitmaps[a2base] = a2map
		}
	}
}

func (lm *ListMap) RemoveBefore(uid uint64) {
	if uid == 0 {
		return
	}
	uidBase := uid & msbBitMask
	// Iteration is not in serial order. So, can't break early.
	for base, bitmap := range lm.bitmaps {
		if base < uidBase {
			delete(lm.bitmaps, base)
		} else if base == uidBase {
			bitmap.RemoveRange(0, uid&lsbBitMask)
		}
	}
}

func Encode(uids []uint64) *pb.UidPack {
	lm := NewListMap(nil)
	lm.AddMany(uids)
	return lm.ToPack()
}

func Decode(pack *pb.UidPack) []uint64 {
	lm := NewListMap(pack)
	return lm.ToUids()
}

// // ExactLen would calculate the total number of UIDs. Instead of using a UidPack, it accepts blocks,
// // so we can calculate the number of uids after a seek.
// func ExactLen(pack *pb.UidPack) int {
// 	if pack == nil {
// 		return 0
// 	}
// 	sz := len(pack.Blocks)
// 	if sz == 0 {
// 		return 0
// 	}
// 	num := 0
// 	for _, b := range pack.Blocks {
// 		num += int(b.NumUids) // NumUids includes the base UID.
// 	}
// 	return num
// }

// CopyUidPack creates a copy of the given UidPack.
func CopyUidPack(pack *pb.UidPack) *pb.UidPack {
	if pack == nil {
		return nil
	}

	packCopy := new(pb.UidPack)
	packCopy.BlockSize = pack.BlockSize
	packCopy.Blocks = make([]*pb.UidBlock, len(pack.Blocks))

	for i, block := range pack.Blocks {
		packCopy.Blocks[i] = new(pb.UidBlock)
		packCopy.Blocks[i].Base = block.Base
		packCopy.Blocks[i].NumUids = block.NumUids
		packCopy.Blocks[i].Deltas = make([]byte, len(block.Deltas))
		copy(packCopy.Blocks[i].Deltas, block.Deltas)
	}

	return packCopy
}
