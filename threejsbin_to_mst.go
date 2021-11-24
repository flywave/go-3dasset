package asset3d

import (
	mst "github.com/flywave/go-mst"
	"github.com/flywave/go3d/float64/vec3"
)

type ThreejsBinToMst struct {
}

func (cv *ThreejsBinToMst) Convert(path string) (*mst.Mesh, *[6]float64, error) {
	mh, err := mst.ThreejsBin2Mst(path)
	if err != nil {
		return nil, nil, err
	}
	bx := getBBoxFromMst(mh)
	return mh, bx.Array(), nil
}

func getBBoxFromMst(mh *mst.Mesh) *vec3.Box {
	bbx := vec3.MinBox
	for _, nd := range mh.Nodes {
		bx := vec3.FromSlice(nd.GetBoundbox()[:])
		bbx.Join(bx)
	}
	return &bbx
}
