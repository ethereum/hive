package helper_test

import (
	"testing"

	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
)

func TestBlobGeneration(t *testing.T) {
	for blobID := helper.BlobID(0); blobID < 100; blobID++ {
		blob, _, _, err := blobID.GenerateBlob()
		if err != nil {
			t.Fatal(err)
		}
		if len(blob) == 0 {
			t.Fatal("blob is empty")
		}
		if eq, err := blobID.VerifyBlob(blob); err != nil {
			t.Fatal(err)
		} else if !eq {
			t.Fatal("blob verification failed")
		}
	}
}

func TestPrecomputedKZGCommitments(t *testing.T) {
	for i := helper.BlobID(0); i < 100; i++ {
		commitment, proof := helper.GetPrecomputedKZG(i)
		if commitment == nil {
			t.Fatal("precomputed KZG commitments and proofs are empty")
		}
		blobID := helper.BlobID(i)
		_, kzgCommitment1, kzgProof1, err := blobID.GenerateBlobNoKZGCache()
		if err != nil {
			t.Fatal(err)
		}
		if *kzgCommitment1 != *commitment {
			t.Fatalf("commitment %d does not match precomputed commitment", i)
		}
		if *kzgProof1 != *proof {
			t.Fatalf("proof %d does not match precomputed proof", i)
		}
		_, kzgCommitment2, kzgProof2, err := blobID.GenerateBlob()
		if err != nil {
			t.Fatal(err)
		}
		if *kzgCommitment2 != *commitment {
			t.Fatalf("commitment %d does not match precomputed commitment", i)
		}
		if *kzgProof2 != *proof {
			t.Fatalf("proof %d does not match precomputed proof", i)
		}
	}
}
