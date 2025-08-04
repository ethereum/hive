package main

import (
	"os"
	"strconv"

	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
)

func main() {
	// First argument is the number of blobs to generate
	arg := os.Args[1]
	numBlobs, err := strconv.Atoi(arg)
	if err != nil {
		panic(err)
	}

	commitmentsArray := make([]byte, numBlobs*48)
	proofsArray := make([]byte, numBlobs*48)

	for i := 0; i < numBlobs; i++ {
		blobID := helper.BlobID(i)
		_, kzg, proof, err := blobID.GenerateBlobNoKZGCache()
		if err != nil {
			panic(err)
		}
		copy(commitmentsArray[i*48:], (*kzg)[:])
		copy(proofsArray[i*48:], (*proof)[:])
	}

	f, err := os.Create("./kzg_commitments.bin")
	if err != nil {
		panic(err)
	}
	f.Write(commitmentsArray)
	defer f.Close()

	f, err = os.Create("./kzg_proofs.bin")
	if err != nil {
		panic(err)
	}
	f.Write(proofsArray)
	defer f.Close()
}
