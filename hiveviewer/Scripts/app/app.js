//app constructor function
function app() {
    var self = this;
    self.errorState = ko.observable(false);
    self.errorMessage = ko.observable("");
    self.testSuites = ko.observableArray([]);
}

// test result for a client
function testClientResult(pass,details,name,version,instantiated,log) {
    this.pass = ko.observable(pass);
    this.details = ko.observable(details);
    this.clientName = ko.observable(name);
    this.clientVersion = ko.observable(version);
    this.clientInstantiated = ko.observable(instantiated);
    this.logfile = ko.observable(log);
}

function makeClientResults(clientResults, clientInfos) {
    $.map(clientResults, function (clientName, testResult) {
        var pass = testResult.pass;
        var details = testResult.details;
        var name = "Missing client info.";
        var version = "";
        var instantiated;
        var log="";
        if (clientInfos.hasOwnProperty(clientName)) {
            var clientInfo = clientInfos[clientName];
            name = clientInfo.name;
            version = clientInfo.versionInfo;
            instantiated = clientInfo.instantiatedAt;
            log = clientInfo.logFile;
        }
        return new testClientResult(pass, details, name, version, instantiated, log);
    });
}

// test case result 
function testResult(data) {
    this.pass = ko.observable(data.pass);
    this.details = ko.observable(data.details);

}

function testCase(data) {
    this.id = ko.observable(data.id);
    this.name = ko.observable(data.name);
    this.description = ko.observable(data.description);
    this.start = ko.observable(data.start);
    this.end = ko.observable(data.end);
    this.summaryResult = ko.observable(new testResult(data.summaryResult));
    this.clientResults = ko.observableArray(makeClientResults(data.clientResults, data.clientInfo));
}

function testSuite(data) {
    var self = this;
    this.id = ko.observable(data.id);
    this.suiteLabel = ko.computed(function () { return "Suite"+self.id()})
    this.name = ko.observable(data.name);
    this.description = ko.observable(data.description);
    var testCases = $.map(data.testCases, function (item) { return new testCase(item) });
    this.testCases = ko.observableArray(testCases)

}

app.prototype.LoadTestSuites = function (src) {
    var self = this;
    $.getJSON(src, function (allData) {
        var testSuites = $.map(allData, function (item) { return new testSuite(item) });
        self.testSuites(testSuites);
    });    
}