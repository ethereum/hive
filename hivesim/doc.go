/*
		Package hivesim is a Go wrapper for the Hive Simulation API.
		You can use this package to write simulations for Hive in Go.

		The hivesim API wrapper contains a few components that are important for interacting with the hive simulation
		API:
				- test suites
				- test cases
				- client(s)
				- networks (if the simulation calls for a more complex network topology


		Test Suites and Test Cases

		A test suite represents a single run of a simulator. A test suite can contain several test cases. Test cases
		represent an individual test against one or more clients.

		In order to execute a test against a client, it is necessary to create a test suite first and add one or more
		test cases to that suite. This can be done by creating `Suite` object, as such:

				suite := hivesim.Suite{
					Name:        	"MyTest",
					Description: 	"This simulation test does XYZ.",
				}

		The `Suite` has an additional field, `Tests`, which represents all the test cases to be executed by the test
		suite. Test cases can be added to the suite using the `Add()` method.

		A test case can be represented in either of the following formats:



				// TestSpec is the description of a test. Using this test type doesn't launch any clients by default.
				// To interact with clients, you can launch them using the t.Client method:
				//
				//    c := t.Client()
				//    c.RPC().Call(...)
				//
				// or run a subtest using t.RunClientTest():
				//
				//    t.RunClientTest(hivesim.ClientTestSpec{...})
				//
				type TestSpec struct {
					Name        string
					Description string
					Run         func(*T) // this is the function that will be executed by the test suite
				}


				// ClientTestSpec is a test against a single client. You can either put this in your suite
				// directly, or launch it using RunClient or RunAllClients from another test.
				//
				// When used as a test in a suite, the test runs against all available client types.
				//
				// If the Name of the test includes "CLIENT", it is replaced by the client name being tested.
				type ClientTestSpec struct {
					Name        string
					Description string
					Parameters  Params
					Files       map[string]string
					Run         func(*T, *Client) // this is the function that will be executed by the test suite
				}

		It is also possible to add a test case to the test suite without using the two above structs, so long as it
		implements the following interface:

				type AnyTest interface {
					runTest(*Simulation, SuiteID) error
				}




		Creating a Test Run

		A test run can make use of the resources granted to it by the





		Running a Test Suite

		It is possible to call either `RunSuite()` or `MustRunSuite()` on the `Suite`, the only difference being the
		error handling.

				`RunSuite()` will run all tests in the `Suite`, returning an error upon failure.

				`MustRunSuite()` will run all tests in the `Suite`, exiting the process if there is a problem executing
				a test.

		Both functions take a pointer to an instance of `Simulation` (which is just a wrapper object for the simulation
		API) as well as a `Suite`.

		To get an instance of `Simulation`, call the constructor function `New()`. This will look up the hive host
		server URI and return an instance of `Simulation` that will be able to access the running hive host server.


###### Getting information about the client container

To get information about the client that is likely necessary for test execution, you can use `hivesim.Client`within the aforementioned test execution function (`myTestFunction`).

**Enode URL**

To get the client's enode URL, call the `EnodeURL()` method.

**RPC client**

Call `RPC()` to get an RPC client connected to the client's RPC server.

###### Configuring networks

The `hivesim.Simulation` API offers different ways to configure the network set-up for a test. To configure networks within the test execution function (`myTestFunction`), use the methods available on the `Sim` field of the `hivesim.T` parameter.

**Create a network**

```go
err := t.Sim.CreateNetwork(t.SuiteID, "networkName")
```

**Remove a network**

```go
err := t.Sim.RemoveNetwork(t.SuiteID, "networkName")
```

**Connect a container to a network**

```go
err := t.Sim.ConnectContainer(t.SuiteID, "networkName", c.Container)
```

where `c` is the `hivesim.Client` parameter of the test execution function.

If the simulation container also needs to be connected to a network, you can pass in the string "simulation" to the `ConnnectContainer` method, as such:

```go
err := t.Sim.ConnectContainer(t.SuiteID, "networkName", "simulation")
```

**Disconnect a container from a network**

```go
err := t.Sim.DisconnectContainer(t.SuiteID, "networkName", c.Container)
```

**Get a container's IP address on a specific network**

```go
t.Sim.ContainerNetworkIP(t.SuiteID, "networkName", c.Container)
```

The default network used by hive is the `bridge` network. The client container's IP address on the bridge network is available as `IP` field of the `hivesim.Client` object.

However, in the case that the simulation container's IP address on the default network is needed, pass `"bridge"` in as the network name, as such:

```go
t.Sim.ContainerNetworkIP(t.SuiteID, "bridge", "simulation")
```
*/
package hivesim
