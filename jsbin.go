package asset3d

import (
	jsbin "github.com/flywave/go-3jsbin"
	mst "github.com/flywave/go-mst"
	vec3d "github.com/flywave/go3d/float64/vec3"
)

type ThreejsBinToMst struct {
}

func (cv *ThreejsBinToMst) Convert(path string) (*mst.Mesh, *[6]float64, error) {
	mh, err := jsbin.ThreejsBin2Mst(path)
	if err != nil {
		return nil, nil, err
	}
	bx := getBBoxFromMst(mh)
	return mh, bx.Array(), nil
}

func getBBoxFromMst(mh *mst.Mesh) *vec3d.Box {
	bbx := vec3d.MinBox
	for _, nd := range mh.Nodes {
		bx := vec3d.FromSlice(nd.GetBoundbox()[:])
		bbx.Join(bx)
	}
	return &bbx
}
