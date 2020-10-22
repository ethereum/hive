const resultsRoot = "/results/"

utils = {
    /*
     * HTML-encoding
     */
    html_encode: function(str) {
        //Let the DOM do it for us.
        var d = document.createElement('textarea');
        d.innerText = str;
        //Yes, I'm aware of
        // http://stackoverflow.com/questions/1219860/html-encoding-in-javascript-jquery
        // I just don't agree.
        return d.innerHTML;
    },

    // encapsulate data inside a tag
    tag: function(typ, str) {
        //Let the DOM do it for us.
        var d = document.createElement(typ);
        d.innerText = ("" + str);
        return d.outerHTML;
    },

    /*
     * HTML Attribute encoding
     */
    attr_encode: function(str) {
        x = document.createElement("x");
        x.setAttribute("b", str);
        var all = x.outerHTML;
        return all.substring(6, all.length - 6);
    },

    /*
     * Dirty url-parsing of the url hash segment
     */
    get_hash_params: function() {
        var retval = {}
        var query = window.location.hash.substring(1);
        var vars = query.split('&');
        for (var i = 0; i < vars.length; i++) {
            var pair = vars[i].split('=');
            retval[decodeURIComponent(pair[0])] = decodeURIComponent(pair[1])
        }
        return retval;
    },

    /*
     * Creates an anchor-element from 'untrusted' link 'data'
     */
    get_link: function(url, text) {
        var a = document.createElement('a');
        a.setAttribute("href", url);
        a.text = text;
        return a.outerHTML;
    },
    get_js_link: function(js, text) {
        var a = document.createElement('a');
        a.setAttribute("href", "javascript:" + js);
        a.text = text;
        return a.outerHTML;
    },

    /*
     * Creates
     * <button type="button" class="btn btn-default">Default</button>
     */
    get_button: function(onclick, text) {
        var a = document.createElement('button');
        a.setAttribute("type", "button");
        a.setAttribute("class", "btn btn-primary btn-xs")
        a.textContent = text;
        a.setAttribute("onclick", onclick)
        return a.outerHTML;
    },

    format_timespan: function(d1, d2) {
        var diff = d2 - d1;
        var _s = "";
        if (diff < 0) {
            _s = "-";
            diff = -diff;
        }
        var d = Math.floor(diff / 86400000);
        diff %= 86400000;
        var h = Math.floor(diff / 3600000);
        diff %= 3600000;
        var m = Math.floor(diff / 60000);
        diff %= 60000;
        var s = Math.floor(diff / 1000);

        var a = d ? (d + "d") : "";
        a += ((a || h) ? (h + "h") : "");
        a += ((a || m) ? (m + "m") : "") + s + "s";
        return _s + a;
    },

    // human readable units
    units: function(loc) {
        if (loc < 1024) {
            return loc + "B"
        }
        loc = loc / 1024
        if (loc < 1024) {
            return loc.toFixed(2) + "KB";
        }
        loc = loc / 1024
        return loc.toFixed(2) + "MB";
    },

    /*
    Expects an object like
        {
            "repo": "https://github.com/ethereum/go-ethereum",
            "commit": "021c3c281629baf2eae967dc2f0a7532ddfdc1fb",
            "branch": "release/1.6"
        }
    Will return a link to the right place in the repo

    link : https://github.com/ethereum/go-ethereum/tree/021c3c281629baf2eae967dc2f0a7532ddfdc1fb
    text : ethereum/go-ethereum@021c3c2 [⎇ release/1.6]
    */
    githubRepoLink: function(data) {
        if (data.repo == "") {
            return "";
        }
        if (data.commit == "") {
            return data.repo;
        }
        if (data.branch == "") {
            return data.repo + "@" + data.commit.slice(0, 7);
        }
        var text = data.repo + "@" + data.commit.slice(0, 7) + " [⎇ " + data.branch + "]";
        if (!data.repo.startsWith("https://")) {
            return utils.html_encode(text); // not github
        }

        var a = document.createElement('a');
        a.setAttribute("target", "_blank");
        // Set just repo first
        a.setAttribute("href", data.repo);
        // Use path for text
        a.text = a.pathname.slice(1) + "@" + data.commit.slice(0, 7) + " [\u2387 " + data.branch + "]";
        // Set both repo and tree/commit version
        a.setAttribute("href", data.repo + "/tree/" + data.commit);
        return a.outerHTML;
    }
}

// nav is a little utility to store things in the url, so that people can link into stuff.
var nav = {
    load: function(key) {
        if (!URLSearchParams) {
            progress("Error: browser doesn't support URLSearchParams. IE or what? ")
            return null
        }
        return new URLSearchParams(location.search).get(key);
    },
    // store stores the key/val combo in the url query
    // this overwrites any previous key
    store: function(key, val) {
        // get current location
        let old = new URLSearchParams(location.search)
        old.set(key, val)
        history.pushState(null, null, "?" + old.toString())
    },
}

