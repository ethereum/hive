import '../extlib/bootstrap.module.js'
import '../extlib/jquery.dataTables.module.js'
import { $ } from '../extlib/jquery.module.js'
import { html, format, nav } from './utils.js'

const resultsRoot = "/results/"

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
	return html.get_link("viewer.html?file=" + escape(data), name)
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
	var suites = []
	data.split("\n").forEach(function(elem, index) {
		if (!elem) {
			return;
		}
		let obj = JSON.parse(elem)
		suites.push(obj)
	})
	filetable = $("#filetable").DataTable({
		data: suites,
		pageLength: 50,
		autoWidth: false,
		order: [[0, 'desc']],
		columns: [
			{
				title: "Start time",
				data: "start",
				type: "date",
				width: "12.5em",
				render: function(data) {
					return new Date(data).toISOString();
				},
			},
			{
				title: "Suite",
				data: "name",
				width: "20%",
			},
			{
				title: "Clients",
				data: "clients",
				width: "30%",
				className: "ellipsis",
				render: function(data) {
					return data.join(",")
				},
			},
			{
				title: "Pass",
				data: null,
				width: "9em",
				render: function(data) {
					if (data.fails > 0) {
						return "&#x2715; <b>Fail (" + data.fails + " / " + (data.fails + data.passes) + ")</b>"
					}
					return "&#x2713 (" + data.passes + ")"
				},
			},
			{
				title: "Log",
				data: "simLog",
				width: "3%",
				orderable: false,
				render: function(file) {
					return logview("results/" + file)
				},
			},
			{
				title: "Load?",
				data: null,
				width: "200px",
				orderable: false,
				render: function(data) {
					let size = format.units(data.size)
					let btn = '<button type="button" class="btn btn-sm btn-primary"><span class="loader" role="status" aria-hidden="true"></span><span class="txt">Load (' + size + ')</span></button>'
					let raw = logview("results/" + data.fileName, "[json]")
					return btn + "&nbsp;" + raw
				},
			},
		],
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
			button.prop("disabled", false);
			spinner.removeClass(spinClasses);
			if (status) {
				label.text("Loaded OK");
				button.prop("title", "");
				openTestSuitePage(fname);
			} else {
				label.text("Loading failed");
				button.prop("title", "Computer says no: " + errmsg);
			}
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
		nav.store({"page": ev.target.id});
	});
	window.addEventListener("popstate", navigationDispatch);
	navigationDispatch();
});

// navigationDispatch switches to the tab selected by the URL.
function navigationDispatch() {
	let suite = nav.load("suite");
	if (suite) {
		// TODO: fix it so we show Loading spinner, and status 'Loaded' (unselectable) once it's loaded.
		loadTestSuite(suite, function(ok) {

		});
	}
	let page = nav.load("page") || "v-pills-home-tab";
	let elem = $("#" + page);
	if (elem && elem.tab) {
		elem.tab("show");
	}
}

// openTestSuitePage navigates to the test suite tab.
function openTestSuitePage(suitefile) {
	// store in url query
	nav.store({
		"page": "v-pills-results-tab",
		"suite": suitefile,
	});
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

// The datatables
var overallresults = null; // Overall results
var execresults = null; // Execution results
var failuresummary = null; // Failure summary

function logFolder(jsonsource, client) {
	return jsonsource.split(".")[0];
}

/* Formatting function for row details */
function formatTestDetails(d) {
	// `d` is the original data object for the row
	var txt = '<div class="details-box">';
	txt += "<p><b>Name</b><br/>" + html.encode(d.name) + "</p>";

	if (d.description != "") {
		txt += "<p>";
		txt += "<b>Description</b><br/>"
		txt += html.urls_to_links(html.encode(d.description));
		txt += "</p>";
	}
	if (d.summaryResult.details != "") {
		txt += "<p><b>Details</b><pre><code>";
		txt += html.urls_to_links(html.encode(d.summaryResult.details));
		txt += "</code></pre></p>";
	}
	txt += "</div>";
	return txt;
}

function onSuiteData(data, jsonsource) {
	// data structure of suite data:
	/*
	data = {
		"id": 0,
		"name": "Devp2p discovery v4 test suite",
		"description": "This suite of tests checks for basic conformity to the discovery v4 protocol and for some known security weaknesses.",
		"clientVersions": "",
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
				"clientInfo": {
					"a46beeb9": {
						"id": "a46beeb9",
						"name": "parity_latest",
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
	$("#testsuite_name").text(data.name);
	$("#testsuite_desc").html(html.urls_to_links(html.encode(data.description)));
	if (data.clientVersions) {
		// Remove empty version strings.
		for (let key in data.clientVersions) {
			if (!data.clientVersions[key]) {
				delete data.clientVersions[key];
			}
		}
		$("#testsuite_clients").html(html.make_definition_list(data.clientVersions));
	} else {
		// This is here for backward-compatibility with old suite files.
		// Remove this after June 2021.
		$("#testsuite_clients").html("");
	}

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
		autoWidth: false,
		order: [[2, 'desc']],
		columns: [
			// First column is an 'expand'-button
			{
				className: 'details-control',
				orderable: false,
				data: null,
				defaultContent: '',
				width: "20px",
			},
			// Second column: Name
			{
				title: "Test",
				data: "name",
				width: "79%",
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
				},
				width: "50px",
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
				},
				width: "19%",
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
			row.child(formatTestDetails(row.data())).show();
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
