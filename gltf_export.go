package asset3d

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	mst "github.com/flywave/go-mst"
	mat4d "github.com/flywave/go3d/float64/mat4"
	quatd "github.com/flywave/go3d/float64/quaternion"
	vec3d "github.com/flywave/go3d/float64/vec3"

	"github.com/flywave/go3d/vec2"
	"github.com/flywave/go3d/vec3"

	"github.com/flywave/gltf"
)

// GltfExportToMst GLTF导出为多个MST文件的转换器
type GltfExportToMst struct {
	doc        *gltf.Document
	nodeMatrix map[uint32]*mat4d.T
	parentMap  map[uint32]uint32
	mtlMap     map[uint32]map[uint32]bool
	outputDir  string
	nodeNames  map[uint32]string // 存储节点名称用于构建路径

	// 新增字段用于记录树形结构
	nodeTree    *NodeTree            // 树形结构根节点
	nodeTreeMap map[uint32]*NodeTree // 节点索引到树节点的映射
}

// NodeTree 记录树形结构和叶子节点UUID
type NodeTree struct {
	Name     string      `json:"name"`
	UUID     string      `json:"uuid,omitempty"`
	Children []*NodeTree `json:"children,omitempty"`
}

// MeshProperties MST文件属性信息
type MeshProperties struct {
	Name  string `json:"name"`
	Group string `json:"group"`
	UUID  string `json:"uuid"`
}

// NewGltfExportToMst 创建新的GLTF导出器
func NewGltfExportToMst(outputDir string) *GltfExportToMst {
	return &GltfExportToMst{
		nodeMatrix:  make(map[uint32]*mat4d.T),
		parentMap:   make(map[uint32]uint32),
		mtlMap:      make(map[uint32]map[uint32]bool),
		outputDir:   outputDir,
		nodeNames:   make(map[uint32]string),
		nodeTreeMap: make(map[uint32]*NodeTree),
	}
}

