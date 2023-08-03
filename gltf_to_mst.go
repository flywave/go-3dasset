package asset3d

import (
	"bytes"
	"encoding/binary"
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

	"github.com/qmuntal/gltf"
)

var (
	emptyMatrix = [16]float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
)

type GltfToMst struct {
	mtlMap        map[uint32]map[uint32]bool
	currentMeshId uint32
	nodeMatrix    map[uint32]*dmat.T
}

func (g *GltfToMst) Convert(path string) (*mst.Mesh, *[6]float64, error) {
	doc, err := gltf.Open(path)
	if err != nil {
		return nil, nil, err
	}
	g.nodeMatrix = make(map[uint32]*dmat.T)
	return g.ConvertFromDoc(doc)
}

func (g *GltfToMst) ConvertFromDoc(doc *gltf.Document) (*mst.Mesh, *[6]float64, error) {
	g.mtlMap = make(map[uint32]map[uint32]bool)
	mesh := mst.NewMesh()
	bbx := &[6]float64{}
	isInstance := make(map[uint32]bool)
	for _, nd := range doc.Nodes {
		if nd.Mesh == nil {
			continue
		}
		if _, ok := isInstance[*nd.Mesh]; ok {
			isInstance[*nd.Mesh] = true
		} else {
			isInstance[*nd.Mesh] = false
			g.nodeMatrix[*nd.Mesh] = toMat(nd)
		}
	}
	instMp := make(map[uint32]*mst.InstanceMesh)
	for _, nd := range doc.Nodes {
		if nd.Mesh == nil {
			continue
		}

		g.currentMeshId = *nd.Mesh
		if v := isInstance[g.currentMeshId]; !v {
			g.mtlMap[g.currentMeshId] = make(map[uint32]bool)
			bx := g.transMesh(doc, mesh, g.currentMeshId)
			addPoint(bbx, &[3]float64{bx[0], bx[1], bx[2]})
			addPoint(bbx, &[3]float64{bx[3], bx[4], bx[5]})
		} else {
			var inst *mst.InstanceMesh
			var ok bool
			if inst, ok = instMp[g.currentMeshId]; !ok {
				g.mtlMap[g.currentMeshId] = make(map[uint32]bool)
				instMh := mst.NewMesh()
				bx := g.transMesh(doc, instMh, g.currentMeshId)
				inst = &mst.InstanceMesh{BBox: bx, Mesh: &instMh.BaseMesh}
				instMp[g.currentMeshId] = inst

			}
			inst.Transfors = append(inst.Transfors, toMat(nd))
		}
	}
	for _, v := range instMp {
		mesh.InstanceNode = append(mesh.InstanceNode, v)
	}
	return mesh, bbx, nil
}