function progress(message) {
    console.log(message)
    let a = $("#debug").text();
    $("#debug").text((new Date()).toLocaleTimeString() + " | " + message + "\n" + a);
}

function resultStats(fails, success, total) {
    f = parseInt(fails), s = parseInt(success);
    t = parseInt(total);
    f = isNaN(f) ? "?" : f;
    s = isNaN(s) ? "?" : s;
    t = isNaN(t) ? "?" : t;
    return '<b><span class="text-danger">' + f +
        '</span>&nbsp;:&nbsp;<span class="text-success">' + s +
        '</span> &nbsp;/&nbsp;' + t + '</b>';
}

function logview(data, name) {
    if (!name) {
        name = "log"
    }
    return utils.get_link("viewer.html?file=" + escape(data), name)
}

function onFileListing(data, error) {
    progress("Got file list")
    // the data is jsonlines
    /*
        {
            "fileName": "./1587325327-fa7ec3c7d09a8cfb754097f79df82118.json",
            "name": "Sync test suite",
            "start": "",
            "simLog": "1587325280-00befe48086b1ef74fbb19b9b7d43e4d-simulator.log",
            "passes": 0,
            "fails": 0,
            "size": 435,
            "clients": [],
            "description": "This suite of tests verifies that clients can sync from each other in different modes.\n It consists of two specific tests, both using geth as the reference client, testing these two aspects: \n\n- Whether the client-under-test can sync from geth\n- Whether geth can sync from the client-under-test'\n",
            "ntests": 0
    }
    */
    let table = $("#filetable")
    suites = []
    data.split("\n").forEach(function(elem, index) {
        if (!elem) {
            return;
        }
        obj = JSON.parse(elem)
        //suites.push([  obj.start,obj.name , obj.simLog, obj.primaryClient, obj.pass, obj.fileName])
        suites.push(obj)
    })
    filetable = $("#filetable").DataTable({
        data: suites,
        pageLength: 50,
        columns: [
            {
                title: "Start time",
                data: "start",
                type: "date",
                render: function(data) {
                    return new Date(data).toLocaleString();
                }
            },
            {
                title: "Test suite",
                data: "name"
            },
            {
                title: "Suite log",
                data: "simLog",
                render: function(file) {
                    return logview("results/" + file)
                }
            },
            {
                title: "Clients",
                data: "clients",
                render: function(data) {
                    return data.join(",")
                }
            },
            {
                title: "Pass",
                data: null,
                render: function(data) {
                    if (data.fails > 0) {
                        return "&#x2715; <b>Fail (" + data.fails + " / " + (data.fails + data.passes) + ")</b>"
                    }
                    return "&#x2713 (" + data.passes + ")"
                }
            },
            // { title: "Number of tests"},
            {
                title: "Load?",
                data: null,
                render: function(data) {
                    let size = utils.units(data.size)
                    btn = '<button type="button" class="btn btn-sm btn-primary"><span class="loader" role="status" aria-hidden="true"></span><span class="txt">Load (' + size + ')</span></button>'
                    raw = logview("results/" + data.fileName, "[json]")
                    return btn + "&nbsp;" + raw
                },
            },
        ],
        order: [[0, 'desc']],
    });

    $('#filetable tbody').on('click', 'button', function() {
        // Documentation about spinners: https://getbootstrap.com/docs/4.4/components/spinners/
        let spinClasses = "spinner-border spinner-border-sm"
        let data = filetable.row($(this).parents('tr')).data();
        let fname = data.fileName;
        let button = $(this).prop("disabled", true)
        let spinner = button.children(".loader").addClass(spinClasses)
        let label = button.children(".txt").text("Loading")
        let onDone = function(status, errmsg) {
            if (status) {
                label.text("Loaded OK");
                button.prop("title", "")
                spinner.removeClass(spinClasses)
                openTestSuitePage(fname);
                return
            }
            label.text("Loading failed")
            spinner.removeClass(spinClasses)
            button.prop("title", "Computer says no: " + errmsg).prop("disabled", false);
        }
        loadTestSuite(fname, onDone);
    });
}

$(document).ready(function() {
    // Retrieve the list of files
    progress("Loading file list...")
    $.ajax("listing.jsonl", {
        success: onFileListing,
        failure: function(status, err) {
            alert(err);
        },
    })

    // Handle navigation clicks.
    $(".nav-link").on("click", function(ev) {
        nav.store('page', ev.target.id);
    });
    window.addEventListener('popstate', navigationDispatch);
    navigationDispatch();
});

// navigationDispatch switches to the tab selected by the URL.
function navigationDispatch() {
    let suite = nav.load("suite")
    if (suite) {
        // TODO: fix it so we show Loading spinner, and status 'Loaded' (unselectable) once it's loaded.
        loadTestSuite(suite, function(ok) {});
    }
    let page = nav.load("page");
    if (page) {
        let elem = $("#" + page);
        if (elem && elem.tab) {
            elem.tab("show");
        }
    }
}

