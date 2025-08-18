package asset3d

import (
	mst "github.com/flywave/go-mst"
)

type FormatConvert interface {
	Convert(path string) (*mst.Mesh, *[6]float64, error)
}

func FormatFactory(format string) FormatConvert {
	switch format {
	case THREEDS:
		return &ThreeDsToMst{}
	case DAE:
		return &DaeToMst{}
	case FBX:
		return &FbxToMst{}
	case GLTF, GLB:
		return &GltfToMst{}
	case OBJ:
		return &ObjToMst{}
	case TBIN:
		return &ThreejsBinToMst{}
	case STL:
		return &StlToMst{}
	}
	return nil
}
