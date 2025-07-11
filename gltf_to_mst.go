package asset3d

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math"

	mst "github.com/flywave/go-mst"
	dmat "github.com/flywave/go3d/float64/mat4"
	"github.com/flywave/go3d/float64/quaternion"
	dvec3 "github.com/flywave/go3d/float64/vec3"

	"github.com/flywave/go3d/vec2"
	"github.com/flywave/go3d/vec3"

	"github.com/flywave/gltf"
)

var (
	emptyMatrix = [16]float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
)

type GltfToMst struct {
	mtlMap        map[uint32]map[uint32]bool
	currentMeshId uint32
	nodeMatrix    map[uint32]*dmat.T
	parentMap     map[uint32]uint32
	doc           *gltf.Document
}

func (g *GltfToMst) Convert(path string) (*mst.Mesh, *[6]float64, error) {
	doc, err := gltf.Open(path)
	if err != nil {
		return nil, nil, err
	}
	g.doc = doc
	g.nodeMatrix = make(map[uint32]*dmat.T)
	g.parentMap = make(map[uint32]uint32)
	return g.ConvertFromDoc(doc)
}

func (g *GltfToMst) ConvertFromDoc(doc *gltf.Document) (*mst.Mesh, *[6]float64, error) {
	g.mtlMap = make(map[uint32]map[uint32]bool)
	mesh := mst.NewMesh()
	bbx := &[6]float64{}
	isInstance := make(map[uint32]bool)
	for _, i := range doc.Scenes[0].Nodes {
		err := g.processNode(i, doc, isInstance)
		if err != nil {
			return nil, nil, err
		}
	}

	instMp := make(map[uint32]*mst.InstanceMesh)
	for _, idx := range doc.Scenes[0].Nodes {
		nd := doc.Nodes[idx]
		if nd.Mesh != nil {
			v := isInstance[g.currentMeshId]
			g.currentMeshId = *nd.Mesh
			err := g.processMesh(doc, instMp, mesh, bbx, int(idx), v)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	for _, v := range instMp {
		mesh.InstanceNode = append(mesh.InstanceNode, v)
	}
	return mesh, bbx, nil
}

func (g *GltfToMst) processNode(i uint32, doc *gltf.Document, isInstance map[uint32]bool) error {
	nd := doc.Nodes[i]
	_, ok := g.nodeMatrix[uint32(i)]
	if ok {
		return nil
	}
	if nd.Mesh != nil {
		_, ok1 := isInstance[*nd.Mesh]
		if ok1 {
			isInstance[*nd.Mesh] = true
		} else {
			isInstance[*nd.Mesh] = false
		}
		var err error
		g.nodeMatrix[uint32(i)], err = g.toMat(nd)
		if err != nil {
			return err
		}
	} else if len(nd.Children) > 0 {
		var err error
		g.nodeMatrix[uint32(i)], err = g.toMat(nd)
		if err != nil {
			return err
		}
		for _, m := range nd.Children {
			g.parentMap[m] = uint32(i)
			g.processNode(m, doc, isInstance)
		}
	}
	return nil
}

func (g *GltfToMst) processMesh(doc *gltf.Document, instMp map[uint32]*mst.InstanceMesh, mesh *mst.Mesh, bbx *[6]float64, i int, isInstance bool) error {
	if !isInstance {
		g.mtlMap[g.currentMeshId] = make(map[uint32]bool)
		bx, err := g.transMesh(doc, mesh, g.currentMeshId, uint32(i))
		if err != nil {
			return err
		}
		addPoint(bbx, &[3]float64{bx[0], bx[1], bx[2]})
		addPoint(bbx, &[3]float64{bx[3], bx[4], bx[5]})
	} else {
		var inst *mst.InstanceMesh
		var ok bool
		if inst, ok = instMp[g.currentMeshId]; !ok {
			g.mtlMap[g.currentMeshId] = make(map[uint32]bool)
			instMh := mst.NewMesh()
			bx, err := g.transMesh(doc, instMh, g.currentMeshId, math.MaxUint32)
			if err != nil {
				return err
			}
			inst = &mst.InstanceMesh{BBox: bx, Mesh: &instMh.BaseMesh}
			instMp[g.currentMeshId] = inst

		}
		trans := g.getMatrix(uint32(i))
		inst.Transfors = append(inst.Transfors, trans)
	}

	for _, v := range instMp {
		mesh.InstanceNode = append(mesh.InstanceNode, v)
	}
	return nil
}

func (g *GltfToMst) transMesh(doc *gltf.Document, mstMh *mst.Mesh, mhid uint32, ndIndex uint32) (*[6]float64, error) {
	mh := doc.Meshes[mhid]
	accMap := make(map[uint32]bool)
	mhNode := &mst.MeshNode{}
	bbx := &[6]float64{}

	for _, ps := range mh.Primitives {
		if ps.Indices == nil {
			continue
		}
		tg := &mst.MeshTriangle{}
		acc := doc.Accessors[int(*ps.Indices)]
		var fv []uint32
		err := readDataByAccessor(doc, acc, func(res interface{}) {
			switch fcs := res.(type) {
			case *uint16:
				{
					fv = append(fv, uint32(*fcs))
				}
			case *uint32:
				{
					fv = append(fv, *fcs)
				}
			}
		})
		if err != nil {
			return nil, err
		}

		for i := 0; i < len(fv); i += 3 {
			f := &mst.Face{
				Vertex: [3]uint32{fv[i], fv[i+1], fv[i+2]},
			}
			tg.Faces = append(tg.Faces, f)
		}

		if idx, ok := ps.Attributes["POSITION"]; ok {
			if _, ok := accMap[idx]; !ok {
				acc = doc.Accessors[idx]
				mat := g.getMatrix(ndIndex)
				err := readDataByAccessor(doc, acc, func(res interface{}) {
					v := (*vec3.T)(res.(*[3]float32))
					if mat != nil {
						dv := dvec3.T{float64(v[0]), float64(v[1]), float64(v[2])}
						dv = mat.MulVec3(&dv)
						v = &vec3.T{float32(dv[0]), float32(dv[1]), float32(dv[2])}
					}
					mhNode.Vertices = append(mhNode.Vertices, *v)
					addPoint(bbx, &[3]float64{float64(v[0]), float64(v[1]), float64(v[2])})
				})
				if err != nil {
					return nil, err
				}
				accMap[idx] = true
			}
		}

		repete := false
		if idx, ok := ps.Attributes["TEXCOORD_0"]; ok {
			if _, ok := accMap[idx]; !ok {
				acc = doc.Accessors[idx]
				err := readDataByAccessor(doc, acc, func(res interface{}) {
					v := (*vec2.T)(res.(*[2]float32))
					mhNode.TexCoords = append(mhNode.TexCoords, *v)
					repete = repete || v[0] > 1.1 || v[1] > 1.1
				})
				if err != nil {
					return nil, err
				}
				accMap[idx] = true
			}
		}

		if idx, ok := ps.Attributes["NORMAL"]; ok {
			if _, ok := accMap[idx]; !ok {
				acc = doc.Accessors[idx]
				err := readDataByAccessor(doc, acc, func(res interface{}) {
					v := (*vec3.T)(res.(*[3]float32))
					mhNode.Normals = append(mhNode.Normals, *v)
				})
				if err != nil {
					return nil, err
				}
				accMap[idx] = true
			}
		}
		mhNode.FaceGroup = append(mhNode.FaceGroup, tg)
		tg.Batchid = int32(len(mstMh.Materials))
		g.transMaterial(doc, mstMh, ps.Material, repete)
	}

	if len(mhNode.FaceGroup) > 0 {
		mstMh.Nodes = append(mstMh.Nodes, mhNode)
	}

	return bbx, nil
}

func (g *GltfToMst) getMatrix(idx uint32) *dmat.T {
	i, ok := g.parentMap[idx]
	mat := dmat.Ident
	if ok {
		mat = *g.getMatrix(i)
	}

	mat2 := dmat.Ident
	mt := g.nodeMatrix[idx]
	if mt == nil {
		return &mat2
	}
	mat2.AssignMul(mt, &mat)
	return &mat2
}

func readDataByAccessor(doc *gltf.Document, acc *gltf.Accessor, procces func(interface{})) error {
	bv := doc.BufferViews[*acc.BufferView]
	buffer := doc.Buffers[bv.Buffer]
	bf := bytes.NewBuffer(buffer.Data[int(bv.ByteOffset+acc.ByteOffset):int(bv.ByteOffset+bv.ByteLength)])

	var fcs interface{}
	if acc.Type == gltf.AccessorVec2 {
		if acc.ComponentType == gltf.ComponentUshort {
			fcs = &[2]uint16{}
		} else if acc.ComponentType == gltf.ComponentUint {
			fcs = &[2]uint32{}
		} else if acc.ComponentType == gltf.ComponentFloat {
			fcs = &[2]float32{}
		}
	} else if acc.Type == gltf.AccessorVec3 {
		if acc.ComponentType == gltf.ComponentUshort {
			fcs = &[3]uint16{}
		} else if acc.ComponentType == gltf.ComponentUint {
			fcs = &[3]uint32{}
		} else if acc.ComponentType == gltf.ComponentFloat {
			fcs = &[3]float32{}
		}
	} else if acc.Type == gltf.AccessorVec4 {
		if acc.ComponentType == gltf.ComponentUshort {
			fcs = &[4]uint16{}
		} else if acc.ComponentType == gltf.ComponentUint {
			fcs = &[4]uint32{}
		} else if acc.ComponentType == gltf.ComponentFloat {
			fcs = &[4]float32{}
		}
	} else if acc.Type == gltf.AccessorScalar {
		if acc.ComponentType == gltf.ComponentUshort {
			n := uint16(0)
			fcs = &n
		} else if acc.ComponentType == gltf.ComponentUint {
			n := uint32(0)
			fcs = &n
		} else if acc.ComponentType == gltf.ComponentFloat {
			n := float32(0)
			fcs = &n
		}
	} else {
		return errors.New("acc have no type")
	}

	for i := 0; i < int(acc.Count); i++ {
		binary.Read(bf, binary.LittleEndian, fcs)
		procces(fcs)
	}
	return nil
}

func (g *GltfToMst) transMaterial(doc *gltf.Document, mstMh *mst.Mesh, idPtr *uint32, repete bool) {
	mtl := &mst.PbrMaterial{}
	mtl.Color[0] = 255
	mtl.Color[1] = 255
	mtl.Color[2] = 255
	mtl.Transparency = 0
	if idPtr != nil {
		id := *idPtr
		if v, ok := g.mtlMap[g.currentMeshId][id]; ok && v {
			return
		}
		mt := doc.Materials[id]
		if mt.PBRMetallicRoughness.BaseColorFactor != nil {
			mtl.Color[0] = byte(mt.PBRMetallicRoughness.BaseColorFactor[0] * 255)
			mtl.Color[1] = byte(mt.PBRMetallicRoughness.BaseColorFactor[1] * 255)
			mtl.Color[2] = byte(mt.PBRMetallicRoughness.BaseColorFactor[2] * 255)
			mtl.Transparency = 1 - float32(mt.PBRMetallicRoughness.BaseColorFactor[3])
		}
		if mt.PBRMetallicRoughness.MetallicFactor != nil {
			mtl.Metallic = float32(*mt.PBRMetallicRoughness.MetallicFactor)
		}
		if mt.PBRMetallicRoughness.RoughnessFactor != nil {
			mtl.Roughness = float32(*mt.PBRMetallicRoughness.RoughnessFactor)
		}
		if mt.PBRMetallicRoughness.BaseColorTexture != nil {
			texInfo := mt.PBRMetallicRoughness.BaseColorTexture
			texIdx := texInfo.Index
			src := *doc.Textures[int(texIdx)].Source
			img := doc.Images[int(src)]
			var tex *mst.Texture
			var buf io.Reader
			var err error
			if img.BufferView != nil {
				view := doc.BufferViews[int(*img.BufferView)]
				bufferIdx := view.Buffer
				buffer := doc.Buffers[int(bufferIdx)]
				bt := buffer.Data[view.ByteOffset : view.ByteOffset+view.ByteLength]
				buf = bytes.NewBuffer(bt)
			}
			tex, err = g.decodeImage(img.MimeType, buf)
			if err != nil {
				return
			}
			if tex != nil {
				tex.Id = int32(texIdx)
				tex.Repeated = repete
				mtl.TextureMaterial.Texture = tex
			}
		}

		if mt.NormalTexture != nil {
			norlTexInfo := mt.NormalTexture
			texIdx := norlTexInfo.Index
			src := *doc.Textures[int(*texIdx)].Source
			img := doc.Images[int(src)]
			var tex *mst.Texture
			var buf io.Reader
			var err error
			if img.BufferView != nil {
				view := doc.BufferViews[int(*img.BufferView)]
				bufferIdx := view.Buffer
				buffer := doc.Buffers[int(bufferIdx)]
				bt := buffer.Data[view.ByteOffset : view.ByteOffset+view.ByteLength]
				buf = bytes.NewBuffer(bt)
			}
			tex, err = g.decodeImage(img.MimeType, buf)
			if err != nil {
				return
			}
			if tex != nil {
				tex.Id = int32(*texIdx)
				tex.Repeated = repete
				mtl.TextureMaterial.Normal = tex
			}
		}

		g.mtlMap[g.currentMeshId][id] = true
	}
	mstMh.Materials = append(mstMh.Materials, mtl)
}

func (g *GltfToMst) decodeImage(mime string, rd io.Reader) (*mst.Texture, error) {
	var img image.Image
	var err error
	tex := &mst.Texture{}
	if mime == "image/png" {
		img, err = png.Decode(rd)
	} else if mime == "image/jpg" || mime == "image/jpeg" {
		img, err = jpeg.Decode(rd)
	}
	if err != nil {
		return nil, err
	}
	if img != nil {
		w := img.Bounds().Size().X
		h := img.Bounds().Size().Y
		tex.Size[0] = uint64(w)
		tex.Size[1] = uint64(h)
		var buf []byte
		for y := h - 1; y >= 0; y-- {
			for x := 0; x < w; x++ {
				cl := img.At(x, y)
				r, g, b, a := color.RGBAModel.Convert(cl).RGBA()
				buf = append(buf, byte(r), byte(g), byte(b), byte(a))
			}
		}
		tex.Format = mst.TEXTURE_FORMAT_RGBA
		tex.Compressed = mst.TEXTURE_COMPRESSED_ZLIB
		tex.Data = mst.CompressImage(buf)
		return tex, nil
	}
	return nil, errors.New("not support image type")
}

func (g *GltfToMst) toMat(nd *gltf.Node) (*dmat.T, error) {
	var trans *[3]float32
	var scl *[3]float32
	var rots *[4]float32

	if v, ok := nd.Extensions["EXT_mesh_gpu_instancing"]; ok {
		ins, ok := v.(map[string]interface{})
		if !ok {
			dt, _ := json.Marshal(v)
			ins = map[string]interface{}{}
			json.Unmarshal(dt, &ins)
		}
		if vv, ok := ins["attributes"]; ok {
			atr, ok := vv.(map[string]interface{})
			if !ok {
				dt, _ := json.Marshal(v)
				atr = map[string]interface{}{}
				json.Unmarshal(dt, &atr)
			}
			tranAcc := int(atr["TRANSLATION"].(float64))
			err := readDataByAccessor(g.doc, g.doc.Accessors[tranAcc], func(res interface{}) {
				trans = res.(*[3]float32)
			})
			if err != nil {
				return nil, err
			}

			sclAcc := int(atr["SCALE"].(float64))
			err = readDataByAccessor(g.doc, g.doc.Accessors[sclAcc], func(res interface{}) {
				scl = res.(*[3]float32)
			})
			if err != nil {
				return nil, err
			}
			rotAcc := int(atr["ROTATION"].(float64))
			err = readDataByAccessor(g.doc, g.doc.Accessors[rotAcc], func(res interface{}) {
				rots = res.(*[4]float32)
			})
			if err != nil {
				return nil, err
			}

		}
	} else {
		scl = &nd.Scale
		if scl[0] == 0 && scl[1] == 0 && scl[2] == 0 {
			scl = &[3]float32{1, 1, 1}
		}
		trans = &nd.Translation
		rots = &nd.Rotation
	}

	sc := dvec3.T{float64(scl[0]), float64(scl[1]), float64(scl[2])}
	tra := dvec3.T{float64(trans[0]), float64(trans[1]), float64(trans[2])}
	rot := quaternion.T{float64(rots[0]), float64(rots[1]), float64(rots[2]), float64(rots[3])}
	mt := dmat.Compose(&tra, &rot, &sc)
	return mt, nil
}

func addPoint(bx *[6]float64, p *[3]float64) {
	bx[0] = math.Min(bx[0], p[0])
	bx[1] = math.Min(bx[1], p[1])
	bx[2] = math.Min(bx[2], p[2])

	bx[3] = math.Max(bx[3], p[0])
	bx[4] = math.Max(bx[4], p[1])
	bx[5] = math.Max(bx[5], p[2])
}