// Export 将GLTF文件导出为多个MST文件
func (g *GltfExportToMst) Export(path string) error {
	doc, err := gltf.Open(path)
	if err != nil {
		return err
	}
	g.doc = doc

	// 确保输出目录存在
	if err := os.MkdirAll(g.outputDir, 0755); err != nil {
		return err
	}

	// 初始化根节点树
	g.nodeTree = &NodeTree{
		Name: "root",
	}

	// 递归处理场景中的根节点
	for _, i := range doc.Scenes[0].Nodes {
		err := g.processSceneNode(i, doc, "")
		if err != nil {
			return err
		}
	}

	// 导出完成后，写入树形结构文件
	treeFilePath := filepath.Join(g.outputDir, "tree.json")
	treeFile, err := os.Create(treeFilePath)
	if err != nil {
		return fmt.Errorf("创建树形结构文件失败 %s: %v", treeFilePath, err)
	}
	defer treeFile.Close()

	encoder := json.NewEncoder(treeFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(g.nodeTree); err != nil {
		return fmt.Errorf("写入树形结构文件失败 %s: %v", treeFilePath, err)
	}
	treeFile.Sync()

	return nil
}

// processSceneNode 递归处理场景节点
func (g *GltfExportToMst) processSceneNode(nodeIndex uint32, doc *gltf.Document, parentPath string) error {
	nd := doc.Nodes[nodeIndex]

	// 获取节点名称，如果没有名称则使用索引
	nodeName := nd.Name
	if nodeName == "" {
		nodeName = fmt.Sprintf("node_%d", nodeIndex)
	}

	// 构建当前节点的路径
	currentPath := parentPath
	if currentPath != "" {
		currentPath = filepath.Join(currentPath, nodeName)
	} else {
		currentPath = nodeName
	}

	// 存储节点名称用于后续引用
	g.nodeNames[nodeIndex] = nodeName

	// 创建树节点
	tree := &NodeTree{
		Name: nodeName,
	}
	g.nodeTreeMap[nodeIndex] = tree

	// 如果是根节点，添加到根树中
	if parentPath == "" {
		g.nodeTree.Children = append(g.nodeTree.Children, tree)
	} else {
		// 查找父节点并添加到其子节点中
		// 通过nodeNames和parentMap找到父节点
		if parentIdx, hasParent := g.parentMap[nodeIndex]; hasParent {
			if parentTree, exists := g.nodeTreeMap[parentIdx]; exists {
				parentTree.Children = append(parentTree.Children, tree)
			}
		}
	}

	// 如果节点有网格，导出网格
	if nd.Mesh != nil {
		err := g.exportNodeMesh(nodeIndex, nd, doc, currentPath, tree)
		if err != nil {
			return fmt.Errorf("导出节点%d失败: %v", nodeIndex, err)
		}
	}

	// 递归处理子节点
	for _, childIndex := range nd.Children {
		g.parentMap[childIndex] = nodeIndex
		err := g.processSceneNode(childIndex, doc, currentPath)
		if err != nil {
			return err
		}
	}

	return nil
}

// exportNodeMesh 导出单个节点的网格为MST文件
func (g *GltfExportToMst) exportNodeMesh(nodeIndex uint32, nd *gltf.Node, doc *gltf.Document, nodePath string, tree *NodeTree) error {
	meshId := *nd.Mesh
	mh := doc.Meshes[meshId]
	nodePath = strings.TrimPrefix(nodePath, "rvmparser-rotate-z-to-y/C:\\Users\\liteng\\Desktop\\TEST\\wang\\YH-TOPSIDE.RVM")

	// 创建MST网格
	mstMesh := mst.NewMesh()

	// 初始化材质映射
	if _, exists := g.mtlMap[meshId]; !exists {
		g.mtlMap[meshId] = make(map[uint32]bool)
	}

	// 获取节点的合成变换矩阵
	transform := g.getNodeMatrix(nodeIndex)

	// 处理网格的所有图元
	for _, ps := range mh.Primitives {
		if ps.Indices == nil {
			continue
		}

		// 创建网格节点
		mhNode := &mst.MeshNode{}

		// 处理索引
		tg := &mst.MeshTriangle{}
		acc := doc.Accessors[int(*ps.Indices)]
		var indices []uint32
		err := g.readDataByAccessor(doc, acc, func(res interface{}) {
			switch fcs := res.(type) {
			case *uint16:
				indices = append(indices, uint32(*fcs))
			case *uint32:
				indices = append(indices, *fcs)
			}
		})
		if err != nil {
			return err
		}

		// 添加面
		for i := 0; i < len(indices); i += 3 {
			f := &mst.Face{
				Vertex: [3]uint32{indices[i], indices[i+1], indices[i+2]},
			}
			tg.Faces = append(tg.Faces, f)
		}

		// 处理顶点位置
		if idx, ok := ps.Attributes["POSITION"]; ok {
			acc = doc.Accessors[idx]
			err := g.readDataByAccessor(doc, acc, func(res interface{}) {
				v := (*vec3.T)(res.(*[3]float32))
				// 应用变换矩阵到顶点
				if transform != nil {
					dv := vec3d.T{float64(v[0]), float64(v[1]), float64(v[2])}
					dv = transform.MulVec3(&dv)
					v = &vec3.T{float32(dv[0]), float32(dv[1]), float32(dv[2])}
				}
				mhNode.Vertices = append(mhNode.Vertices, *v)
			})
			if err != nil {
				return err
			}
		}

		// 处理纹理坐标
		repete := false
		if idx, ok := ps.Attributes["TEXCOORD_0"]; ok {
			acc = doc.Accessors[idx]
			err := g.readDataByAccessor(doc, acc, func(res interface{}) {
				v := (*vec2.T)(res.(*[2]float32))
				mhNode.TexCoords = append(mhNode.TexCoords, *v)
				repete = repete || v[0] > 1.1 || v[1] > 1.1
			})
			if err != nil {
				return err
			}
		}

		// 处理法线
		if idx, ok := ps.Attributes["NORMAL"]; ok {
			acc = doc.Accessors[idx]
			// 计算法线变换矩阵（使用变换矩阵的逆转置）
			normalMatrix := mat4d.T{}
			if transform != nil {
				// 创建单位矩阵
				normalMatrix = mat4d.Ident
				// TODO: 正确计算法线变换矩阵
			}

			err := g.readDataByAccessor(doc, acc, func(res interface{}) {
				v := (*vec3.T)(res.(*[3]float32))
				// 应用法线变换矩阵
				if transform != nil {
					dv := vec3d.T{float64(v[0]), float64(v[1]), float64(v[2])}
					dv = normalMatrix.MulVec3(&dv)
					// 标准化法线
					dv = *dv.Normalize()
					v = &vec3.T{float32(dv[0]), float32(dv[1]), float32(dv[2])}
				}
				mhNode.Normals = append(mhNode.Normals, *v)
			})
			if err != nil {
				return err
			}
		}

		// 添加面组
		mhNode.FaceGroup = append(mhNode.FaceGroup, tg)

		// 处理材质
		tg.Batchid = int32(len(mstMesh.Materials))
		g.transMaterial(doc, mstMesh, ps.Material, repete, nodePath)

		// 添加节点到网格
		mstMesh.Nodes = append(mstMesh.Nodes, mhNode)
	}

	// 生成UUID
	uuid, err := g.generateUUID()
	if err != nil {
		return fmt.Errorf("生成UUID失败: %v", err)
	}
	// 如果整个网格只包含一个图元，也生成一个包含整个网格的文件
	// 只使用当前节点名称作为文件名，而不是完整路径
	baseName := filepath.Base(nodePath)
	filename := fmt.Sprintf("%s.mst", uuid)
	outputFilePath := filepath.Join(g.outputDir, filename)

	// 确保目录存在
	dir := filepath.Dir(outputFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 保存MST文件
	if err := mst.MeshWriteTo(outputFilePath, mstMesh); err != nil {
		return fmt.Errorf("保存MST文件失败 %s: %v", outputFilePath, err)
	}

	// 生成并保存属性文件
	props := &MeshProperties{
		Name:  baseName,
		Group: filepath.Dir(nodePath), // 去掉nodename的部分
	}

	props.UUID = uuid

	// 设置树节点的UUID
	tree.UUID = uuid

	// 保存属性文件
	propsFilename := fmt.Sprintf("%s.json", uuid)
	propsOutputFilePath := filepath.Join(g.outputDir, propsFilename)

	propsFile, err := os.Create(propsOutputFilePath)
	if err != nil {
		return fmt.Errorf("创建属性文件失败 %s: %v", propsOutputFilePath, err)
	}
	defer propsFile.Close()

	encoder := json.NewEncoder(propsFile)
	encoder.SetIndent("", "  ") // 格式化输出
	if err := encoder.Encode(props); err != nil {
		return fmt.Errorf("写入属性文件失败 %s: %v", propsOutputFilePath, err)
	}

	return nil
}

// getNodeMatrix 获取节点的合成变换矩阵
func (g *GltfExportToMst) getNodeMatrix(idx uint32) *mat4d.T {
	// 检查是否已经计算过该节点的矩阵
	if matrix, exists := g.nodeMatrix[idx]; exists {
		return matrix
	}

	// 获取节点
	nd := g.doc.Nodes[idx]

	// 计算当前节点的变换矩阵
	matrix, err := g.toMat(*nd)
	if err != nil {
		// 如果计算失败，返回单位矩阵
		matrix = &mat4d.Ident
	}

	// 检查是否有父节点
	if parentIdx, hasParent := g.parentMap[idx]; hasParent {
		// 递归获取父节点的矩阵
		parentMatrix := g.getNodeMatrix(parentIdx)
		// 合成变换矩阵：父矩阵 * 当前矩阵
		resultMatrix := &mat4d.T{}
		resultMatrix.AssignMul(parentMatrix, matrix)
		g.nodeMatrix[idx] = resultMatrix
		return resultMatrix
	}

	// 没有父节点，直接返回当前矩阵
	g.nodeMatrix[idx] = matrix
	return matrix
}

// readDataByAccessor 读取访问器数据
func (g *GltfExportToMst) readDataByAccessor(doc *gltf.Document, acc *gltf.Accessor, process func(interface{})) error {
	bv := doc.BufferViews[*acc.BufferView]
	buffer := doc.Buffers[bv.Buffer]
	bf := bytes.NewBuffer(buffer.Data[int(bv.ByteOffset+acc.ByteOffset):int(bv.ByteOffset+bv.ByteLength)])

	var fcs interface{}
	switch acc.Type {
	case gltf.AccessorVec2:
		switch acc.ComponentType {
		case gltf.ComponentUshort:
			fcs = &[2]uint16{}
		case gltf.ComponentUint:
			fcs = &[2]uint32{}
		case gltf.ComponentFloat:
			fcs = &[2]float32{}
		}
	case gltf.AccessorVec3:
		switch acc.ComponentType {
		case gltf.ComponentUshort:
			fcs = &[3]uint16{}
		case gltf.ComponentUint:
			fcs = &[3]uint32{}
		case gltf.ComponentFloat:
			fcs = &[3]float32{}
		}
	case gltf.AccessorVec4:
		switch acc.ComponentType {
		case gltf.ComponentUshort:
			fcs = &[4]uint16{}
		case gltf.ComponentUint:
			fcs = &[4]uint32{}
		case gltf.ComponentFloat:
			fcs = &[4]float32{}
		}
	case gltf.AccessorScalar:
		switch acc.ComponentType {
		case gltf.ComponentUshort:
			n := uint16(0)
			fcs = &n
		case gltf.ComponentUint:
			n := uint32(0)
			fcs = &n
		case gltf.ComponentFloat:
			n := float32(0)
			fcs = &n
		}
	default:
		return errors.New("不支持的访问器类型")
	}

	for i := 0; i < int(acc.Count); i++ {
		binary.Read(bf, binary.LittleEndian, fcs)
		process(fcs)
	}
	return nil
}

// transMaterial 转换材质
func (g *GltfExportToMst) transMaterial(doc *gltf.Document, mstMh *mst.Mesh, idPtr *uint32, repete bool, nodePath string) {
	mtl := &mst.PbrMaterial{}
	// 使用根据区域映射的颜色
	r, gVal, b := g.getColorByArea(nodePath)
	mtl.Color[0] = r
	mtl.Color[1] = gVal
	mtl.Color[2] = b
	mtl.Transparency = 0

	if idPtr != nil {
		id := *idPtr

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
				// 忽略解码错误，使用默认材质
				fmt.Printf("警告: 解码纹理失败: %v\n", err)
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
				// 忽略解码错误，使用默认材质
				fmt.Printf("警告: 解码法线纹理失败: %v\n", err)
			}
			if tex != nil {
				tex.Id = int32(*texIdx)
				tex.Repeated = repete
				mtl.TextureMaterial.Normal = tex
			}
		}
	}
	mstMh.Materials = append(mstMh.Materials, mtl)
}

