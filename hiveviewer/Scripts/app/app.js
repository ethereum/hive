/**************************************************************************************************************/

/* App Class
 * 
 * The app class is the main entity to which the 
 * single-page-app is bound. It is a view-model
 * maintaining the test-suites to view and the
 * filter/sorting toggles.
 */
//app constructor function

var hiveViewer;

function app() {
    var self = this;
    self.rootFolder = "";
    self.showFilters = ko.observable(false);
    self.errorState = ko.observable(false);
    self.errorMessage = ko.observable("");
    self.testSuites = ko.observableArray([]);
    self.selectedClient = ko.observable("");
    self.showPasses = ko.observable(true);
    self.showFails = ko.observable(true);
    self.sortDateAsc = ko.observable(false);
    self.loading = ko.observable(false);
    self.modalVisible = ko.computed(function () {
        var loading = self.loading();
        if (loading) {
            $('#modalSpinner').modal('show');
        } else {
            $('#modalSpinner').on('shown.bs.modal', function (e) {
                $("#modalSpinner").modal('hide');
            });
         
        }
    });
    self.sortedFilteredSuites = ko.computed(function () {
        var a = -1;
        var b = 1;
        if (!self.sortDateAsc()) {
            a = 1;
            b = -1;
        }
        var res = self.testSuites().sort(function (l, r) {
            return l.started() == r.started() ? 0
                : l.started() < r.started() ? a
                    : b;
        }).filter(function (t) {
            
            return            (
                (t.pass() && self.showPasses()) ||
                (!t.pass() && self.showFails())
            ) &&
            (
                self.selectedClient() == "All" ||
                t.primaryClient() == self.selectedClient()
            );
            }
        );
        return res;
    });
    self.filterMenu = ko.computed(function () {
        if (self.showFilters()) {
            return "Hide controls";
        } else {
            return "Show controls";
        }
    });
    self.filterMenuMin = ko.computed(function () {
        if (self.showFilters()) {
            return "-";
        } else {
            return "+";
        }
    });
    self.passFilter = ko.computed(function() {
        if (self.showPasses()) {
            return "Passes";
        } else {
            return "No passes";
        }
    });
    self.failFilter = ko.computed(function () {
        if (self.showFails()) {
            return "Fails";
        } else {
            return "No fails";
        }
    });
    self.sortMode = ko.computed(function () {
        if (self.sortDateAsc()) {
            return "Oldest";
        } else {
            return "Newest";
        }
    });
    self.clients = ko.computed(function () {
        var clientList= self.testSuites().map(function (t) {
            return t.primaryClient();
        });

        var uniqueClientList = $.grep(clientList, function (v, k) {
            return $.inArray(v, clientList) === k;
        });

        uniqueClientList.push("All");
        

        return uniqueClientList;

    });
}

app.prototype.ToggleFilterMenu = function () {
    var self = this;
    self.showFilters(!self.showFilters());
    return true;
}

app.prototype.ToggleFilterPasses = function () {
    var self = this;
    self.showPasses(!self.showPasses());
}

app.prototype.ToggleFilterFailures = function () {
    var self = this;
    self.showFails(!self.showFails());
}

app.prototype.ToggleDateSort = function () {
    var self = this;
    self.sortDateAsc(!self.sortDateAsc());
 }
app.prototype.ExportSuiteJSON = function () {
    var self = this;
    
    self.loading(true);
    //$('#modalSpinner').modal('show');

    var outputArray = [];
    var suitesToExport = ko.toJS(self.sortedFilteredSuites());
    for (var i = 0; i < suitesToExport.length; i++) {
        var suitePath = suitesToExport[i].path + "/" + suitesToExport[i].fileName;
        suitesToExport[i].data.filepath = suitePath;

        outputArray.push(suitesToExport[i].data);

        $.ajax({
            url: suitePath,
            dataType: 'json',
            async: false,
            success: function (suiteData) {
                suitesToExport[i].data.suite = suiteData;
                suitesToExport[i].data.info = "";
            },
            error: function () {
                suitesToExport[i].data.info = "failed to load suite data";
            }

        });
            
    }
    
    var output = JSON.stringify(outputArray);

    self.loading(false);
  //  $('#modalSpinner').modal('hide');
   

    var blob = new Blob([output], { type: "application/json" });

   
    var saveAs = window.saveAs;

    saveAs(blob, "testSuites.json");
    
}

app.prototype.ExportSuiteCSV = function () {
    //var self = this;
    //var blob = new Blob(self.testSuites(), { type: "application/json" });

    //var saveAs = window.saveAs;
    //saveAs(blob, "testSuites.json");

}

