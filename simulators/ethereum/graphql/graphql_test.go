package main

import "testing"

func TestJsonComparison(t *testing.T) {

	a := `{
  "errors": [
    {
      "message": "Exception while fetching data (/block) : Block hash 0x123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0 was not found",
      "locations": [
        {
          "line": 1,
          "column": 2
        }
      ],
      "path": ["block"],
      "extensions": {
        "classification": "DataFetchingException"
      }
    }
  ],
  "data": null
}
`
	b := `
{
  "errors" : [ {
    "message" : "Exception while fetching data (/block) : Block hash 0x123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0 was not found",
    "extensions" : {
      "classification" : "DataFetchingException"
    },
    "path" : [ "block" ],
    "locations" : [ {
      "column" : 2,
      "line" : 1
    } ]
  } ],
  "data" : null
}`
	got, err := areEqualJSON(a, a)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatalf("Expected equality=true for identity, got false")
	}
	got, err = areEqualJSON(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatalf("Expected equality=true, got false")
	}
}
