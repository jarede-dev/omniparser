package edi

import (
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/jf-tech/go-corelib/jsons"

	"github.com/jf-tech/omniparser/extensions/omniv21/samples"
)

func Test1_CWR(t *testing.T) {
	cupaloy.SnapshotT(t, jsons.BPJ(samples.SampleTestCommon(
		t, "./cwr.schema.json", "./cwr.input.txt")))
}
