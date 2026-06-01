package asset3d

import (
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	mst "github.com/flywave/go-mst"
	osg "github.com/flywave/go-osg"
	"github.com/flywave/go-osg/model"
	pj "github.com/flywave/go-proj"
	vec3d "github.com/flywave/go3d/float64/vec3"
	"github.com/flywave/go3d/vec2"
	"github.com/flywave/go3d/vec3"
)

var _ FormatConvert = (*TilesOsgbToMst)(nil)

type TilesOsgbToMst struct {
	currentPath   string
	dataPath      string
	origin        [3]float64
	srs           string
	loadedFiles   map[string]bool
	ApplyOrigin   bool
	GeoRef        *mst.GeoRef
	GenerateNormals bool
}

func (t *TilesOsgbToMst) ConvertMultiple(path string) ([]*mst.Mesh, *[6]float64, error) {
	t.currentPath = path
	t.loadedFiles = make(map[string]bool)

	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}

	// Single .osgb file → extract its finest geometry directly
	if !info.IsDir() {
		if !strings.HasSuffix(strings.ToLower(path), ".osgb") {
			return nil, nil, fmt.Errorf("unsupported file format: %s", path)
		}
		var meshes []*mst.Mesh
		ext := vec3d.MinBox
		mesh := mst.NewMesh()
		t.loadFile(path, mesh, &ext)
		if len(mesh.Nodes) > 0 {
			meshes = append(meshes, mesh)
		}
		return meshes, ext.Array(), nil
	}

	// Directory input
	metadataPath := filepath.Join(path, "metadata.xml")
	t.parseMetadata(metadataPath)

	var meshes []*mst.Mesh
	ext := vec3d.MinBox

	// Strategy 1: Data/Tile_+xxx_+xxx/ structure (ContextCapture standard)
	dataDir := filepath.Join(path, "Data")
	if s, err := os.Stat(dataDir); err == nil && s.IsDir() {
		tileDirs, _ := filepath.Glob(filepath.Join(dataDir, "Tile_*"))
		for _, tileDir := range tileDirs {
			osgbFiles, _ := filepath.Glob(filepath.Join(tileDir, "*.osgb"))
			if len(osgbFiles) == 0 {
				continue
			}
			finest := findFinestLod(osgbFiles)
			if finest == "" {
				continue
			}
			mesh := mst.NewMesh()
			t.loadFile(finest, mesh, &ext)
			if len(mesh.Nodes) > 0 {
				if geoRef := t.computeGeoRef(); geoRef != nil {
					mesh.GeoRef = geoRef
				}
				meshes = append(meshes, mesh)
			}
		}
		if len(meshes) > 0 {
			return meshes, ext.Array(), nil
		}
	}

	// Strategy 2: OSGB files directly in root directory
	osgbFiles, _ := filepath.Glob(filepath.Join(path, "*.osgb"))
	if len(osgbFiles) > 0 {
		// Group by region (files without _L are coarser versions of same region)
		groups := make(map[string][]string)
		for _, f := range osgbFiles {
			base := filepath.Base(f)
			// Strip _L<level> suffix to group by region
			region := stripLODSuffix(base)
			groups[region] = append(groups[region], f)
		}
		for _, files := range groups {
			if len(files) == 0 {
				continue
			}
			finest := findFinestLod(files)
			if finest == "" {
				continue
			}
			mesh := mst.NewMesh()
			t.loadFile(finest, mesh, &ext)
			if len(mesh.Nodes) > 0 {
				if geoRef := t.computeGeoRef(); geoRef != nil {
					mesh.GeoRef = geoRef
				}
				meshes = append(meshes, mesh)
			}
		}
		return meshes, ext.Array(), nil
	}

	return meshes, ext.Array(), nil
}

func (t *TilesOsgbToMst) Convert(path string) (*mst.Mesh, *[6]float64, error) {
	meshes, ext, err := t.ConvertMultiple(path)
	if err != nil {
		return nil, ext, err
	}
	if len(meshes) == 0 {
		return nil, ext, nil
	}
	return meshes[0], ext, nil
}

// --- file discovery ---

