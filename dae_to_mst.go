package asset3d

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	dae "github.com/flywave/go-collada"
	mst "github.com/flywave/go-mst"
	dmat "github.com/flywave/go3d/float64/mat4"
	dvec3 "github.com/flywave/go3d/float64/vec3"
	dvec4 "github.com/flywave/go3d/float64/vec4"
	"github.com/flywave/go3d/vec2"
	"github.com/flywave/go3d/vec3"
)

type DaeToMst struct {
	texId     int
	texMap    map[string]*mst.Texture
	mtlMap    map[string]*dae.Material
	effectMap map[string]*dae.Effect
	baseDir   string
}

func (cv *DaeToMst) Convert(path string) (*mst.Mesh, *[6]float64, error) {
	mesh := mst.NewMesh()
	var insts []*mst.InstanceMesh
	ext := dvec3.MinBox
	instMp := make(map[string]*mst.InstanceMesh)

	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}

	defer file.Close()
	collada, err := dae.LoadDocumentFromReader(file)
	if err != nil {
		return nil, nil, err
	}
	cv.baseDir = filepath.Dir(path)

	cv.texMap = make(map[string]*mst.Texture)
	for _, libimg := range collada.LibraryImages {
		for _, img := range libimg.Image {
			uri := img.InitFrom.Ref.Ref
			_, fn := filepath.Split(uri)
			fp := filepath.Join(cv.baseDir, fn)
			tex, err := convertTex(fp, cv.texId)
			if err != nil {
				continue
			}
			cv.texMap[string(img.HasId.Id)] = tex
			cv.texId++
		}
	}

	cv.mtlMap = make(map[string]*dae.Material)
	for _, m := range collada.LibraryMaterials {
		for _, mt := range m.Material {
			cv.mtlMap[string(mt.Id)] = mt
		}
	}

	cv.effectMap = make(map[string]*dae.Effect)
	for _, ef := range collada.LibraryEffects {
		for _, e := range ef.Effect {
			cv.effectMap[string(e.Id)] = e
		}
	}

	daeGeoMap := make(map[string]*dae.Geometry)
	for _, g := range collada.LibraryGeometries {
		for _, geo := range g.Geometry {
			daeGeoMap[string(geo.Id)] = geo
		}
	}

	isInstGeomey := make(map[string]bool)
	for _, sce := range collada.LibraryVisualScenes {
		for _, vs := range sce.VisualScene {
			for _, nd := range vs.Node {
				for _, instNd := range nd.InstanceNode {
					ndId := instNd.Url.GetId()
					for _, vs := range sce.VisualScene {
						for _, nd2 := range vs.Node {
							if string(nd2.Id) == ndId {
								geomats := getNodeTransform(nd2)
								for i, g := range nd2.InstanceGeometry {
									geoId := g.Url.GetId()
									isInstGeomey[geoId] = true
									var inst *mst.InstanceMesh
									var ok bool
									if inst, ok = instMp[g.Url.GetId()]; !ok {
										inst_mesh := mst.NewMesh()
										bbx := cv.convertMesh(daeGeoMap[geoId], inst_mesh, collada, geomats[i])
										inst = &mst.InstanceMesh{BBox: bbx.Array(), Mesh: &inst_mesh.BaseMesh}
										instMp[geoId] = inst
									}
									mats := getNodeTransform(nd)
									inst.Transfors = append(inst.Transfors, mats...)
								}
							}
						}
					}
				}
			}
		}
	}

	for _, sce := range collada.LibraryVisualScenes {
		for _, vs := range sce.VisualScene {
			for _, nd := range vs.Node {
				mats := getNodeTransform(nd)
				for i, g := range nd.InstanceGeometry {
					geoId := g.Url.GetId()
					if _, ok := isInstGeomey[geoId]; !ok {
						bx := cv.convertMesh(daeGeoMap[geoId], mesh, collada, mats[i])
						ext.Join(bx)
					}
				}
			}
		}
	}

	for _, ins := range instMp {
		insts = append(insts, ins)
	}
	mesh.InstanceNode = insts
	return mesh, ext.Array(), nil
}

