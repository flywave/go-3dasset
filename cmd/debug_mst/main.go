package main

import (
	"fmt"
	mst "github.com/flywave/go-mst"
)

func main() {
	mesh, err := mst.MeshReadFrom("../../data/1/0131/Model/tiles.mst")
	if err != nil {
		panic(err)
	}
	
	fmt.Println("=== Node FaceGroup Batchid (fixed) ===")
	for i, node := range mesh.Nodes {
		for j, fg := range node.FaceGroup {
			fmt.Printf("Node %d, FaceGroup %d: Batchid=%d\n", i, j, fg.Batchid)
		}
	}
}