func findFinestLod(files []string) string {
	if len(files) == 0 {
		return ""
	}
	best := files[0]
	bestLod := extractLodLevel(filepath.Base(best))
	for _, f := range files[1:] {
		if l := extractLodLevel(filepath.Base(f)); l > bestLod {
			bestLod = l
			best = f
		}
	}
	return best
}

func extractLodLevel(filename string) int {
	lod := 0
	for i := 0; i < len(filename); i++ {
		if filename[i] == 'L' && i+1 < len(filename) {
			for j := i + 1; j < len(filename) && filename[j] >= '0' && filename[j] <= '9'; j++ {
				lod = lod*10 + int(filename[j]-'0')
			}
			return lod
		}
	}
	return 0
}

func stripLODSuffix(filename string) string {
	for i := 0; i < len(filename); i++ {
		if filename[i] == 'L' && i+1 < len(filename) && filename[i+1] >= '0' && filename[i+1] <= '9' {
			// Check _L pattern
			if i > 0 && filename[i-1] == '_' {
				return filename[:i-1] + ".osgb"
			}
		}
	}
	return filename
}

// --- metadata ---

func (t *TilesOsgbToMst) parseMetadata(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var meta ModelMetadata
	if err := xml.Unmarshal(data, &meta); err != nil {
		return err
	}
	t.srs = meta.SRS
	if parts := strings.Split(meta.SRSOrigin, ","); len(parts) >= 3 {
		t.origin[0] = parseFloat(parts[0])
		t.origin[1] = parseFloat(parts[1])
		t.origin[2] = parseFloat(parts[2])
	}
	return nil
}

func (t *TilesOsgbToMst) computeGeoRef() *mst.GeoRef {
	if t.GeoRef != nil {
		return t.GeoRef
	}
	if t.srs == "" || t.srs == "unknown" {
		return nil
	}
	proj, err := pj.NewProj(t.srs)
	if err != nil {
		return nil
	}
	defer proj.Close()

	wgs84, err := pj.NewProj("+proj=longlat +datum=WGS84 +no_defs")
	if err != nil {
		return nil
	}
	defer wgs84.Close()

	lon, lat, err := pj.Transform2(proj, wgs84, t.origin[0], t.origin[1])
	if err != nil {
		return nil
	}
	ecefX, ecefY, ecefZ, err := pj.Lonlat2Ecef(lon, lat, t.origin[2])
	if err != nil {
		return nil
	}
	return &mst.GeoRef{
		EcefOrigin:   [3]float64{ecefX, ecefY, ecefZ},
		LatLonOrigin: [3]float64{lat, lon, t.origin[2]},
	}
}

// --- loading ---

func (t *TilesOsgbToMst) loadFile(osgbPath string, mesh *mst.Mesh, ext *vec3d.Box) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[ERROR] Recovered from panic: %v\n", r)
		}
	}()

	absPath, _ := filepath.Abs(osgbPath)
	if t.loadedFiles[absPath] {
		return
	}
	t.loadedFiles[absPath] = true

	rw := osg.NewReadWrite()
	res := rw.ReadNode(osgbPath, nil)
	if res == nil || res.GetNode() == nil {
		return
	}
	t.traverse(res.GetNode(), mesh, ext, filepath.Dir(osgbPath), nil)
}

func (t *TilesOsgbToMst) traverse(n interface{}, mesh *mst.Mesh, ext *vec3d.Box, baseDir string, matrix *[4][4]float32) {
	switch v := n.(type) {
	case *model.Geode:
		for _, c := range v.GetChildren() {
			if g, ok := c.(*model.Geometry); ok {
				t.processGeometry(g, mesh, ext, matrix)
			}
		}

	case *model.Group:
		for _, c := range v.GetChildren() {
			t.traverse(c, mesh, ext, baseDir, matrix)
		}

	case *model.MatrixTransform:
		combined := combineMatrix(matrix, &v.Matrix)
		for _, c := range v.GetChildren() {
			t.traverse(c, mesh, ext, baseDir, combined)
		}

	case *model.PositionAttitudeTransform:
		combined := combineMatrix(matrix, patToMatrix(v.Position, v.Attitude, v.Scale))
		for _, c := range v.GetChildren() {
			t.traverse(c, mesh, ext, baseDir, combined)
		}

	case *model.PagedLod:
		// Only process inline geometry (finest LOD level).
		// PagedLod file references point to lower-LOD child tiles;
		// skip them to extract only the highest-detail geometry.
		for _, c := range v.GetChildren() {
			t.traverse(c, mesh, ext, baseDir, matrix)
		}
	}
}