func (cv *DaeToMst) convertMesh(geo *dae.Geometry, mstMesh *mst.Mesh, collada *dae.Collada, mat *dmat.T) *dvec3.Box {
	mstNd := &mst.MeshNode{}
	mh := geo.Mesh

	srcMap := make(map[string]*dae.Source)
	for _, src := range mh.Source {
		srcMap[string(src.Id)] = src
	}

	if len(mh.Polylist) > 0 {
		for _, p := range mh.Polylist {
			cv.parsePolyInuts(p, srcMap, mstNd, mstMesh)
			mtlId := p.Material
			cmtl, ok := cv.mtlMap[mtlId]
			if ok {
				cv.convertMtl(mstMesh, cmtl, collada)
			} else {
				cv.convertMtl(mstMesh, nil, collada)
			}
		}
	} else {
		var tgs []dae.Trig
		if mh.Triangles != nil {
			for _, t := range mh.Triangles {
				tgs = append(tgs, t)
			}
		} else if mh.Trifans != nil {
			for _, t := range mh.Trifans {
				tgs = append(tgs, t)
			}
		} else if mh.Tristrips != nil {
			for _, t := range mh.Tristrips {
				tgs = append(tgs, t)
			}
		}
		for _, p := range tgs {
			cv.parseTrgs(p, srcMap, mstNd, mstMesh)
			mtlId := p.GetMaterial()
			cmtl, ok := cv.mtlMap[mtlId]
			if ok {
				cv.convertMtl(mstMesh, cmtl, collada)
			} else {
				cv.convertMtl(mstMesh, nil, collada)
			}
		}
	}

	bbx := dvec3.MinBox
	for _, p := range mh.Vertices.Input {
		bx := cv.parseVectorInuts(mh, p, srcMap, mstNd, mat)
		bbx.Join(bx)
	}
	cv.rebuildFaceGrouops(mstNd)
	mstMesh.Nodes = append(mstMesh.Nodes, mstNd)
	return &bbx
}

