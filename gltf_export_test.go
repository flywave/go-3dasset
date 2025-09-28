package asset3d

import (
	"testing"
)

func TestGltfExport(t *testing.T) {
	// 创建临时输出目录
	outputDir := "./test_output"

	// 创建导出器
	exporter := NewGltfExportToMst(outputDir)

	exporter.Export("./test/topside.glb")
}