// --- geometry processing ---

func (t *TilesOsgbToMst) processGeometry(g *model.Geometry, mesh *mst.Mesh, ext *vec3d.Box, matrix *[4][4]float32) {
	if g.VertexArray == nil || g.VertexArray.Data == nil {
		return
	}
	positions := extractVec3s(g.VertexArray, t.ApplyOrigin, t.origin)
	if len(positions) == 0 {
		return
	}
	if matrix != nil {
		positions = applyMatrix(positions, matrix)
	}

	var uvs []vec2.T
	if len(g.TexCoordArrayList) > 0 && g.TexCoordArrayList[0] != nil && g.TexCoordArrayList[0].Data != nil {
		uvs = extractVec2s(g.TexCoordArrayList[0])
	}
	hasUV := len(uvs) > 0

	meshNode := &mst.MeshNode{}
	faceGroup := &mst.MeshTriangle{Batchid: int32(len(mesh.Materials))}

	for _, prim := range g.Primitives {
		t.processPrim(prim, positions, uvs, hasUV, meshNode, faceGroup, ext)
	}

	if len(faceGroup.Faces) == 0 {
		return
	}

	if t.GenerateNormals && len(meshNode.Vertices) > 0 && len(meshNode.Normals) == 0 {
		meshNode.Normals = computeNormals(meshNode.Vertices, faceGroup.Faces)
	}
	if len(meshNode.Normals) == 0 && len(meshNode.Vertices) > 0 {
		for range meshNode.Vertices {
			meshNode.Normals = append(meshNode.Normals, vec3.T{0, 0, 1})
		}
	}

	meshNode.FaceGroup = append(meshNode.FaceGroup, faceGroup)
	mesh.Nodes = append(mesh.Nodes, meshNode)
	mesh.Materials = append(mesh.Materials, &mst.BaseMaterial{Color: [3]byte{200, 200, 200}})
}

func (t *TilesOsgbToMst) processPrim(prim interface{}, positions []vec3.T, uvs []vec2.T, hasUV bool, node *mst.MeshNode, fg *mst.MeshTriangle, ext *vec3d.Box) {
	switch p := prim.(type) {
	case *model.DrawElementsUInt:
		if p.Data == nil {
			return
		}
		for i := 0; i+2 < len(p.Data); i += 3 {
			t.emitTriangle(int(p.Data[i]), int(p.Data[i+1]), int(p.Data[i+2]), positions, uvs, hasUV, node, fg, ext)
		}
	case *model.DrawElementsUShort:
		if p.Data == nil {
			return
		}
		for i := 0; i+2 < len(p.Data); i += 3 {
			t.emitTriangle(int(p.Data[i]), int(p.Data[i+1]), int(p.Data[i+2]), positions, uvs, hasUV, node, fg, ext)
		}
	case *model.DrawElementsUByte:
		if p.Data == nil {
			return
		}
		for i := 0; i+2 < len(p.Data); i += 3 {
			t.emitTriangle(int(p.Data[i]), int(p.Data[i+1]), int(p.Data[i+2]), positions, uvs, hasUV, node, fg, ext)
		}
	case *model.DrawArrays:
		if p.Count < 3 {
			return
		}
		for i := int(p.First); i+2 < int(p.First)+int(p.Count); i += 3 {
			t.emitTriangle(i, i+1, i+2, positions, uvs, hasUV, node, fg, ext)
		}
	}
}

func (t *TilesOsgbToMst) emitTriangle(i0, i1, i2 int, positions []vec3.T, uvs []vec2.T, hasUV bool, node *mst.MeshNode, fg *mst.MeshTriangle, ext *vec3d.Box) {
	if i0 >= len(positions) || i1 >= len(positions) || i2 >= len(positions) {
		return
	}
	base := uint32(len(node.Vertices))
	for _, vi := range []int{i0, i1, i2} {
		v := positions[vi]
		node.Vertices = append(node.Vertices, v)
		ext.Extend(&vec3d.T{float64(v[0]), float64(v[1]), float64(v[2])})
		if hasUV && vi < len(uvs) {
			node.TexCoords = append(node.TexCoords, uvs[vi])
		} else {
			node.TexCoords = append(node.TexCoords, vec2.T{0, 0})
		}
	}
	fg.Faces = append(fg.Faces, &mst.Face{Vertex: [3]uint32{base, base + 1, base + 2}})
}