func (cv *DaeToMst) convertMtl(mesh *mst.Mesh, mtl *dae.Material, collada *dae.Collada) {
	baseMtl := &mst.BaseMaterial{Color: [3]byte{255, 255, 255}}
	if mtl == nil {
		mesh.Materials = append(mesh.Materials, baseMtl)
		return
	}
	mt := &mst.TextureMaterial{}
	effect, ok := cv.effectMap[string(mtl.InstanceEffect.Url.GetId())]
	if !ok {
		mesh.Materials = append(mesh.Materials, baseMtl)
		return
	}
	common := effect.ProfileCommon
	if len(common.Newparam) > 0 {
		for _, param := range common.Newparam {
			if param.Semantic.Value == "DIFFUSECOLOR" {
				if param.Float3 != nil {
					sc := param.Float3.ToSlice()
					v, _ := strconv.ParseFloat(sc[0], 32)
					baseMtl.Color[0] = byte(v * 255)
					v, _ = strconv.ParseFloat(sc[1], 32)
					baseMtl.Color[1] = byte(v * 255)
					v, _ = strconv.ParseFloat(sc[2], 32)
					baseMtl.Color[2] = byte(v * 255)
				} else if param.Float4 != nil {
					sc := param.Float3.ToSlice()
					v, _ := strconv.ParseFloat(sc[0], 32)
					baseMtl.Color[0] = byte(v * 255)
					v, _ = strconv.ParseFloat(sc[1], 32)
					baseMtl.Color[1] = byte(v * 255)
					v, _ = strconv.ParseFloat(sc[2], 32)
					baseMtl.Color[2] = byte(v * 255)
					v, _ = strconv.ParseFloat(sc[3], 32)
					baseMtl.Transparency = 1 - float32(v)
				}
				mesh.Materials = append(mesh.Materials, baseMtl)
				return
			} else if param.Sampler2D != nil {
				img := param.Sampler2D.Source.Texture
				tex, ok := cv.texMap[img]
				if !ok {
					mesh.Materials = append(mesh.Materials, baseMtl)
					return
				}
				mt.Texture = tex
			}
		}
	} else if common.TechniqueFx != nil {
		phg := common.TechniqueFx.Phone
		if phg != nil {
			pmt := &mst.PhongMaterial{}
			if phg.Diffuse.Texture != nil {
				img := phg.Diffuse.Texture.Texture
				tex, ok := cv.texMap[img]
				if !ok {
					mesh.Materials = append(mesh.Materials, baseMtl)
					return
				}
				pmt.Texture = tex
			} else {
				if phg.Diffuse != nil && phg.Diffuse.Color != nil {
					cl := phg.Diffuse.Color.Float3.ToSlice()
					v1, _ := strconv.ParseFloat(cl[0], 32)
					v2, _ := strconv.ParseFloat(cl[1], 32)
					v3, _ := strconv.ParseFloat(cl[2], 32)
					pmt.Diffuse[0] = byte(v1 * 255)
					pmt.Diffuse[1] = byte(v2 * 255)
					pmt.Diffuse[2] = byte(v3 * 255)
					pmt.Color = pmt.Diffuse
				}
				if phg.Emission != nil && phg.Emission.Color != nil {
					cl := phg.Emission.Color.Float3.ToSlice()
					v1, _ := strconv.ParseFloat(cl[0], 32)
					v2, _ := strconv.ParseFloat(cl[1], 32)
					v3, _ := strconv.ParseFloat(cl[2], 32)
					pmt.Emissive[0] = byte(v1 * 255)
					pmt.Emissive[1] = byte(v2 * 255)
					pmt.Emissive[2] = byte(v3 * 255)
				}
				if phg.AmbientFx != nil && phg.AmbientFx.Color != nil {
					cl := phg.AmbientFx.Color.Float3.ToSlice()
					v1, _ := strconv.ParseFloat(cl[0], 32)
					v2, _ := strconv.ParseFloat(cl[1], 32)
					v3, _ := strconv.ParseFloat(cl[2], 32)
					pmt.Ambient[0] = byte(v1 * 255)
					pmt.Ambient[1] = byte(v2 * 255)
					pmt.Ambient[2] = byte(v3 * 255)
				}
				if phg.Specular != nil && phg.Specular.Color != nil {
					cl := phg.Specular.Color.Float3.ToSlice()
					v1, _ := strconv.ParseFloat(cl[0], 32)
					v2, _ := strconv.ParseFloat(cl[1], 32)
					v3, _ := strconv.ParseFloat(cl[2], 32)
					pmt.Specular[0] = byte(v1 * 255)
					pmt.Specular[1] = byte(v2 * 255)
					pmt.Specular[2] = byte(v3 * 255)
				}
				if phg.Shininess != nil && phg.Shininess.Float != nil {
					cl := phg.Shininess.Float.Value
					pmt.Shininess = float32(cl)
				}
				if phg.Transparency != nil && phg.Transparency.Float != nil {
					cl := phg.Transparency.Float.Value
					pmt.Transparency = 1 - float32(cl)
				}
			}
			mesh.Materials = append(mesh.Materials, pmt)
			return
		}
	}
	mesh.Materials = append(mesh.Materials, baseMtl)
}

func (cv *DaeToMst) parseVectorInuts(daeMh *dae.Mesh, input *dae.InputUnshared, srcMap map[string]*dae.Source, mstNd *mst.MeshNode, mat *dmat.T) *dvec3.Box {
	srcId := input.Source.GetId()
	src := srcMap[srcId]
	ay := src.FloatArray.ToSlice()
	stride := src.TechniqueCommon.Accessor.Stride
	vt := dvec3.T{}
	bbx := dvec3.MinBox
	for _, fg := range mstNd.FaceGroup {
		for _, f := range fg.Faces {
			for _, i := range f.Vertex {
				vt[0], _ = strconv.ParseFloat(ay[int(i)*stride], 64)
				vt[1], _ = strconv.ParseFloat(ay[int(i)*stride+1], 64)
				vt[2], _ = strconv.ParseFloat(ay[int(i)*stride+2], 64)
				if input.Semantic == "POSITION" {
					vt = mat.MulVec3(&vt)
					mstNd.Vertices = append(mstNd.Vertices, vec3.T{float32(vt[0]), float32(vt[1]), float32(vt[2])})
					bbx.Extend(&vt)
				} else if input.Semantic == "NORMAL" {
					mstNd.Normals = append(mstNd.Normals, vec3.T{float32(vt[0]), float32(vt[1]), float32(vt[2])})
				}
			}
		}
	}
	return &bbx
}

