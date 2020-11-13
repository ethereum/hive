# Simulation API Wrapper

The simulation API wrapper provides an easy-to-use way of accessing the hive simulation API. 

###### Accessing the `hivesim` test API:
To access the `hivesim` API, create a `Suite` as such:
```go
	suite := hivesim.Suite{
		Name:        "MyTest",
		Description: "This simulation test does XYZ.",
	}
```
The `Suite` will only execute tests that are added to it using the `Add()` method. 

###### Adding a test to the `Suite`

To add a test to the `Suite`, you must write a function with the following signature:

```go
func myTestFunction(t *hivesim.T, c *hivesim.Client)
```
where 

* `hivesim.T` represents a running test. It behaves similarly to `testing.T` in package `testing` (a Golang standard library for testing), but has some additional methods for launching clients; and

* `hivesim.Client`represents a running client.

The job of `myTestFunction()` is to get any information necessary from the client container to execute the test and to create or modify docker networks in the case that the simulation requires a more complex networking set up.

`myTestFunction()` may be added to the `Suite` using `hivesim.ClientTestSpec`, which represents a test against a single client:

```go
type ClientTestSpec struct {
	Name        string // name of the test
	Description string // description of the test
	Parameters  Params // parameters necessary to configure the client
	Files       map[string]string // files to be mounted into the client container
	Run         func(*T, *Client) // the test function itself
}
```

where the `Run` field would take `myTestFunction` as the parameter.

######  Running the `Suite`

To run the `Suite`, you can call either `RunSuite()`or `MustRunSuite()` depending on how you want errors to be handled. 

* `RunSuite()` will run all tests in the `Suite`, returning an error upon failure. 

* `MustRunSuite()` runs the given suite, exiting the process if there is a problem executing the test.

Both Run functions take `Suite` as a parameter as well as a pointer to an instance of `hivesim.Simulation`, as such: 

```go
hivesim.MustRunSuite(hivesim.New(), suite)
```

`hivesim.Simulation` is just a wrapper API that can access the hive server. To get a new instance of `hivesim.Simulation`, call `hivesim.New()`. This will look up the hive host server URI and connect to it


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