// --- vertex extraction ---

func extractVec3s(arr *model.Array, applyOrigin bool, origin [3]float64) []vec3.T {
	if arr == nil || arr.Data == nil {
		return nil
	}
	switch data := arr.Data.(type) {
	case [][3]float32:
		out := make([]vec3.T, len(data))
		for i, v := range data {
			if applyOrigin {
				out[i] = vec3.T{v[0] + float32(origin[0]), v[1] + float32(origin[1]), v[2] + float32(origin[2])}
			} else {
				out[i] = vec3.T{v[0], v[1], v[2]}
			}
		}
		return out
	case []float32:
		n := len(data) / 3
		out := make([]vec3.T, n)
		for i := 0; i < n; i++ {
			if applyOrigin {
				out[i] = vec3.T{data[i*3] + float32(origin[0]), data[i*3+1] + float32(origin[1]), data[i*3+2] + float32(origin[2])}
			} else {
				out[i] = vec3.T{data[i*3], data[i*3+1], data[i*3+2]}
			}
		}
		return out
	case []int16:
		n := len(data) / 3
		out := make([]vec3.T, n)
		for i := 0; i < n; i++ {
			x, y, z := float64(data[i*3])/1000, float64(data[i*3+1])/1000, float64(data[i*3+2])/1000
			if applyOrigin {
				x += origin[0]; y += origin[1]; z += origin[2]
			}
			out[i] = vec3.T{float32(x), float32(y), float32(z)}
		}
		return out
	case []uint16:
		n := len(data) / 3
		out := make([]vec3.T, n)
		for i := 0; i < n; i++ {
			x, y, z := float64(data[i*3])/1000, float64(data[i*3+1])/1000, float64(data[i*3+2])/1000
			if applyOrigin {
				x += origin[0]; y += origin[1]; z += origin[2]
			}
			out[i] = vec3.T{float32(x), float32(y), float32(z)}
		}
		return out
	case []int32:
		n := len(data) / 3
		out := make([]vec3.T, n)
		for i := 0; i < n; i++ {
			x, y, z := float64(data[i*3])/1000, float64(data[i*3+1])/1000, float64(data[i*3+2])/1000
			if applyOrigin {
				x += origin[0]; y += origin[1]; z += origin[2]
			}
			out[i] = vec3.T{float32(x), float32(y), float32(z)}
		}
		return out
	}
	return nil
}

func extractVec2s(arr *model.Array) []vec2.T {
	if arr == nil || arr.Data == nil {
		return nil
	}
	switch data := arr.Data.(type) {
	case [][2]float32:
		out := make([]vec2.T, len(data))
		for i, v := range data {
			out[i] = vec2.T{v[0], v[1]}
		}
		return out
	case []float32:
		n := len(data) / 2
		out := make([]vec2.T, n)
		for i := 0; i < n; i++ {
			out[i] = vec2.T{data[i*2], data[i*2+1]}
		}
		return out
	case []int16:
		n := len(data) / 2
		out := make([]vec2.T, n)
		for i := 0; i < n; i++ {
			out[i] = vec2.T{float32(data[i*2]), float32(data[i*2+1])}
		}
		return out
	case []int32:
		n := len(data) / 2
		out := make([]vec2.T, n)
		for i := 0; i < n; i++ {
			out[i] = vec2.T{float32(data[i*2]), float32(data[i*2+1])}
		}
		return out
	}
	return nil
}

// --- matrix helpers ---

func combineMatrix(parent, child *[4][4]float32) *[4][4]float32 {
	if parent == nil {
		return child
	}
	if child == nil {
		return parent
	}
	var r [4][4]float32
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			for k := 0; k < 4; k++ {
				r[i][j] += parent[i][k] * child[k][j]
			}
		}
	}
	return &r
}

