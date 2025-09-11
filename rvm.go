package asset3d

import (
	"fmt"

	mst "github.com/flywave/go-mst"
	rvm "github.com/flywave/go-rvm"
)

// RvmToMst RVM到MST的转换器
type RvmToMst struct {
	// 可以添加一些配置选项
	options *RvmToMstOptions
}

// RvmToMstOptions RVM到MST转换器的选项
type RvmToMstOptions struct {
	CenterModel       bool // 是否居中模型
	RotateZToY        bool // 是否旋转Z轴到Y轴
	IncludeAttributes bool // 是否包含属性
	MergeGeometries   bool // 是否合并几何体
	Anchors           bool // 是否包含锚点
}

// NewRvmToMst 创建新的RVM到MST转换器
func NewRvmToMst() *RvmToMst {
	return &RvmToMst{
		options: &RvmToMstOptions{
			CenterModel:       false,
			RotateZToY:        false,
			IncludeAttributes: false,
			MergeGeometries:   false,
			Anchors:           true,
		},
	}
}

// NewRvmToMstWithOptions 创建带有选项的RVM到MST转换器
func NewRvmToMstWithOptions(options *RvmToMstOptions) *RvmToMst {
	return &RvmToMst{
		options: options,
	}
}

// Convert 将RVM文件转换为MST网格格式
func (cv *RvmToMst) Convert(inputFilename string) (*mst.Mesh, *[6]float64, error) {
	// 创建RVM存储
	store := rvm.NewStore()

	// 创建日志记录器
	logger := func(level int, format string, args ...interface{}) {
		// 可以根据需要处理日志
		// 目前我们只是简单地忽略日志
		_ = level
		_ = format
		_ = args
	}

	// 解析RVM文件
	parsed, err := rvm.ParseFile(store, logger, inputFilename)
	if err != nil {
		return nil, nil, fmt.Errorf("解析RVM文件失败: %v", err)
	}
	if !parsed {
		return nil, nil, fmt.Errorf("不支持的RVM文件格式: %s", inputFilename)
	}

	// 连接和对齐几何体
	rvm.Connect(store, logger, true)
	rvm.Align(store, logger)

	// 细分几何体（使用默认容差）
	tessellator := rvm.NewTessellator(logger, 0.1, -1, -1, 100)
	store.Apply(tessellator)

	// 创建MST导出器
	exporter := rvm.NewExportMST(logger)

	// 设置导出选项
	exporter.SetCenterModel(cv.options.CenterModel)
	exporter.SetRotateZToY(cv.options.RotateZToY)
	exporter.SetIncludeAttributes(cv.options.IncludeAttributes)
	exporter.SetMergeGeometries(cv.options.MergeGeometries)
	exporter.SetAnchors(cv.options.Anchors)
	exporter.SetPrimitiveBoundingBoxes(true) // 默认启用基本边界框

	// 初始化导出器
	exporter.Init(store)

	// 应用访问者模式导出几何体
	store.Apply(exporter)

	// 获取转换后的网格和边界框
	mesh := exporter.GetMesh()
	bbox := exporter.GetBoundingBox()

	return mesh, bbox, nil
}

// ConvertFromStore 直接从RVM存储转换
func (cv *RvmToMst) ConvertFromStore(store *rvm.Store) (*mst.Mesh, *[6]float64, error) {
	// 创建日志记录器
	logger := func(level int, format string, args ...interface{}) {
		// 可以根据需要处理日志
		_ = level
		_ = format
		_ = args
	}

	// 连接和对齐几何体
	rvm.Connect(store, logger, true)
	rvm.Align(store, logger)

	// 细分几何体（使用默认容差）
	tessellator := rvm.NewTessellator(logger, 0.1, -1, -1, 100)
	store.Apply(tessellator)

	// 创建MST导出器
	exporter := rvm.NewExportMST(logger)

	// 设置导出选项
	exporter.SetCenterModel(cv.options.CenterModel)
	exporter.SetRotateZToY(cv.options.RotateZToY)
	exporter.SetIncludeAttributes(cv.options.IncludeAttributes)
	exporter.SetMergeGeometries(cv.options.MergeGeometries)
	exporter.SetAnchors(cv.options.Anchors)
	exporter.SetPrimitiveBoundingBoxes(true) // 默认启用基本边界框

	// 初始化导出器
	exporter.Init(store)

	// 应用访问者模式导出几何体
	store.Apply(exporter)

	// 获取转换后的网格和边界框
	mesh := exporter.GetMesh()
	bbox := exporter.GetBoundingBox()

	return mesh, bbox, nil
}

// Ensure RvmToMst implements FormatConvert interface
var _ FormatConvert = (*RvmToMst)(nil)
