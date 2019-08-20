// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package layout

import (
	"math"

	"gonum.org/v1/gonum/spatial/r2"
)

const (
	openOrdRadius     = 10
	openOrdGridSize   = 1000
	openOrdView       = 4000
	openOrdViewToGrid = float64(openOrdGridSize) / float64(openOrdView)
)

type densityGrid struct {
	// The approach taken here is the apparently old
	// static allocation approach used by OpenOrd. The
	// current OpenOrd code dynamically allocates the
	// work spaces.
	//
	// TODO(kortschak): Revisit this.
	fallOff [openOrdRadius*2 + 1][openOrdRadius*2 + 1]float64
	density [openOrdGridSize][openOrdGridSize]float64
	bins    [openOrdGridSize][openOrdGridSize]openOrdQueue
}

func (g *densityGrid) init() {
	for i := -openOrdRadius; i <= openOrdRadius; i++ {
		for j := -openOrdRadius; j <= openOrdRadius; j++ {
			g.fallOff[i+openOrdRadius][j+openOrdRadius] = (openOrdRadius - math.Abs(float64(i))/openOrdRadius) * (openOrdRadius - math.Abs(float64(j))/openOrdRadius)
		}
	}
}

func (g *densityGrid) at(pos r2.Vec, fine bool) float64 {
	x := int((pos.X + openOrdView/2 + 0.5) * openOrdViewToGrid)
	y := int((pos.Y + openOrdView/2 + 0.5) * openOrdViewToGrid)

	const boundary = 10
	if y < boundary || openOrdGridSize-boundary < y {
		return 1e4
	}
	if x < boundary || openOrdGridSize-boundary < x {
		return 1e4
	}

	if !fine {
		d := g.density[y][x]
		return d * d
	}

	var d float64
	for i := y - 1; i <= y+1; i++ {
		for j := x - 1; j <= x+1; j++ {
			for _, r := range g.bins[i][j].slice() {
				v := pos.Sub(r.addPos)
				d = v.X*v.X + v.Y*v.Y
				d += 1e-4 / (d + 1e-50)
			}
		}
	}
	return d
}

func (g *densityGrid) add(n *openOrdNode, fine bool) {
	if fine {
		g.fineAdd(n)
	} else {
		g.coarseAdd(n)
	}
}

func (g *densityGrid) fineAdd(n *openOrdNode) {
	x := int((n.addPos.X + openOrdView/2 + 0.5) * openOrdViewToGrid)
	y := int((n.addPos.Y + openOrdView/2 + 0.5) * openOrdViewToGrid)
	n.subPos = n.addPos
	g.bins[y][x].enqueue(n)
}

func (g *densityGrid) coarseAdd(n *openOrdNode) {
	x := int((n.addPos.X+openOrdView/2+0.5)*openOrdViewToGrid) - openOrdRadius
	y := int((n.addPos.Y+openOrdView/2+0.5)*openOrdViewToGrid) - openOrdRadius
	if x < 0 || openOrdGridSize <= x {
		panic("openord: node outside grid")
	}
	if y < 0 || openOrdGridSize <= y {
		panic("openord: node outside grid")
	}
	n.subPos = n.addPos
	for i := 0; i <= openOrdRadius*2; i++ {
		for j := 0; j <= openOrdRadius*2; j++ {
			g.density[y+i][x+j] += g.fallOff[i][j]
		}
	}
}

func (g *densityGrid) sub(n *openOrdNode, firstAdd, fineFirstAdd, fine bool) {
	if fine && !fineFirstAdd {
		g.fineSub(n)
	} else if !firstAdd {
		g.coarseSub(n)
	}
}

func (g *densityGrid) fineSub(n *openOrdNode) {
	x := int((n.addPos.X + openOrdView/2 + 0.5) * openOrdViewToGrid)
	y := int((n.addPos.Y + openOrdView/2 + 0.5) * openOrdViewToGrid)
	g.bins[y][x].dequeue()
}

func (g *densityGrid) coarseSub(n *openOrdNode) {
	x := int((n.addPos.X+openOrdView/2+0.5)*openOrdViewToGrid) - openOrdRadius
	y := int((n.addPos.Y+openOrdView/2+0.5)*openOrdViewToGrid) - openOrdRadius
	for i := 0; i <= openOrdRadius*2; i++ {
		for j := 0; j <= openOrdRadius*2; j++ {
			g.density[y+i][x+j] -= g.fallOff[i][j]
		}
	}
}

type openOrdNode struct {
	id int64

	fixed bool

	addPos, subPos r2.Vec

	energy float64
}

// openOrdQueue implements a FIFO queue.
type openOrdQueue struct {
	head int
	data []*openOrdNode
}

// len returns the number of nodes in the queue.
func (q *openOrdQueue) len() int { return len(q.data) - q.head }

// enqueue adds the node n to the back of the queue.
func (q *openOrdQueue) enqueue(n *openOrdNode) {
	if len(q.data) == cap(q.data) && q.head > 0 {
		l := q.len()
		copy(q.data, q.data[q.head:])
		q.head = 0
		q.data = append(q.data[:l], n)
	} else {
		q.data = append(q.data, n)
	}
}

// dequeue returns the openOrdNode at the front of the queue and
// removes it from the queue.
func (q *openOrdQueue) dequeue() *openOrdNode {
	if q.len() == 0 {
		panic("queue: empty queue")
	}

	var n *openOrdNode
	n, q.data[q.head] = q.data[q.head], n
	q.head++

	if q.len() == 0 {
		q.reset()
	}

	return n
}

func (q *openOrdQueue) slice() []*openOrdNode {
	return q.data[q.head:]
}

// reset clears the queue for reuse.
func (q *openOrdQueue) reset() {
	q.head = 0
	q.data = q.data[:0]
}