func patToMatrix(pos [3]float64, att [4]float64, scl [3]float64) *[4][4]float32 {
	var r [4][4]float32
	for i := 0; i < 4; i++ {
		r[i][i] = 1
	}
	if scl[0] == 0 || scl[1] == 0 || scl[2] == 0 || scl[0] > 1e100 {
		scl = [3]float64{1, 1, 1}
	}
	x, y, z, w := att[0], att[1], att[2], att[3]
	xx, yy, zz := x*x, y*y, z*z
	xy, xz, yz := x*y, x*z, y*z
	wx, wy, wz := w*x, w*y, w*z

	r[0][0] = float32(1 - 2*(yy+zz))
	r[0][1] = float32(2 * (xy + wz))
	r[0][2] = float32(2 * (xz - wy))
	r[1][0] = float32(2 * (xy - wz))
	r[1][1] = float32(1 - 2*(xx+zz))
	r[1][2] = float32(2 * (yz + wx))
	r[2][0] = float32(2 * (xz + wy))
	r[2][1] = float32(2 * (yz - wx))
	r[2][2] = float32(1 - 2*(xx+yy))

	r[0][0] *= float32(scl[0])
	r[0][1] *= float32(scl[0])
	r[0][2] *= float32(scl[0])
	r[1][0] *= float32(scl[1])
	r[1][1] *= float32(scl[1])
	r[1][2] *= float32(scl[1])
	r[2][0] *= float32(scl[2])
	r[2][1] *= float32(scl[2])
	r[2][2] *= float32(scl[2])

	r[3][0] = float32(pos[0])
	r[3][1] = float32(pos[1])
	r[3][2] = float32(pos[2])
	return &r
}

func applyMatrix(vs []vec3.T, m *[4][4]float32) []vec3.T {
	out := make([]vec3.T, len(vs))
	for i, v := range vs {
		x, y, z := float64(v[0]), float64(v[1]), float64(v[2])
		out[i] = vec3.T{
			float32(float64(m[0][0])*x + float64(m[1][0])*y + float64(m[2][0])*z + float64(m[3][0])),
			float32(float64(m[0][1])*x + float64(m[1][1])*y + float64(m[2][1])*z + float64(m[3][1])),
			float32(float64(m[0][2])*x + float64(m[1][2])*y + float64(m[2][2])*z + float64(m[3][2])),
		}
		w := float64(m[3][0])*x + float64(m[3][1])*y + float64(m[3][2])*z + float64(m[3][3])
		if w != 0 && w != 1 {
			out[i][0] /= float32(w)
			out[i][1] /= float32(w)
			out[i][2] /= float32(w)
		}
	}
	return out
}

// --- normal generation ---

func computeNormals(verts []vec3.T, faces []*mst.Face) []vec3.T {
	n := make([]vec3.T, len(verts))
	for _, f := range faces {
		i0, i1, i2 := int(f.Vertex[0]), int(f.Vertex[1]), int(f.Vertex[2])
		if i0 >= len(verts) || i1 >= len(verts) || i2 >= len(verts) {
			continue
		}
		a := vec3.Sub(&verts[i1], &verts[i0])
		b := vec3.Sub(&verts[i2], &verts[i0])
		fn := vec3.Cross(&a, &b)
		len2 := fn[0]*fn[0] + fn[1]*fn[1] + fn[2]*fn[2]
		if len2 < 1e-10 {
			continue
		}
		inv := 1.0 / float32(math.Sqrt(float64(len2)))
		fn[0] *= inv
		fn[1] *= inv
		fn[2] *= inv
		n[i0] = vec3.Add(&n[i0], &fn)
		n[i1] = vec3.Add(&n[i1], &fn)
		n[i2] = vec3.Add(&n[i2], &fn)
	}
	for i := range n {
		l := n[i][0]*n[i][0] + n[i][1]*n[i][1] + n[i][2]*n[i][2]
		if l < 1e-10 {
			n[i] = vec3.T{0, 0, 1}
			continue
		}
		inv := 1.0 / float32(math.Sqrt(float64(l)))
		n[i][0] *= inv
		n[i][1] *= inv
		n[i][2] *= inv
	}
	return n
}

// --- util ---
