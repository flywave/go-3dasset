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
	mesh := &mst.Mesh{}
	meshNode := &mst.MeshNode{}

	var err error
	mtlGroup := make(map[uint32]int)
	gmap := make(map[uint32]int)

	for _, fg := range loader.FaceGroup {
		mtg := &mst.MeshTriangle{}
		first := fg[0]
		if loader.Triangles != nil {
			for i := 0; i < fg[1]; i++ {
				t := &loader.Triangles[first+i]
				obj.addTrigToMeshNode(mtg, t, meshNode, mtlGroup, gmap, &ext)
			}
		} else if loader.Triarray != nil {
			for i := 0; i < fg[1]; i++ {
				t, er := loader.Triarray.GetTriangle(first + i)
				if er != nil {
					return nil, nil, er
				}
				obj.addTrigToMeshNode(mtg, &t, meshNode, mtlGroup, gmap, &ext)
			}
		}
		meshNode.FaceGroup = append(meshNode.FaceGroup, mtg)
	}

	mesh.Nodes = append(mesh.Nodes, meshNode)
	if len(loader.Materials) > 0 {
		for i, mtl := range loader.Materials {
			texMtl := &mst.TextureMaterial{}
			texMtl.Color = mtl.Color
			texMtl.Transparency = 1 - mtl.Opacity
			if mtl.Mode == fmesh.TEXTURE|fmesh.COLOR {
				texMtl = &mst.TextureMaterial{}
				var tex *fmesh.Texture
				texId := mtlGroup[uint32(i)]
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
			if mtl.Type == fmesh.MTL_BASE {
				mesh.Materials = append(mesh.Materials, texMtl)
			} else if mtl.Type == fmesh.MTL_LAMBERT {
				mstMtl := &mst.LambertMaterial{}
				mstMtl.TextureMaterial = *texMtl
				mesh.Materials = append(mesh.Materials, mstMtl)
			} else if mtl.Type == fmesh.MTL_PHONG {
				mstMtl := &mst.PhongMaterial{}
				mstMtl.TextureMaterial = *texMtl
				mesh.Materials = append(mesh.Materials, mstMtl)
			} else if mtl.Type == fmesh.MTL_PBR {
				mstMtl := &mst.PbrMaterial{}
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

func (obj *ObjToMst) addTrigToMeshNode(mrg *mst.MeshTriangle, trg *fmesh.Triangle, nd *mst.MeshNode, mtlGroup map[uint32]int, groupmap map[uint32]int, ext *dvec3.Box) {
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
