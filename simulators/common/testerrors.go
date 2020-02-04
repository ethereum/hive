package common

import "errors"

var (
	// ErrNoSuchNode no node with the requested id
	ErrNoSuchNode = errors.New("no such node")

	// ErrNoSuchProviderType the provider type does not exist
	ErrNoSuchProviderType = errors.New("no such provider type")

	// ErrNoSuchTestSuite is that the test suite does not exist
	ErrNoSuchTestSuite = errors.New("no such test suite")

	// ErrNoSuchTestCase is that the test case does not exist
	ErrNoSuchTestCase = errors.New("no such test case")

	// ErrMissingClientType is that the operation was expecting a CLIENT parameter
	ErrMissingClientType = errors.New("missing client type")

	// ErrNoAvailableClients is the error that no pre-supplied clients of the requested type are available
	ErrNoAvailableClients = errors.New("no available clients")

	// ErrTestSuiteRunning is that the test suite is still running and cannot be ended
	ErrTestSuiteRunning = errors.New("test suite still has running tests")

	// ErrMissingOutputDestination is that the test suite is missing an output definition
	ErrMissingOutputDestination = errors.New("test suite requires an output")

	// ErrNoSummaryResult is that the mandatory test case summary result during test case completion is missing
	ErrNoSummaryResult = errors.New("test case must be ended with a summary result")

	// ErrDBUpdateFailed indicates that the output set could not be backed up or updated
	ErrDBUpdateFailed = errors.New("could not update results set")

	// ErrTestSuiteLimited warns that the testsuite max test count is limited
	ErrTestSuiteLimited = errors.New("testsuite test count is limited")
)