// openTestSuitePage navigates to the test suite tab.
function openTestSuitePage(suitefile) {
    // store in url query
    nav.store("suite", suitefile);
    nav.store("page", "v-pills-results-tab");
    $("#v-pills-results-tab").tab("show")
}

// loadTestSuite loads the given testsuite file.
function loadTestSuite(suitefile, doneFn) {
    //let filename = "results/"+suitefile
    let filename = suitefile
    progress("Loading " + filename);
    var jqxhr = $.getJSON(resultsRoot + "/" + filename, function(data) {
        doneFn(true);
        onSuiteData(data, filename);
    }).fail(function(x, status, err) {
        progress("error fetching " + filename + " : " + err)
        doneFn(false, err);
    });
}

/*
 * Performs filtering on the "Execution Result" datatable
 */
function execfilter(str) {
    $('#execresults').dataTable().api().search(str).draw();
}

var converter = new showdown.Converter()

// The datatables
var overallresults = null; // Overall results
var execresults = null; // Execution results
var failuresummary = null; // Failure summary

// Contains all the data that we load
var alldata = {};

function logFolder(jsonsource, client) {
    return jsonsource.split(".")[0];
}

/* Formatting function for row details */
function format(d) {
    // `d` is the original data object for the row
    let txt = ""
    txt += "<b>Name</b>" + utils.tag('p', d.name)
    txt += "<b>Description</b>" + converter.makeHtml(d.description)
    txt += "<br/><b>Details</b>" + converter.makeHtml(d.summaryResult.details)
    return txt
}

function onSuiteData(data, jsonsource) {
    // data structure of suite data:
    /*
    data = {
        "id": 0,
        "name": "Devp2p discovery v4 test suite",
        "description": "This suite of tests checks for basic conformity to the discovery v4 protocol and for some known security weaknesses.",
        "testCases": {
            "1": {
                "id": 1,
                "name": "SpoofSanityCheck(v4013)",
                "description": "A sanity check to make sure that the network setup works for spoofing",
                "start": "2020-04-22T17:12:13.018490141Z",
                "end": "2020-04-22T17:12:17.169151639Z",
                "summaryResult": {
                    "pass": true,
                    "details": " \n"
                },
                "clientResults": null,
                "clientInfo": {
                    "a46beeb9": {
                        "id": "a46beeb9",
                        "name": "parity_latest",
                        "versionInfo": "",
                        "instantiatedAt": "2020-04-22T17:12:14.275491827Z",
                        "logFile": "parity_latest/client-a46beeb9.log",
                        "WasInstantiated": true
                    }
                }
            },
        }
    }
    */

    // Set title info
    $("#testsuite_name").text(data.name)
    $("#testsuite_desc").text(data.description)

    // Convert to list
    let cases = []
    for (var k in data.testCases) {
        cases.push(data.testCases[k])
    }
    progress("got " + cases.length + " testcases")

    //datatables can't be reinitalized, we need to destroy them if they exist
    if (execresults != null) {
        execresults.clear().destroy();
        $("#execresults").html("")
    }

    // Init the datatable
    var thetable = $('#execresults').DataTable({
        data: cases,
        pageLength: 100,
        columns: [
            // First column is an 'expand'-button
            {
                "className": 'details-control',
                "orderable": false,
                "data": null,
                "defaultContent": ''
            },
            // Second column: Name
            {
                title: "Name",
                data: "name"
            },
            //  Status: pass or not
            {
                title: "Status",
                data: "summaryResult",
                render: function(summaryResult) {
                    if (summaryResult.pass) {
                        return "&#x2713"
                    };
                    return "&#x2715; <b>Fail</b>";
                }
            },
            // The logs for clients related to the test
            {
                title: "Logs",
                data: "clientInfo",
                render: function(clientInfo) {
                    let logs = []
                    for (let instanceID in clientInfo) {
                        let instanceInfo = clientInfo[instanceID]
                        logs.push(logview("results/" + instanceInfo.logFile, instanceInfo.name))
                    }
                    return logs.join(",")
                }
            },
        ],
    });

    // This sets up the expanded info on click
    // https://www.datatables.net/examples/api/row_details.html
    $('#execresults tbody').on('click', 'td.details-control', function() {
        var tr = $(this).closest('tr');
        var row = thetable.row(tr);
        if (row.child.isShown()) {
            // This row is already open - close it
            row.child.hide();
            tr.removeClass('shown');
        } else {
            // Open this row
            row.child(format(row.data())).show();
            tr.addClass('shown');
        }
    });
    execresults = thetable
    return
    /*  if(params.execfilter){
            execfilter(params.execfilter);
        }
        if(params.summaryfilter){
            $('#summary').dataTable().api()
                .search(params.summaryfilter).draw();
        }
    */
}
