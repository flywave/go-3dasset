package asset3d

import (
	"path/filepath"

	tds "github.com/flywave/go-3ds"
	mst "github.com/flywave/go-mst"
	dmat "github.com/flywave/go3d/float64/mat4"
	quat "github.com/flywave/go3d/float64/quaternion"
	dvec3 "github.com/flywave/go3d/float64/vec3"
	dvec4 "github.com/flywave/go3d/float64/vec4"

	"github.com/flywave/go3d/vec2"
	"github.com/flywave/go3d/vec3"
)

type ThreeDsToMst struct {
	texId        int
	backup_texId int
	baseDir      string
}

func (cv *ThreeDsToMst) Convert(path string) (*mst.Mesh, *[6]float64, error) {
	mesh := mst.NewMesh()

	f := tds.OpenFile(path)
	mhs := f.GetMeshs()
	mtls := f.GetMaterials()

	nds := f.GetMeshInstanceNode()
	ndMap := make(map[string]*tds.MeshInstanceNode)
	for _, nd := range nds {
		ndMap[nd.InstanceName] = nd
	}
	cv.baseDir = filepath.Dir(path)
	ext := dvec3.MinBox
	instMp := make(map[string]*mst.InstanceMesh)

	for _, m := range mhs {
		instsNd, ok := ndMap[m.Name]
		if !ok {
			bx := cv.convert3dsMesh(&m, mesh, mtls)
			ext.Join(bx)
		} else {
			var inst *mst.InstanceMesh
			var ok bool
			if inst, ok = instMp[m.Name]; !ok {
				cv.backup_texId = cv.texId
				cv.texId = 0
				ins_mesh := mst.NewMesh()
				bx := cv.convert3dsMesh(&m, ins_mesh, mtls)
				inst = &mst.InstanceMesh{BBox: bx.Array(), Mesh: &ins_mesh.BaseMesh}
				instMp[m.Name] = inst
				cv.texId = cv.backup_texId
			}
			inst.Transfors = append(inst.Transfors, cv.toMat(instsNd))
		}
	}
	for _, ins := range instMp {
		mesh.InstanceNode = append(mesh.InstanceNode, ins)
	}

	return mesh, ext.Array(), nil
}

func (cv *ThreeDsToMst) convert3dsMesh(m *tds.Mesh, mstMesh *mst.Mesh, mtls []tds.Material) *dvec3.Box {
	ext := dvec3.MinBox
	nd := &mst.MeshNode{}
	mat := dmat.Ident
	for i, m := range m.Matrix {
		mat[i] = dvec4.T{float64(m[0]), float64(m[1]), float64(m[2]), float64(m[3])}
	}

	for _, v := range m.Vertices {
		vt := &dvec3.T{float64(v[0]), float64(v[1]), float64(v[2])}
		*vt = mat.MulVec3(vt)
		ext.Extend(vt)
		nd.Vertices = append(nd.Vertices, vec3.T{float32(vt[0]), float32(vt[1]), float32(vt[2])})
	}

	for _, v := range m.Texcos {
		nd.TexCoords = append(nd.TexCoords, vec2.T{v[0], v[1]})
	}

	tgMap := make(map[int32]*mst.MeshTriangle)
	for _, f := range m.Faces {
		tg, ok := tgMap[f.Material]
		if !ok {
			tg = &mst.MeshTriangle{Batchid: int32(len(mstMesh.Materials))}
			tgMap[f.Material] = tg
			nd.FaceGroup = append(nd.FaceGroup, tg)
			cv.convert3dsMtl(mstMesh, &mtls[f.Material])
		}
		tg.Faces = append(tg.Faces, &mst.Face{Vertex: [3]uint32{uint32(f.Index[0]), uint32(f.Index[1]), uint32(f.Index[2])}})
	}
	mstMesh.Nodes = append(mstMesh.Nodes, nd)
	return &ext
}

func (cv *ThreeDsToMst) convert3dsMtl(mesh *mst.Mesh, m *tds.Material) {
	texMtl := &mst.PhongMaterial{}
	mesh.Materials = append(mesh.Materials, texMtl)
	texMtl.Color[0] = byte(m.Diffuse[0] * 255)
	texMtl.Color[1] = byte(m.Diffuse[1] * 255)
	texMtl.Color[2] = byte(m.Diffuse[2] * 255)
	texMtl.Transparency = m.Transparency

	texMtl.Ambient[0] = byte(m.Ambient[0] * 255)
	texMtl.Ambient[1] = byte(m.Ambient[1] * 255)
	texMtl.Ambient[2] = byte(m.Ambient[2] * 255)

	texMtl.Specular[0] = byte(m.Specular[0] * 255)
	texMtl.Specular[1] = byte(m.Specular[1] * 255)
	texMtl.Specular[2] = byte(m.Specular[2] * 255)

	texMtl.Shininess = m.Shininess
	texPath := ""
	for i := range m.Texture1Map.Name {
		if m.Texture1Map.Name[i] == 0 {
			texPath = string(m.Texture1Map.Name[:i])
			break
		}
	}
	if texPath != "" {
		texPath = filepath.Join(cv.baseDir, texPath)
		t, err := convertTex(texPath, cv.texId)
		if err != nil {
			return
		}
		cv.texId++
		texMtl.Texture = t
	}
}

func (cv *ThreeDsToMst) toMat(nd *tds.MeshInstanceNode) *dmat.T {
	m := &dmat.T{}
	q := quat.FromVec4(&dvec4.T{float64(nd.Rot[0]), float64(nd.Rot[1]), float64(nd.Rot[2]), float64(nd.Rot[3])})
	t := &dvec3.T{float64(nd.Pos[0]), float64(nd.Pos[1]), float64(nd.Pos[2])}
	s := &dvec3.T{float64(nd.Scl[0]), float64(nd.Scl[1]), float64(nd.Scl[2])}
	m.AssignQuaternion(&q)
	m.ScaleVec3(s)
	m.Translate(t)
	return m
}
