package asset3d

import (
	"image/color"

	fmesh "github.com/flywave/flywave-mesh"

	mst "github.com/flywave/go-mst"

	dvec3 "github.com/flywave/go3d/float64/vec3"
	"github.com/flywave/go3d/vec2"
	"github.com/flywave/go3d/vec3"
)

type ObjToMst struct {
}

func (obj *ObjToMst) Convert(path string) (*mst.Mesh, *[6]float64, error) {
	ext := dvec3.MinBox
	loader := &fmesh.ObjLoader{}
	er := loader.LoadMesh(path)
	if er != nil {
		return nil, nil, er
	}
	mesh := mst.NewMesh()
	meshNode := &mst.MeshNode{}

	var err error
	mtlGroup := make(map[uint32]int)
	gmap := make(map[uint32]int)
	gcount := len(loader.Materials)
	if gcount == 0 {
		gcount = 1
	}

	meshNode.FaceGroup = make([]*mst.MeshTriangle, gcount)

	if loader.Triangles != nil {
		for i := 0; i < len(loader.Triangles); i++ {
			t := &loader.Triangles[i]
			mtlId := t.Mtl
			if int32(mtlId) < 0 {
				mtlId = 0
			}
			mtg := meshNode.FaceGroup[mtlId]
			if mtg == nil {
				mtg = &mst.MeshTriangle{Batchid: int32(mtlId)}
				meshNode.FaceGroup[mtlId] = mtg
				mtlGroup[mtlId] = int(t.Tex)
			}
			obj.addTrigToMeshNode(mtg, t, meshNode, gmap, &ext)
		}
	} else if loader.Triarray != nil {
		for i := 0; i < int(loader.Triarray.Size()); i++ {
			t, er := loader.Triarray.GetTriangle(i)
			if er != nil {
				return nil, nil, er
			}
			mtlId := t.Mtl
			if int32(mtlId) < 0 {
				mtlId = 0
			}
			mtg := meshNode.FaceGroup[mtlId]
			if mtg == nil {
				mtg = &mst.MeshTriangle{Batchid: int32(mtlId)}
				meshNode.FaceGroup[mtlId] = mtg
				mtlGroup[mtlId] = int(t.Tex)
			}
			obj.addTrigToMeshNode(mtg, &t, meshNode, gmap, &ext)
		}
	}

	var ng []*mst.MeshTriangle
	for _, fg := range meshNode.FaceGroup {
		if fg != nil {
			ng = append(ng, fg)
		}
	}
	meshNode.FaceGroup = ng

	mesh.Nodes = append(mesh.Nodes, meshNode)
	if len(loader.Materials) > 0 {
		for i, mtl := range loader.Materials {
			texMtl := &mst.TextureMaterial{}
			texMtl.Color = mtl.Color
			texMtl.Transparency = 1 - mtl.Opacity
			texId := mtlGroup[uint32(i)]
			if mtl.Mode == fmesh.TEXTURE|fmesh.COLOR && int32(texId) >= 0 {
				var tex *fmesh.Texture
				if loader.Textures != nil {
					tex = loader.Textures[texId]
				} else {
					tex, err = loader.Texarray.GetTexture(texId)
					if err != nil {
						return nil, nil, err
					}
				}
				img := tex.Image
				bd := img.Bounds()
				buf := []byte{}
				for y := 0; y < bd.Dy(); y++ {
					for x := 0; x < bd.Dx(); x++ {
						cl := img.At(x, y)
						r, g, b, a := color.RGBAModel.Convert(cl).RGBA()
						buf = append(buf, byte(r), byte(g), byte(b), byte(a))
					}
				}

				t := &mst.Texture{}
				t.Id = int32(texId)
				t.Format = mst.TEXTURE_FORMAT_RGBA
				t.Size = [2]uint64{uint64(bd.Dx()), uint64(bd.Dy())}
				t.Compressed = mst.TEXTURE_COMPRESSED_ZLIB
				t.Data = mst.CompressImage(buf)
				t.Repeated = tex.Repeated()
				texMtl.Texture = t
			}

			switch mtl.Type {
			case fmesh.MTL_BASE:
				if texMtl.Texture == nil {
					mesh.Materials = append(mesh.Materials, &texMtl.BaseMaterial)
				} else {
					mesh.Materials = append(mesh.Materials, texMtl)
				}
			case fmesh.MTL_LAMBERT:
				mstMtl := toLambert(mtl)
				mstMtl.TextureMaterial = *texMtl
				mesh.Materials = append(mesh.Materials, mstMtl)
			case fmesh.MTL_PHONG:
				mstMtl := toPhone(mtl)
				mstMtl.TextureMaterial = *texMtl
				mesh.Materials = append(mesh.Materials, mstMtl)
			case fmesh.MTL_PBR:
				mstMtl := toPbr(mtl)
				mstMtl.TextureMaterial = *texMtl
				mesh.Materials = append(mesh.Materials, mstMtl)
			}
		}
	} else {
		mstMtl := &mst.BaseMaterial{}
		mstMtl.Color = [3]byte{255, 255, 255}
		mesh.Materials = append(mesh.Materials, mstMtl)
	}
	return mesh, ext.Array(), nil
}

func toLambert(mtl fmesh.Material) *mst.LambertMaterial {
	return &mst.LambertMaterial{
		Ambient:  mtl.Ambient,
		Diffuse:  mtl.Diffuse,
		Emissive: mtl.Emissive,
	}
}

func toPhone(mtl fmesh.Material) *mst.PhongMaterial {
	return &mst.PhongMaterial{
		LambertMaterial: *toLambert(mtl),
		Specular:        mtl.Specular,
		Shininess:       mtl.Shininess,
	}
}

func toPbr(mtl fmesh.Material) *mst.PbrMaterial {
	return &mst.PbrMaterial{
		Emissive:  mtl.Emissive,
		Metallic:  mtl.Metallic,
		Roughness: mtl.Roughness,
	}
}

func (obj *ObjToMst) addTrigToMeshNode(mrg *mst.MeshTriangle, trg *fmesh.Triangle, nd *mst.MeshNode, groupmap map[uint32]int, ext *dvec3.Box) {
	v0 := &trg.Vertices[0]
	v1 := &trg.Vertices[1]
	v2 := &trg.Vertices[2]
	ext.Extend(&dvec3.T{float64(v0.V[0]), float64(v0.V[1]), float64(v0.V[2])})
	ext.Extend(&dvec3.T{float64(v1.V[0]), float64(v1.V[1]), float64(v1.V[2])})
	ext.Extend(&dvec3.T{float64(v2.V[0]), float64(v2.V[1]), float64(v2.V[2])})
	mrg.Batchid = int32(trg.Mtl)

	mrg.Faces = append(mrg.Faces, &mst.Face{Vertex: [3]uint32{uint32(len(nd.Vertices)), uint32(len(nd.Vertices) + 1), uint32(len(nd.Vertices) + 2)}})
	nd.Vertices = append(nd.Vertices, (vec3.T)(v0.V), (vec3.T)(v1.V), (vec3.T)(v2.V))
	nd.TexCoords = append(nd.TexCoords, (vec2.T)(v0.T), (vec2.T)(v1.T), (vec2.T)(v2.T))
	nd.Normals = append(nd.Normals, (vec3.T)(v0.VN), (vec3.T)(v1.VN), (vec3.T)(v2.VN))
}