func (g *GltfToMst) transMesh(doc *gltf.Document, mstMh *mst.Mesh, mhid uint32) *[6]float64 {
	mh := doc.Meshes[mhid]
	accMap := make(map[uint32]bool)
	mhNode := &mst.MeshNode{}
	bbx := &[6]float64{}
	var faceBuff *gltf.Buffer
	var posBuff *gltf.Buffer
	var posView *gltf.BufferView
	var texBuff *gltf.Buffer
	var texView *gltf.BufferView
	var nlBuff *gltf.Buffer
	var nlView *gltf.BufferView
	for _, ps := range mh.Primitives {
		if ps.Indices == nil {
			continue
		}
		tg := &mst.MeshTriangle{}
		acc := doc.Accessors[int(*ps.Indices)]
		faceBuff = doc.Buffers[int(doc.BufferViews[int(*acc.BufferView)].Buffer)]
		tg.Faces = make([]*mst.Face, int(acc.Count/3))
		faceView := doc.BufferViews[int(*acc.BufferView)]
		start := int(faceView.ByteOffset + acc.ByteOffset)
		end := start + int(acc.Count*acc.ComponentType.ByteSize())
		buff := faceBuff.Data[start:end]
		bf := bytes.NewBuffer(buff)
		if acc.ComponentType == gltf.ComponentUshort {
			fcs := [3]uint16{}
			for i := 0; i < len(tg.Faces); i++ {
				f := &mst.Face{}
				binary.Read(bf, binary.LittleEndian, &fcs)
				f.Vertex[0] = uint32(fcs[0])
				f.Vertex[1] = uint32(fcs[1])
				f.Vertex[2] = uint32(fcs[2])
				tg.Faces[i] = f

			}
		} else if acc.ComponentType == gltf.ComponentUint {
			fcs := [3]uint32{}
			for i := 0; i < len(tg.Faces); i++ {
				binary.Read(bf, binary.LittleEndian, &fcs)
				f := &mst.Face{Vertex: fcs}
				tg.Faces[i] = f
			}
		}

		if idx, ok := ps.Attributes["POSITION"]; ok {
			if _, ok := accMap[idx]; !ok {
				mat, ok := g.nodeMatrix[mhid]
				acc = doc.Accessors[idx]
				posView = doc.BufferViews[int(*acc.BufferView)]
				posBuff = doc.Buffers[int(posView.Buffer)]
				bf := bytes.NewBuffer(posBuff.Data[int(posView.ByteOffset):int(posView.ByteOffset+posView.ByteLength)])
				for i := 0; i < int(acc.Count); i++ {
					v := vec3.T{}
					binary.Read(bf, binary.LittleEndian, &v)
					if ok {
						dv := dvec3.T{float64(v[0]), float64(v[1]), float64(v[2])}
						dv = mat.MulVec3(&dv)
						v = vec3.T{float32(dv[0]), float32(dv[1]), float32(dv[2])}
					}
					mhNode.Vertices = append(mhNode.Vertices, v)
					addPoint(bbx, &[3]float64{float64(v[0]), float64(v[1]), float64(v[2])})
				}
				accMap[idx] = true
			}
		}

		repete := false
		if idx, ok := ps.Attributes["TEXCOORD_0"]; ok {
			if _, ok := accMap[idx]; !ok {
				acc = doc.Accessors[idx]
				texView = doc.BufferViews[int(*acc.BufferView)]
				texBuff = doc.Buffers[int(texView.Buffer)]
				bf := bytes.NewBuffer(texBuff.Data[int(texView.ByteOffset):int(texView.ByteOffset+texView.ByteLength)])
				for i := 0; i < int(acc.Count); i++ {
					v := vec2.T{}
					binary.Read(bf, binary.LittleEndian, &v)
					mhNode.TexCoords = append(mhNode.TexCoords, v)
					repete = repete || v[0] > 1.1 || v[1] > 1.1
				}
				accMap[idx] = true
			}
		}

		if idx, ok := ps.Attributes["NORMAL"]; ok {
			if _, ok := accMap[idx]; !ok {
				acc = doc.Accessors[idx]
				nlView = doc.BufferViews[int(*acc.BufferView)]
				nlBuff = doc.Buffers[int(nlView.Buffer)]
				bf := bytes.NewBuffer(nlBuff.Data[int(nlView.ByteOffset):int(nlView.ByteOffset+nlView.ByteLength)])
				for i := 0; i < int(acc.Count); i++ {
					v := vec3.T{}
					binary.Read(bf, binary.LittleEndian, &v)
					mhNode.Normals = append(mhNode.Normals, v)
				}
				accMap[idx] = true
			}
		}
		mhNode.FaceGroup = append(mhNode.FaceGroup, tg)
		tg.Batchid = int32(len(mstMh.Materials))
		g.transMaterial(doc, mstMh, *ps.Material, repete)
	}

	if len(mhNode.FaceGroup) > 0 {
		mstMh.Nodes = append(mstMh.Nodes, mhNode)
	}

	return bbx
}

func (g *GltfToMst) transMaterial(doc *gltf.Document, mstMh *mst.Mesh, id uint32, repete bool) {
	if v, ok := g.mtlMap[g.currentMeshId][id]; ok && v {
		return
	}
	mt := doc.Materials[id]
	mtl := &mst.PbrMaterial{}
	mtl.Emissive[0] = byte(mt.EmissiveFactor[0] * 255)
	mtl.Emissive[1] = byte(mt.EmissiveFactor[0] * 255)
	mtl.Emissive[2] = byte(mt.EmissiveFactor[0] * 255)
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

	mstMh.Materials = append(mstMh.Materials, mtl)
	g.mtlMap[g.currentMeshId][id] = true
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

func toMat(nd *gltf.Node) *dmat.T {
	sc := dvec3.T{float64(nd.Scale[0]), float64(nd.Scale[1]), float64(nd.Scale[2])}
	tra := dvec3.T{float64(nd.Translation[0]), float64(nd.Translation[1]), float64(nd.Translation[2])}
	rot := quaternion.T{float64(nd.Rotation[0]), float64(nd.Rotation[1]), float64(nd.Rotation[2]), float64(nd.Rotation[3])}
	return dmat.Compose(&tra, &rot, &sc)
}

func addPoint(bx *[6]float64, p *[3]float64) {
	bx[0] = math.Min(bx[0], p[0])
	bx[1] = math.Min(bx[1], p[1])
	bx[2] = math.Min(bx[2], p[2])

	bx[3] = math.Max(bx[3], p[0])
	bx[4] = math.Max(bx[4], p[1])
	bx[5] = math.Max(bx[5], p[2])
}
