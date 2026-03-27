package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	mst "github.com/flywave/go-mst"
)

func main() {
	input := flag.String("i", "", "input mst file path")
	output := flag.String("o", "", "output gltf/glb file path")
	flag.Parse()

	if *input == "" {
		log.Fatal("please specify input mst file with -i flag")
	}

	if *output == "" {
		ext := filepath.Ext(*input)
		*output = (*input)[:len(*input)-len(ext)] + ".glb"
	}

	err := ConvertMstToGltf(*input, *output)
	if err != nil {
		log.Fatalf("convert failed: %v", err)
	}

	fmt.Printf("Converted: %s -> %s\n", *input, *output)
}

func ConvertMstToGltf(inputPath, outputPath string) error {
	mesh, err := mst.MeshReadFrom(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read mst file: %w", err)
	}

	doc, err := mst.MstToGltf([]*mst.Mesh{mesh})
	if err != nil {
		return fmt.Errorf("failed to convert to gltf: %w", err)
	}

	data, err := mst.GetGltfBinary(doc, 4)
	if err != nil {
		return fmt.Errorf("failed to get binary: %w", err)
	}

	return os.WriteFile(outputPath, data, 0644)
}