app.prototype.LoadTestSuites = function(path, file) {
    var self = this;
    self.rootFolder = path;
    $.ajax({
        url: path+"/"+file,
        data: null,
        success: function (allData) {
            var lines = allData.split("\n").filter(function (line) { return line.length > 0; });
            var testSuiteSummaries = $.map(lines, function (item)
            {
                var summary = new testSuiteSummary(JSON.parse(item));
                summary.path = path;
                return summary;
            });
            self.testSuites(testSuiteSummaries);
        },
        dataType: "text"
    }
    ).
    fail(function (e) {
        alert("error");
    });
}

/**************************************************************************************************************/

/* TestSuiteSummary Class
 *
 * A test suite summary contains metadata
 * about a test suite execution, including
 * for example if the test suite passed
 * and what its purpose is.
 */

// test suite summary ctor function
function testSuiteSummary(data) {
    self = this;
    self.data = data;
    self.path = "";
    self.fileName = ko.observable(data.fileName);
    self.name = ko.observable(data.name);
    self.started = ko.observable(Date.parse(data.start));
    self.primaryClient = ko.observable(data.primaryClient);
    self.pass = ko.observable(data.pass);
    self.passStyle = ko.computed(function () {
        return self.pass() ? "border-success" : "border-danger";
    });
    self.suiteLabel = ko.computed(function () { return "Suite" + self.fileName().slice(0, -5); });
    self.suiteDetailLabel = ko.computed(function () { return "CollapseSuite" + self.fileName().slice(0, -5); });
    self.testSuite = ko.observable();
    self.loading = ko.observable(false);
    self.loaded = ko.observable(false);
    self.loadingError = ko.observable(false);
    self.expanded = ko.observable(false);

}



//testSuiteSummary.prototype.ToggleSuiteState = function () {
//    self = this;
//    self.expanded(!self.expanded());
//}

testSuiteSummary.prototype.ShowSuite = function () {
    var suitePath = this.path + "/" + this.fileName();
    var self = this;
    self.expanded(true);
    if (!self.loaded()) {
        self.loading(true);
        self.loadingError(false);
        $.getJSON(
            suitePath,
            function (suiteData) {
                self.testSuite(new testSuite(suiteData));
                self.loaded(true);
            }

        )
        .fail(function () {
            self.loadingError(true);
        })
        .always(function () {
            self.loading(false);
        })
        ;
    }
    return true;

}

testSuiteSummary.prototype.HideSuite = function () {
    self = this;
    self.expanded(false);
   
    return true;

}

/**************************************************************************************************************/

/* testClientInfo Class
 *
 * This describes a client type
 */

// testClientInfo ctor
function testClientInfo( name, version,  log, instantiated) {

    this.clientName = ko.observable(name);
    this.clientVersion = ko.observable(version);
    this.logfile = ko.observable(log);
    this.clientInstantiated = ko.observable(instantiated);
}

function sleep(ms) {
    return new Promise(function (resolve) { setTimeout(resolve, ms) });
}
//this function repeatedly attempts to call
// initPopup on the popup window, as there
// does not seem to be any reliable way
// of waiting on the definition of the function
// to be ready.
function provokePopup(popup,self) {
    $(popup).ready(async function () {

        var counter = 30;
        var success = false;
        for (i = counter; i >= 0; i--) {

            try {
                popup.initPopup(ko, self);
                success = true;
            }
            catch (e) {
                console.error("Exception thrown", e.stack);
            };


            await sleep(200);
            if (success) break;
        }


    });
}

testClientInfo.prototype.ShowLogs = function () {
    self = this;
    
  
   

    var popup = window.open("popup.html", "_blank", 'toolbar=no, menubar=no, resizable=yes');
  
    provokePopup(popup,self);

   

    return true;

}
/**************************************************************************************************************/

/* testClientResult Class
 *
 * This describes a specific test result in a test case
 * for a specific client type
 */

// testClientResult ctor
function testClientResult(pass,details,name,version,instantiated,log) {
    this.pass = ko.observable(pass);
    this.details = ko.observable(details);
    this.clientName = ko.observable(name);
    this.clientVersion = ko.observable(version);
    this.clientInstantiated = ko.observable(instantiated);
    this.logfile = ko.observable(log);
}
/**************************************************************************************************************/

/* testResult Class
 *
 *  A test result, including if it passed
 *  and some descriptive information
 */

