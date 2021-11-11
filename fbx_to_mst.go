package asset3d

import (
	"os"
	"path/filepath"

	mst "github.com/flywave/go-mst"
	dvec3 "github.com/flywave/go3d/float64/vec3"
	"github.com/flywave/go3d/vec2"
	"github.com/flywave/go3d/vec3"
	fbx "github.com/flywave/ofbx"
)

type FbxToMst struct {
	baseDir string
	texId   int
}

func (cv *FbxToMst) Convert(path string) (*mst.Mesh, *[6]float64, error) {
	mesh := mst.NewMesh()
	bbx := dvec3.MinBox

	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	scene, er := fbx.Load(f)
	if er != nil {
		return nil, nil, er
	}
	cv.baseDir = filepath.Dir(path)
	isInstance := make(map[uint64]bool)
	instMp := make(map[uint64]*mst.InstanceMst)

	for _, mh := range scene.Meshes {
		if _, ok := isInstance[mh.ID()]; ok {
			isInstance[mh.ID()] = true
		} else {
			isInstance[mh.ID()] = false
		}
	}

	for _, mh := range scene.Meshes {
		meshId := mh.ID()
		if v := isInstance[meshId]; !v {
			bx := cv.convertMesh(mesh, mh)
			bbx.Join(bx)
		} else {
			var inst *mst.InstanceMst
			var ok bool
			if inst, ok = instMp[meshId]; !ok {
				bx := cv.convertMesh(mesh, mh)
				inst = &mst.InstanceMst{MeshNodeId: uint32(len(mesh.Nodes)), BBox: bx.Array()}
				instMp[meshId] = inst
			}
			mtx := fbx.GetGlobalMatrix(mh)
			inst.Transfors = append(inst.Transfors, arryToMat(mtx.ToArray()))
		}
	}
	insts := []*mst.InstanceMst{}
	for _, v := range instMp {
		insts = append(insts, v)
	}
	mesh.InstanceNode = insts
	return mesh, bbx.Array(), nil
}

func (cv *FbxToMst) convertMesh(mstMh *mst.Mesh, mh *fbx.Mesh) *dvec3.Box {
	mhNode := &mst.MeshNode{}
	bbx := dvec3.MinBox
	g := mh.Geometry
	for _, v := range g.Vertices {
		vt := vec3.T{float32(v[0]), float32(v[1]), float32(v[2])}
		mhNode.Vertices = append(mhNode.Vertices, vt)
		bbx.Extend((*dvec3.T)(&v))
	}
	if g.Normals != nil {
		for _, v := range g.Normals {
			mhNode.Normals = append(mhNode.Normals, vec3.T{float32(v[0]), float32(v[1]), float32(v[2])})
		}
	}
	if g.UVs[0] != nil {
		for _, v := range g.UVs[0] {
			mhNode.TexCoords = append(mhNode.TexCoords, vec2.T{float32(v[0]), float32(v[1])})
		}
	}

	oldV := g.GetOldVerts()
	batchs := g.Materials
	fgMap := make(map[int32]*mst.MeshTriangle)
	mtlMp := make(map[int]int32)
	for i := 0; i < len(batchs); i++ {
		batchId := batchs[i]
		bid, ok := mtlMp[batchId]
		var gp *mst.MeshTriangle
		if !ok {
			bid = int32(len(mstMh.Materials))
			mtlMp[batchId] = bid
			gp = &mst.MeshTriangle{Batchid: bid}
			fgMap[bid] = gp
			mhNode.FaceGroup = append(mhNode.FaceGroup, gp)
			cv.convertMaterial(mstMh, mh.Materials[batchId])
		} else {
			gp = fgMap[bid]
		}
		gp.Faces = append(gp.Faces, &mst.Face{Vertex: [3]uint32{uint32(oldV[i*3]), uint32(oldV[i*3+1]), uint32(oldV[i*3+2])}})
	}
	return &bbx
}

func (cv *FbxToMst) convertMaterial(mstMh *mst.Mesh, mt *fbx.Material) {
	mtl := &mst.PhongMaterial{}

	mtl.Color[0] = byte(mt.DiffuseColor.R * 255)
	mtl.Color[1] = byte(mt.DiffuseColor.G * 255)
	mtl.Color[2] = byte(mt.DiffuseColor.B * 255)

	mtl.Diffuse = mtl.Color

	mtl.Emissive[0] = byte(mt.EmissiveColor.R * 255)
	mtl.Emissive[1] = byte(mt.EmissiveColor.G * 255)
	mtl.Emissive[2] = byte(mt.EmissiveColor.B * 255)

	mtl.Ambient[0] = byte(mt.AmbientColor.R * 255)
	mtl.Ambient[1] = byte(mt.AmbientColor.G * 255)
	mtl.Ambient[2] = byte(mt.AmbientColor.B * 255)

	mtl.Specular[0] = byte(mt.SpecularColor.R * 255)
	mtl.Specular[1] = byte(mt.SpecularColor.G * 255)
	mtl.Specular[2] = byte(mt.SpecularColor.B * 255)

	mtl.Shininess = float32(mt.Shininess)
	mtl.Specularity = float32(mt.SpecularFactor)
	mstMh.Materials = append(mstMh.Materials, mtl)
	if len(mt.Textures) > 0 {
		_, fileName := filepath.Split(string(mt.Textures[0].GetFileName()))
		f := filepath.Join(cv.baseDir, fileName)
		tex, err := convertTex(f, cv.texId)
		if err != nil {
			return
		}
		mtl.Texture = tex
		cv.texId++
	}
}
