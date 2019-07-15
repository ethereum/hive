package common

import "errors"

var (
	// ErrNoSuchNode no node with the requested id
	ErrNoSuchNode = errors.New("no such node")

	// ErrNoSuchTestSuite is that the test suite does not exist
	ErrNoSuchTestSuite = errors.New("no such test suite context")

	// ErrMissingClientType is that the operation was expecting a CLIENT parameter
	ErrMissingClientType = errors.New("missing client type")

	// ErrNoAvailableClients is the error that no pre-supplied clients of the requested type are available
	ErrNoAvailableClients = errors.New("no available clients")

	// ErrTestSuiteRunning is that the test suite is still running and cannot be ended
	ErrTestSuiteRunning = errors.New("test suite still has running tests")

	// ErrMissingOutputWriter is that the test suite is missing an output stream writer
	ErrMissingOutputWriter = errors.New("test suite requires an output stream writer")
)