// test case result ctor
function testResult(data) {
    var self = this;
    this.pass = ko.observable(data.pass);
    this.details = ko.observable(data.details);
    this.passLabel = ko.computed(function () {
        return self.pass() ? "pass" : "fail";
    });

}
/**************************************************************************************************************/


/* testCase Class
 *
 *  A test case, which could involve
 *  one or more clients, with name and
 *  description of the intended purpose,
 *  an overall test result and list of
 *  per client results.
 */

// testCase ctor
function testCase(data) {
    var self = this;
    this.id = ko.observable(data.id);
    this.name = ko.observable(data.name);
    this.description = ko.observable(data.description);
    this.start = ko.observable(Date.parse(data.start));
    this.end = ko.observable(Date.parse(data.end));
    this.summaryResult = ko.observable(new testResult(data.summaryResult));
    this.clientResults = ko.observableArray(makeClientResults(data.clientResults, data.clientInfo));
    var clientInfos = makeClientInfo(data.clientInfo);
    this.clients = ko.observableArray(clientInfos);
    self.dur = ko.observable(moment.duration(moment(self.end()).diff(moment(self.start()))));
    self.duration = ko.observable(calcFineDuration(self.dur()));
    self.passTextStyle = ko.computed(function () {
        return self.summaryResult().pass() ? "text-success" : "text-danger";
    });
    
}

function getFilePath(file) {
    if (!file) {
        return "";
    }
    file = file.replace(/\\/g,"/");
    var files = file.split('/');
    if (files[0].toLowerCase() != hiveViewer.rootFolder.toLowerCase()) {
        return hiveViewer.rootFolder + "/" + file;
    } else {
        return file;
    }
}

function makeClientResults(clientResults, clientInfos) {
    return $.map(clientResults, function (testResult,clientName) {
        var pass = testResult.pass;
        var details = testResult.details;
        var name = "Missing client info.";
        var version = "";
        var instantiated;
        var log = "";
        if (clientInfos.hasOwnProperty(clientName)) {
            var clientInfo = clientInfos[clientName];
            name = clientInfo.name;
            version = clientInfo.versionInfo;
            instantiated = clientInfo.instantiatedAt;
            log = getFilePath(clientInfo.logFile);
        }
        return new testClientResult(pass, details, name, version, instantiated, log);
    });
}
function makeClientInfo( clientInfos) {
    return $.map(clientInfos, function (info, infoId) {
        return new testClientInfo(info.name, info.versionInfo, getFilePath(info.logFile) , info.WasInstantiated) ;
    });
}


function calcDuration(duration) {
    var ret = ""
    if (duration.hours() > 0) { ret = ret + duration.hours() + "hr "; }
    if (duration.minutes() > 0) { ret = ret + duration.minutes() + "min "; }
    ret = ret + duration.seconds() + "s "; 
    return ret;
}

function calcFineDuration(duration) {
    var ret = ""
    var hours = 0;
    
    if (duration.minutes() > 0 || duration.hours()>0) { ret = ret + duration.minutes()+(duration.hours()*60) + "min "; }
    ret = ret + duration.seconds() + "s ";
    ret = ret + duration.milliseconds() + "ms ";
    
    return ret;
}
/**************************************************************************************************************/


/* testSuite Class
 *
 *  A test suite, with name and description,
 *  covers a specific functional area, such
 *  as p2p discovery, consensus etc. It is 
 *  a single execution and contains a list of 
 *  testCase results.
 */
