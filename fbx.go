package asset3d

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	mst "github.com/flywave/go-mst"
	mat4d "github.com/flywave/go3d/float64/mat4"
	vec3d "github.com/flywave/go3d/float64/vec3"

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
	bbx := vec3d.MinBox

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

func (cv *FbxToMst) convertMesh(mstMh *mst.Mesh, mh *fbx.Mesh) *vec3d.Box {
	mhNode := &mst.MeshNode{}
	bbx := vec3d.MinBox
	g := mh.Geometry
	if strings.HasPrefix(mh.Name(), "Cylinder67") {
		fmt.Println(mh.Name())
	}
	mtx := fbx.GetGlobalMatrix(mh)
	mary := mtx.ToArray()
	matrix := mat4d.FromArray(mary)

	repete := false
	batchs := g.Materials

	if g.UVs[0] != nil {
		for _, v := range g.UVs[0] {
			mhNode.TexCoords = append(mhNode.TexCoords, vec2.T{float32(v[0]), float32(v[1])})
			repete = repete || v[0] > 1.1 || v[1] > 1.1 || v[0] < 0 || v[1] < 0
		}
	}

	fgMap := make(map[int32]*mst.MeshTriangle)
	mtlMp := make(map[int]int32)
	if len(batchs) == 0 {
		batchs = make([]int, len(g.Faces))
	}

	vertexOffset := 0
	for i := 0; i < len(batchs); i++ {
		batchId := batchs[i]
		bid, ok := mtlMp[batchId]
		var gp *mst.MeshTriangle
		var mt *fbx.Material

		if len(mh.Materials) > batchId {
			mt = mh.Materials[batchId]
		}
		if !ok {
			bid = cv.convertMaterial(mstMh, mt, repete)
			mtlMp[batchId] = bid
			gp = &mst.MeshTriangle{Batchid: bid}
			fgMap[bid] = gp
			mhNode.FaceGroup = append(mhNode.FaceGroup, gp)
		} else {
			gp = fgMap[bid]
		}

		face := g.Faces[i]
		count := len(face)

		newFaces := [][]int{face}
		if count != 3 {
			var tris [][]int
			switch count {
			case 4:
				pts := []*vec3d.T{}
				for _, f := range face {
					v := g.Vertices[f]
					pt := &vec3d.T{float64(v[0]), float64(v[1]), float64(v[2])}
					pts = append(pts, pt)
				}
				tris = quadToTriangles(face, pts)
			case 5:
				tris = pentagonToTriangles(face)
			}
			newFaces = tris
		}

		for _, fc := range newFaces {
			for _, f := range fc {
				vt := g.Vertices[f]
				dvt := matrix.MulVec3(&vec3d.T{float64(vt[0]), float64(vt[1]), float64(vt[2])})
				mhNode.Vertices = append(mhNode.Vertices, vec3.T{float32(dvt[0]), float32(dvt[1]), float32(dvt[2])})
				bbx.Extend((*vec3d.T)(&dvt))
			}
			baseIdx := uint32(vertexOffset)
			gp.Faces = append(gp.Faces, &mst.Face{
				Vertex: [3]uint32{baseIdx, baseIdx + 1, baseIdx + 2},
			})
			vertexOffset += 3
		}
	}

	mhNode.ReComputeNormal()
	mstMh.Nodes = append(mstMh.Nodes, mhNode)
	return &bbx
}

func pentagonToTriangles(pent []int) [][]int {
	return [][]int{
		{pent[0], pent[1], pent[2]}, // 三角形1
		{pent[0], pent[2], pent[4]}, // 三角形2
		{pent[2], pent[3], pent[4]}, // 三角形3
	}
}

func quadToTriangles(quad []int, vertices []*vec3d.T) [][]int {
	p0, p1, p2, p3 := vertices[0], vertices[1], vertices[2], vertices[3]

	// 计算对角线距离
	diag1 := distance(p0, p2)
	diag2 := distance(p1, p3)

	if diag1 <= diag2 {
		return [][]int{
			{quad[0], quad[1], quad[2]}, // 三角形1
			{quad[0], quad[2], quad[3]}, // 三角形2
		}
	} else {
		return [][]int{
			{quad[0], quad[1], quad[3]},
			{quad[1], quad[2], quad[3]},
		}
	}
}

// 计算两点间距离
func distance(a, b *vec3d.T) float64 {
	dx := a[0] - b[0]
	dy := a[2] - b[1]
	dz := a[2] - b[2]
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func (cv *FbxToMst) convertMaterial(mstMh *mst.Mesh, mt *fbx.Material, repete bool) int32 {
	idx := int32(len(mstMh.Materials))
	mtl := &mst.PbrMaterial{Metallic: 0, Roughness: 1}
	if mt != nil {
		if mt.Textures[0] != nil {
			str := strings.ReplaceAll(mt.Textures[0].GetRelativeFileName().String(), "\\", "/")
			_, fileName := filepath.Split(str)
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
			str := strings.ReplaceAll(mt.Textures[1].GetRelativeFileName().String(), "\\", "/")
			_, fileName := filepath.Split(str)
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

		cl := mt.EmissiveColor
		mtl.Emissive[0] = byte(cl.R * 255)
		mtl.Emissive[1] = byte(cl.G * 255)
		mtl.Emissive[2] = byte(cl.B * 255)

		cl = mt.DiffuseColor
		mtl.Color[0] = byte(cl.R * 255)
		mtl.Color[1] = byte(cl.G * 255)
		mtl.Color[2] = byte(cl.B * 255)
	} else {
		mtl.Color = [3]byte{255, 255, 255}
	}
	mstMh.Materials = append(mstMh.Materials, mtl)
	return idx
}

// Ensure FbxToMst implements FormatConvert interface
var _ FormatConvert = (*FbxToMst)(nil)