func (cv *DaeToMst) rebuildFaceGrouops(mstNd *mst.MeshNode) {
	var newTrgs []*mst.MeshTriangle
	var start int = 0
	for _, fg := range mstNd.FaceGroup {
		ng := &mst.MeshTriangle{}
		ng.Batchid = fg.Batchid
		for i := range fg.Faces {
			ng.Faces = append(ng.Faces, &mst.Face{Vertex: [3]uint32{uint32(i*3 + start), uint32(i*3 + 1 + start), uint32(i*3 + 2 + start)}})
		}
		start += len(fg.Faces) * 3
		newTrgs = append(newTrgs, ng)
	}
	mstNd.FaceGroup = newTrgs
}

func (cv *DaeToMst) parsePolyInuts(plist *dae.Polylist, srcMap map[string]*dae.Source, mstNd *mst.MeshNode, mstMesh *mst.Mesh) {
	var input *dae.InputShared
	for _, input = range plist.Input {
		st := input.Semantic
		if st == "VERTEX" {
			break
		}
	}

	offset := input.Offset
	fc := plist.VCount.ToSlice()
	idxs := plist.P.ToSlice()
	j := 0
	faceG := &mst.MeshTriangle{}
	faceG.Batchid = int32(len(mstMesh.Materials))
	inputCount := len(plist.Input)
	for i := 0; i < len(fc); i++ {
		count, _ := strconv.ParseInt(fc[i], 10, 32)
		fs := make([]int, count)
		for k := 0; k < int(count); k++ {
			v, _ := strconv.ParseInt(idxs[j+int(offset)], 10, 32)
			fs[k] = int(v)
			j += inputCount
		}
		if count == 3 {
			f := [3]uint32{}
			f[0] = uint32(fs[0])
			f[1] = uint32(fs[1])
			f[2] = uint32(fs[2])
			faceG.Faces = append(faceG.Faces, &mst.Face{Vertex: f})
		} else if count == 4 {
			f := [3]uint32{}
			f[0] = uint32(fs[0])
			f[1] = uint32(fs[1])
			f[2] = uint32(fs[2])
			faceG.Faces = append(faceG.Faces, &mst.Face{Vertex: f})
			f1 := [3]uint32{}
			f1[0] = uint32(fs[2])
			f1[1] = uint32(fs[3])
			f1[2] = uint32(fs[0])
			faceG.Faces = append(faceG.Faces, &mst.Face{Vertex: f})
		}
	}
	mstNd.FaceGroup = append(mstNd.FaceGroup, faceG)

	for _, ipt := range plist.Input {
		st := ipt.Semantic
		if st == "VERTEX" {
			continue
		} else if st == "NORMAL" {
			cv.parseNormalInuts(ipt, plist, srcMap, mstNd)
		} else if st == "TEXCOORD" {
			cv.parseTexCoordInuts(ipt, plist, srcMap, mstNd)
		}
	}
}

func (cv *DaeToMst) parseNormalInuts(input *dae.InputShared, plist *dae.Polylist, srcMap map[string]*dae.Source, mstNd *mst.MeshNode) {
	srcId := input.Source.GetId()
	src := srcMap[srcId]

	stride := src.TechniqueCommon.Accessor.Stride
	ay := src.FloatArray.ToSlice()

	offset := input.Offset
	fc := plist.VCount.ToSlice()
	idxs := plist.P.ToSlice()
	j := 0
	inputCount := len(plist.Input)
	vt := &vec3.T{}

	for i := 0; i < len(fc); i++ {
		count, _ := strconv.ParseInt(fc[i], 10, 32)
		fs := make([]int, count)
		for k := 0; k < int(count); k++ {
			v, _ := strconv.ParseInt(idxs[j+int(offset)], 10, 32)
			fs[k] = int(v)
			j += inputCount
		}
		for _, idx := range fs[:3] {
			pos := idx * stride
			x, _ := strconv.ParseFloat(ay[pos], 64)
			y, _ := strconv.ParseFloat(ay[pos+1], 64)
			z, _ := strconv.ParseFloat(ay[pos+2], 64)
			vt[0] = float32(x)
			vt[1] = float32(y)
			vt[2] = float32(z)
			mstNd.Normals = append(mstNd.Normals, *vt)
		}
		if count == 4 {
			f := [3]int{fs[2], fs[3], fs[0]}
			for _, idx := range f {
				pos := idx * stride
				x, _ := strconv.ParseFloat(ay[pos], 64)
				y, _ := strconv.ParseFloat(ay[pos+1], 64)
				z, _ := strconv.ParseFloat(ay[pos+2], 64)
				vt[0] = float32(x)
				vt[1] = float32(y)
				vt[2] = float32(z)
				mstNd.Normals = append(mstNd.Normals, *vt)
			}
		}
	}
}