function testSuite(data) {
    var self = this;
    self.original = data;
    self.id = ko.observable(data.id);
   
    self.name = ko.observable(data.name);
    self.description = ko.observable(data.description);
    var testCases = $.map(data.testCases, function (item) { return new testCase(item) });
    self.testCases = ko.observableArray(testCases.sort(function (l, r) {
        return l.summaryResult().pass() == r.summaryResult().pass() ? 0
            : l.summaryResult().pass() < r.summaryResult().pass() ? -1
                : 1;
    }));
    var earliest = Math.min.apply(Math, testCases.map(function (tc) { return tc.start(); }));
    var latest = Math.max.apply(Math, testCases.map(function (tc) { return tc.end(); }));
    var fails = testCases.map(function (tc) { if (!tc.summaryResult().pass()) return 1; else return 0; }).reduce(function (a, b) { return a + b; },0);
    var successes = testCases.map(function (tc) { if (tc.summaryResult().pass()) return 1; else return 0; }).reduce(function (a, b) { return a + b; }, 0);
    self.started = ko.observable(earliest);
    self.ended = ko.observable(latest);
    self.passes = ko.observable(successes);
    self.fails = ko.observable(fails);
    var dur= moment.duration(  moment(self.ended()).diff(moment(self.started())));
    self.duration = ko.observable(calcDuration(dur));
    self.showStateFlag = ko.observable(false);
    self.showState = ko.computed(function () {
        return self.showStateFlag() ? "Hide" : "Inline";
    });

    self.pageNumber = ko.observable(0);
    self.maxPerPage = ko.observable(25);
    self.totalPages = ko.computed(function () {
        return Math.ceil(self.testCases().length / self.maxPerPage());
    });
    self.pageEntries = ko.computed(function () {
        var start = self.pageNumber() * self.maxPerPage();
        return self.testCases.slice(start, start + self.maxPerPage());
    });
    self.hasPrevious = ko.computed(function () {
        return (self.pageNumber() !== 0) ? "" : "disabled";
    });

    self.hasNext = ko.computed(function () {
        return (self.pageNumber() !== self.totalPages()) ? "" : "disabled";
    });
    self.nameSortAsc = ko.observable(true);
    self.nameSort = ko.computed(function () { return self.nameSortAsc() ? "fa-angle-down" : "fa-angle-up" });
    self.startSortAsc = ko.observable(true);
    self.startSort = ko.computed(function () { return self.startSortAsc() ? "fa-angle-down" : "fa-angle-up" });
    self.durationSortAsc = ko.observable(true);
    self.durationSort = ko.computed(function () { return self.durationSortAsc() ? "fa-angle-down" : "fa-angle-up" });
    self.passSortAsc = ko.observable(true);
    self.passSort = ko.computed(function () { return self.passSortAsc() ? "fa-angle-down" : "fa-angle-up" });
}


testSuite.prototype.nameSortToggle = function () {
    self = this;
    self.nameSortAsc(!self.nameSortAsc());
    //implemented here on the toggle event, which is not recommended,
    //as a way of implementing 'sort this, then that', depending on
    //click order
    self.testCases.sort(function (l, r) {
        if (!self.nameSortAsc()) { var s = r; r = l; l = s;}
        return l.name() == r.name() ? 0
            : l.name() < r.name() ? -1
                : 1;
    });

}

testSuite.prototype.durationSortToggle = function () {
    self = this;
    self.durationSortAsc(!self.durationSortAsc());
    
    self.testCases.sort(function (l, r) {
        if (!self.durationSortAsc()) { var s = r; r = l; l = s; }
        return l.dur().asMilliseconds() == r.dur().asMilliseconds() ? 0
            : l.dur().asMilliseconds() < r.dur().asMilliseconds() ? -1
                : 1;
    });

}

testSuite.prototype.startSortToggle = function () {
    self = this;
    self.startSortAsc(!self.startSortAsc());

    self.testCases.sort(function (l, r) {
        if (!self.startSortAsc()) { var s = r; r = l; l = s; }
        return l.start() == r.start() ? 0
            : l.start() < r.start() ? -1
                : 1;
    });
}


testSuite.prototype.passSortToggle = function () {
    self = this;
    self.passSortAsc(!self.passSortAsc());

    self.testCases.sort(function (l, r) {
        if (!self.passSortAsc()) { var s = r; r = l; l = s; }
        return l.summaryResult().pass() == r.summaryResult().pass() ? 0
            : l.summaryResult().pass() < r.summaryResult().pass() ? -1
                : 1;
    });
}


testSuite.prototype.Next = function () {
    var self = this;
    if (self.pageNumber() < self.totalPages()) {
        self.pageNumber(self.pageNumber() + 1);
    }
}

testSuite.prototype.Previous = function () {
    var self = this;
    if (self.pageNumber() != 0) {
        self.pageNumber(self.pageNumber() - 1);
    }
}

testSuite.prototype.ToggleTestCases = function () {
    var self = this;

    self.showStateFlag(!self.showStateFlag());

}

testSuite.prototype.OpenTestSuite = function (vm,e) {
    var context = ko.contextFor(e.target);
    var summary= context.$parent;
   // var clonedSummary = new testSuiteSummary(ko.utils.parseJson(ko.toJSON(summary)));
 
 //   var clonedSuite = new testSuite(ko.utils.parseJson(ko.toJSON(this.original)));
  //  clonedSummary.testSuite(clonedSuite);
  //  clonedSuite.maxPerPage(100);
    var popup = window.open("testsuite.html", "_blank", 'toolbar=no, menubar=no, resizable=yes');

    provokePopup(popup, summary);


}