// decodeImage 解码图像
func (g *GltfExportToMst) decodeImage(mime string, rd io.Reader) (*mst.Texture, error) {
	var img image.Image
	var err error
	tex := &mst.Texture{}
	switch mime {
	case "image/png":
		img, err = png.Decode(rd)
	case "image/jpg", "image/jpeg":
		img, err = jpeg.Decode(rd)
	default:
		return nil, fmt.Errorf("不支持的图像类型: %s", mime)
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
	return nil, errors.New("图像解码失败")
}

// generateUUID 生成UUID字符串
func (g *GltfExportToMst) generateUUID() (string, error) {
	uuid := make([]byte, 16)
	_, err := rand.Read(uuid)
	if err != nil {
		return "", err
	}

	// 设置UUID版本和变体
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // 版本4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // 变体1

	// 格式化为UUID字符串
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16]), nil
}

// getColorByArea 根据区域名称映射颜色
func (g *GltfExportToMst) getColorByArea(nodePath string) (byte, byte, byte) {
	// 定义区域到颜色的映射关系
	areaColors := map[string][3]byte{
		"ARCH": {255, 0, 0},   // 红色
		"ELEC": {0, 255, 0},   // 绿色
		"EQUI": {0, 0, 255},   // 蓝色
		"HVAC": {255, 255, 0}, // 黄色
		"PIPE": {255, 0, 255}, // 品红
		"STRU": {0, 255, 255}, // 青色
	}

	// 从路径中提取区域信息
	// 路径格式类似于: YhSyz-All-Moudle/ARCH-AREA_A/...
	parts := strings.Split(nodePath, string(filepath.Separator))
	for _, part := range parts {
		if strings.Contains(part, "-AREA_") {
			// 提取前缀作为区域类型
			areaPrefix := strings.Split(part, "-")[0]
			if color, exists := areaColors[areaPrefix]; exists {
				return color[0], color[1], color[2]
			}
			break
		}
	}

	// 默认返回灰色
	return 128, 128, 128
}

// toMat 将GLTF节点转换为变换矩阵
func (g *GltfExportToMst) toMat(nd gltf.Node) (*mat4d.T, error) {
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
			err := g.readDataByAccessor(g.doc, g.doc.Accessors[tranAcc], func(res interface{}) {
				trans = res.(*[3]float32)
			})
			if err != nil {
				return nil, err
			}

			sclAcc := int(atr["SCALE"].(float64))
			err = g.readDataByAccessor(g.doc, g.doc.Accessors[sclAcc], func(res interface{}) {
				scl = res.(*[3]float32)
			})
			if err != nil {
				return nil, err
			}
			rotAcc := int(atr["ROTATION"].(float64))
			err = g.readDataByAccessor(g.doc, g.doc.Accessors[rotAcc], func(res interface{}) {
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

	sc := vec3d.T{float64(scl[0]), float64(scl[1]), float64(scl[2])}
	tra := vec3d.T{float64(trans[0]), float64(trans[1]), float64(trans[2])}
	rot := quatd.T{float64(rots[0]), float64(rots[1]), float64(rots[2]), float64(rots[3])}
	mt := mat4d.Compose(&tra, &rot, &sc)
	return mt, nil
}
