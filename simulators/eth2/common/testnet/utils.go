package testnet

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/ethereum/hive/simulators/eth2/common/clients"
)

// result object used to get a result/error from each node
type result struct {
	idx      int
	name     string
	msg      string
	err      error
	errCount int
	fatal    error
	maxErr   int
	done     bool
	result   interface{}
}

func (r *result) Clear() {
	r.msg = ""
	r.err = nil
	r.fatal = nil
	r.done = false
	r.result = nil
}

type resultsArr []*result

func (rs resultsArr) Clear() {
	for _, r := range rs {
		r.Clear()
	}
}

func (rs resultsArr) CheckError() error {
	for _, r := range rs {
		if r.fatal != nil {
			return errors.Wrap(
				r.fatal,
				fmt.Sprintf("node %d (%s)", r.idx, r.name),
			)
		} else if r.err != nil && r.errCount >= r.maxErr {
			return errors.Wrap(
				r.err,
				fmt.Sprintf("node %d (%s)", r.idx, r.name),
			)
		} else if r.err != nil {
			r.msg = fmt.Sprintf("WARN: node %d (%s): error %d/%d: %v", r.idx, r.name, r.errCount, r.maxErr, r.err)
			r.errCount++
		} else {
			r.errCount = 0
		}
	}
	return nil
}

func (rs resultsArr) PrintMessages(
	logf func(fmt string, values ...interface{}),
) {
	for _, r := range rs {
		if r.msg != "" {
			logf("node %d (%s): %s", r.idx, r.name, r.msg)
		}
	}
}

func (rs resultsArr) AllDone() bool {
	for _, r := range rs {
		if !r.done {
			return false
		}
	}
	return true
}

func makeResults(nodes clients.Nodes, maxErr int) resultsArr {
	res := make(resultsArr, len(nodes))
	for i, n := range nodes {
		r := result{
			idx:    i,
			name:   n.ClientNames(),
			maxErr: maxErr,
		}
		res[i] = &r
	}
	return res
}