func (cv *DaeToMst) parseTexCoordInuts(input *dae.InputShared, plist *dae.Polylist, srcMap map[string]*dae.Source, mstNd *mst.MeshNode) {
	srcId := input.Source.GetId()
	src := srcMap[srcId]

	stride := src.TechniqueCommon.Accessor.Stride
	ay := src.FloatArray.ToSlice()

	offset := input.Offset
	fc := plist.VCount.ToSlice()
	idxs := plist.P.ToSlice()
	j := 0
	inputCount := len(plist.Input)
	vt := &vec2.T{}

	for i := 0; i < len(fc); i++ {
		count, _ := strconv.ParseInt(fc[i], 10, 32)
		fs := make([]int, count)
		for k := 0; k < int(count); k++ {
			v, _ := strconv.ParseInt(idxs[j+int(offset)], 10, 32)
			fs[k] = int(v)
			j += inputCount
		}
		for _, idx := range fs[:3] {
			pos := idx * stride
			x, _ := strconv.ParseFloat(ay[pos], 64)
			y, _ := strconv.ParseFloat(ay[pos+1], 64)
			vt[0] = float32(x)
			vt[1] = float32(y)
			mstNd.TexCoords = append(mstNd.TexCoords, *vt)
		}
		if count == 4 {
			f := [3]int{fs[2], fs[3], fs[0]}
			for _, idx := range f {
				pos := idx * stride
				x, _ := strconv.ParseFloat(ay[pos], 64)
				y, _ := strconv.ParseFloat(ay[pos+1], 64)
				vt[0] = float32(x)
				vt[1] = float32(y)
				mstNd.TexCoords = append(mstNd.TexCoords, *vt)
			}
		}
	}
}

func (cv *DaeToMst) parseTrgs(trg dae.Trig, srcMap map[string]*dae.Source, mstNd *mst.MeshNode, mstMesh *mst.Mesh) {
	var input *dae.InputShared
	for _, input = range trg.GetSharedInput() {
		st := input.Semantic
		if st == "VERTEX" {
			break
		}
	}
	offset := input.Offset
	idxs := trg.GetP().ToSlice()
	faceG := &mst.MeshTriangle{}
	faceG.Batchid = int32(len(mstMesh.Materials))
	inputCount := len(trg.GetSharedInput())
	count := trg.GetCount()
	for k := 0; k < int(count); k++ {
		f := [3]uint32{}
		index := k * inputCount * 3
		v, _ := strconv.ParseInt(idxs[index+int(offset)], 10, 32)
		f[0] = uint32(v)
		index += inputCount
		v, _ = strconv.ParseInt(idxs[index+int(offset)], 10, 32)
		f[1] = uint32(v)
		index += inputCount
		v, _ = strconv.ParseInt(idxs[index+int(offset)], 10, 32)
		f[2] = uint32(v)
		faceG.Faces = append(faceG.Faces, &mst.Face{Vertex: f})
	}
	mstNd.FaceGroup = append(mstNd.FaceGroup, faceG)

	for _, ipt := range trg.GetSharedInput() {
		st := ipt.Semantic
		if st == "VERTEX" {
			continue
		} else if st == "NORMAL" {
			cv.parseTrgNormalInuts(ipt, trg, srcMap, mstNd)
		} else if st == "TEXCOORD" {
			cv.parseTrgTexCoordInuts(ipt, trg, srcMap, mstNd)
		}
	}

}

