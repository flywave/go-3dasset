package asset3d

import (
	"os"
	"path/filepath"

	mst "github.com/flywave/go-mst"
	"github.com/flywave/go3d/float64/mat4"
	dvec3 "github.com/flywave/go3d/float64/vec3"
	"github.com/flywave/go3d/vec2"
	"github.com/flywave/go3d/vec3"
	fbx "github.com/flywave/ofbx"
)

type FbxToMst struct {
	baseDir      string
	texId        int
	backup_texId int
	texMap       map[string]int32
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
	cv.texMap = map[string]int32{}
	cv.baseDir = filepath.Dir(path)
	isInstance := make(map[uint64]bool)
	instMp := make(map[uint64]*mst.InstanceMesh)

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
			var inst *mst.InstanceMesh
			var ok bool
			if inst, ok = instMp[meshId]; !ok {
				cv.backup_texId = cv.texId
				cv.texId = 0
				inst_mesh := mst.NewMesh()
				bx := cv.convertMesh(inst_mesh, mh)
				inst = &mst.InstanceMesh{BBox: bx.Array(), Mesh: &inst_mesh.BaseMesh}
				instMp[meshId] = inst
				cv.texId = cv.backup_texId
			}
			mtx := fbx.GetGlobalMatrix(mh)
			inst.Transfors = append(inst.Transfors, arryToMat(mtx.ToArray()))
		}
	}
	insts := []*mst.InstanceMesh{}
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
	mtx := fbx.GetGlobalMatrix(mh)

	repete := false
	batchs := g.Materials

	if g.UVs[0] != nil {
		for _, v := range g.UVs[0] {
			mhNode.TexCoords = append(mhNode.TexCoords, vec2.T{float32(v[0]), float32(v[1])})
			repete = repete || v[0] > 1.1 || v[1] > 1.1 || v[0] < 0 || v[1] < 0
		}
	}

	if g.Normals != nil {
		for _, v := range g.Normals {
			mhNode.Normals = append(mhNode.Normals, vec3.T{float32(v[0]), float32(v[1]), float32(v[2])})
		}
	}

	fgMap := make(map[int32]*mst.MeshTriangle)
	mtlMp := make(map[int]int32)
	if len(batchs) == 0 {
		batchs = make([]int, len(g.Faces))
	}

	for i := 0; i < len(batchs); i++ {
		batchId := batchs[i]
		bid, ok := mtlMp[batchId]
		var gp *mst.MeshTriangle

		if !ok {
			bid = cv.convertMaterial(mstMh, mh.Materials[batchId], repete)
			mtlMp[batchId] = bid
			gp = &mst.MeshTriangle{Batchid: bid}
			fgMap[bid] = gp
			mhNode.FaceGroup = append(mhNode.FaceGroup, gp)
		} else {
			gp = fgMap[bid]
		}
		for _, f := range g.Faces[i] {
			v := g.Vertices[f]
			vt := vec3.T{float32(v[0]), float32(v[1]), float32(v[2])}
			mt := mat4.FromArray(mtx.ToArray())
			dvt := mt.MulVec3(&dvec3.T{float64(vt[0]), float64(vt[1]), float64(vt[2])})
			mhNode.Vertices = append(mhNode.Vertices, vec3.T{float32(dvt[0]), float32(dvt[1]), float32(dvt[2])})
			bbx.Extend((*dvec3.T)(&v))
		}
		gp.Faces = append(gp.Faces, &mst.Face{Vertex: [3]uint32{uint32(i * 3), uint32(i*3 + 1), uint32(i*3 + 2)}})
	}

	mstMh.Nodes = append(mstMh.Nodes, mhNode)
	return &bbx
}

func (cv *FbxToMst) convertMaterial(mstMh *mst.Mesh, mt *fbx.Material, repete bool) int32 {
	mtl := &mst.PbrMaterial{Metallic: 0, Roughness: 1}
	idx := int32(len(mstMh.Materials))
	if mt.Textures[0] != nil {
		_, fileName := filepath.Split(string(mt.Textures[0].GetFileName()))
		// str := strings.ReplaceAll(mt.Textures[0].GetRelativeFileName().String(), "\\", "/")
		f := filepath.Join(cv.baseDir, fileName)

		if midx, ok := cv.texMap[f]; ok {
			return midx
		}

		tex, err := convertTex(f, cv.texId)
		if err != nil {
			return 0
		}
		tex.Repeated = repete
		mtl.Texture = tex
		cv.texId++
		cv.texMap[f] = idx
	}

	if mt.Textures[1] != nil {
		_, fileName := filepath.Split(string(mt.Textures[1].GetFileName()))
		f := filepath.Join(cv.baseDir, fileName)
		if midx, ok := cv.texMap[f]; ok {
			return midx
		}

		tex, err := convertTex(f, cv.texId)
		if err != nil {
			return 0
		}
		tex.Repeated = repete
		mtl.Normal = tex
		cv.texId++
		cv.texMap[f] = idx
	}

	mtl.Color[0] = byte(mt.DiffuseColor.R * 255)
	mtl.Color[1] = byte(mt.DiffuseColor.G * 255)
	mtl.Color[2] = byte(mt.DiffuseColor.B * 255)
	mstMh.Materials = append(mstMh.Materials, mtl)
	return idx
}