func (cv *DaeToMst) parseTrgNormalInuts(input *dae.InputShared, trg dae.Trig, srcMap map[string]*dae.Source, mstNd *mst.MeshNode) {
	srcId := input.Source.GetId()
	src := srcMap[srcId]

	offset := input.Offset
	stride := src.TechniqueCommon.Accessor.Stride

	ay := src.FloatArray.ToSlice()
	idxs := trg.GetP().ToSlice()
	vt := &vec3.T{}
	inputCount := len(trg.GetSharedInput())
	for i := 0; i < len(idxs); i += inputCount {
		tm, _ := strconv.ParseInt(idxs[i+int(offset)], 10, 32)
		idx := int(tm) * stride
		x, _ := strconv.ParseFloat(ay[idx], 64)
		y, _ := strconv.ParseFloat(ay[idx+1], 64)
		z, _ := strconv.ParseFloat(ay[idx+2], 64)
		vt[0] = float32(x)
		vt[1] = float32(y)
		vt[2] = float32(z)
		mstNd.Normals = append(mstNd.Normals, *vt)
	}
}

func (cv *DaeToMst) parseTrgTexCoordInuts(input *dae.InputShared, trg dae.Trig, srcMap map[string]*dae.Source, mstNd *mst.MeshNode) {
	srcId := input.Source.GetId()
	src := srcMap[srcId]

	offset := input.Offset
	stride := src.TechniqueCommon.Accessor.Stride

	ay := src.FloatArray.ToSlice()
	idxs := trg.GetP().ToSlice()
	vt := &vec2.T{}
	inputCount := len(trg.GetSharedInput())
	for i := 0; i < len(idxs); i += inputCount {
		tm, _ := strconv.ParseInt(idxs[i+int(offset)], 10, 32)
		idx := int(tm) * stride
		x, _ := strconv.ParseFloat(ay[idx], 64)
		y, _ := strconv.ParseFloat(ay[idx+1], 64)
		vt[0] = float32(x)
		vt[1] = float32(y)
		mstNd.TexCoords = append(mstNd.TexCoords, *vt)
	}
}

func getNodeTransform(nd *dae.Node) []*dmat.T {
	var mats []*dmat.T
	if len(nd.Matrix) > 0 {
		var ay [16]float64
		for _, m := range nd.Matrix {
			vs := m.ToSlice()
			for i, str := range vs {
				if i > 15 {
					break
				}
				s := strings.Trim(str, " ")
				ay[i], _ = strconv.ParseFloat(s, 64)
			}
			mat := arryToMat(ay)
			mat = mat.Transpose()
			mats = append(mats, mat)
		}
	} else {
		mt := &dmat.T{}
		for _, t := range nd.Rotate {
			vs := t.ToSlice()
			v := &dvec4.T{}
			for i, str := range vs {
				if i > 3 {
					break
				}
				s := strings.Trim(str, " ")
				v[i], _ = strconv.ParseFloat(s, 64)
			}

			if t.Sid == "rotationX" {
				mt.AssignXRotation(v[3])
			}
			if t.Sid == "rotationY" {
				mt.AssignYRotation(v[3])
			}
			if t.Sid == "rotationZ" {
				mt.AssignZRotation(v[3])
			}
		}
		scale := &dvec3.T{1, 1, 1}
		if len(nd.Scale) > 0 {
			t := nd.Scale[0]
			vs := t.ToSlice()
			for i, str := range vs {
				if i > 2 {
					break
				}
				st := strings.Trim(str, " ")
				scale[i], _ = strconv.ParseFloat(st, 64)
			}
		}
		mt.ScaleVec3(scale)
		for _, t := range nd.Translate {
			m := *mt
			vs := t.ToSlice()
			v := &dvec3.T{}
			for i, str := range vs {
				if i > 2 {
					break
				}
				s := strings.Trim(str, " ")
				v[i], _ = strconv.ParseFloat(s, 64)
			}
			m.Translate(v)
			mats = append(mats, &m)
		}
	}
	return mats
}

func arryToMat(mat [16]float64) *dmat.T {
	m := &dmat.T{}
	m[0] = dvec4.T{mat[0], mat[1], mat[2], mat[3]}
	m[1] = dvec4.T{mat[4], mat[5], mat[6], mat[7]}
	m[2] = dvec4.T{mat[8], mat[9], mat[10], mat[11]}
	m[3] = dvec4.T{mat[12], mat[13], mat[14], mat[15]}
	return m
}
