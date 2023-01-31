import '../extlib/bootstrap.module.js'
import '../extlib/dataTables.module.js'
import { $ } from '../extlib/jquery.module.js'
import { html, nav, format, loader } from './utils.js'
import * as app from './app.js'

$(document).ready(function () {
	app.init();

	let name = nav.load("suitename");
	if (name) {
		showSuiteName(name);
	}
	let filename = nav.load("suiteid");
	if (!filename) {
		showError("no suite ID in URL");
		return;
	}
	var testid = null;
	if (window.location.hash.match(/^#test-/)) {
		testid = parseInt(window.location.hash.replace(/^#test-/, ''));
	}

	console.log("Loading:", filename, "name:", name);
	$.ajax({
		xhr: loader.newXhrWithProgressBar,
		type: 'GET',
		url: app.resultsRoot + filename,
		dataType: 'json',
		success: function(suiteData) {
			showSuiteData(suiteData, filename);
			if (testid) {
				scrollToTest(suiteData, testid);
			}
		},
		error: function(xhr, status, error) {
			showError("error fetching " + filename + " : " + error);
		},
	});
})

function showError(message) {
	console.error(message);
	$("#testsuite_desc").text("Error: " + message);
}

// formatting function for row details.
// `d` is the original data object for the row
function formatTestDetails(suiteData, d) {
	let container = document.createElement("div");
	container.classList.add("details-box");

	if (d.description != "") {
		let descP = document.createElement("p");
		let description = html.urls_to_links(html.encode(d.description.trim()));
		let txt = "<b>Description</b><br/>" + description;
		descP.innerHTML = txt;
		container.appendChild(descP)
	}

	if (d.summaryResult.details != "") {
		let detailsP = document.createElement("p");
		let detailsTitle = document.createElement("b");
		detailsTitle.innerText = "Details";
		detailsP.appendChild(detailsTitle);
		container.appendChild(detailsP);

		let detailsOutput = formatTestLog(suiteData, d);
		container.appendChild(detailsOutput);
	}

	return container;
}

// countLines returns the number of lines in the given string.
function countLines(text) {
	var lines = 0, offset = 0;
	while (true) {
		lines++;
		offset = text.indexOf('\n', offset);
		if (offset == -1) {
			return lines;
		} else {
			offset++;
		}
	}
}

// formatTestLog processes the test output. Log output from the test is shortened
// to avoid freezing the browser.
function formatTestLog(suiteData, test) {
	const maxLines = 25;

	let text = test.summaryResult.details;
	let totalLines = countLines(text);

	var offset = 0, end = 0, lineNumber = 0;
	var prefixOutput = "";
	var suffixOutput = "";
	var hiddenLines = 0;
	while (end < text.length) {
		// Find bounding indexes of the next line.
		end = text.indexOf('\n', offset);
		if (end == -1) {
			end = text.length;
		}
		let begin = offset;
		offset = end+1;

		// Collect lines if they're in the visible range.
		let inPrefix = lineNumber < maxLines;
		let inSuffix = lineNumber > (totalLines-maxLines);
		if (inPrefix || inSuffix) {
			let line = text.substring(begin, end);
			let content = highlightErrorsInTestOutput(html.encode(line));
			if (lineNumber < totalLines-1) {
				content += "\n";
			}
			if (inPrefix) {
				prefixOutput += content;
			} else {
				suffixOutput += content;
			}
		} else {
			hiddenLines++;
		}
		lineNumber++;
	}

	// Create the output sections.
	let output = document.createElement("div");
	output.classList.add("test-output");

	if (prefixOutput.length > 0) {
		// Add the beginning of text.
		let el = document.createElement("code");
		el.innerHTML = prefixOutput;
		el.classList.add("output-prefix");
		if (suffixOutput.length == 0) {
			el.classList.add("output-suffix");
		}
		output.appendChild(el);
	}

	if (hiddenLines > 0) {
		// Create the truncation marker.
		let linkText = "..." + hiddenLines + " lines hidden: click for full output...";
		let linkURL = app.route.testLogInViewer(suiteData.suiteID, suiteData.name, test.testIndex);
		let trunc = html.get_link(linkURL, linkText);
		trunc.classList.add("output-trunc");
		output.appendChild(trunc);
	}

	if (suffixOutput.length > 0) {
		// Add the remaining text.
		let el = document.createElement("code");
		el.innerHTML = suffixOutput;
		el.classList.add("output-suffix");
		output.appendChild(el);
	}

	return output;
}

function highlightErrorsInTestOutput(content) {
	return content.replace(/(ERROR|FAIL|Error)(:)?.*/, function (m) {
		return '<span class="output-error">' + m + '</span>';
	});
}

// showSuiteName displays the suite title.
function showSuiteName(name) {
	$("#testsuite_name").text(name);
	document.title = name + " - hive";
}

// showSuiteData displays the suite and its tests in the table.
// This is called after loading the suite.
function showSuiteData(data, suiteID) {
	let suiteName = data.name;
	data['suiteID'] = suiteID;

	// data structure of suite data:
	/*
	data = {
		"id": 0,
		"name": "Devp2p discovery v4 test suite",
		"description": "This suite of tests checks...",
		"simLog": "1674486996-simulator-0ee…eb2e3f04a893bff1017.log",
		"clientVersions": { "parity_latest": "..." },
		"testCases": {
			"1": {
				"id": 1,
				"name": "SpoofSanityCheck(v4013)",
				"description": "A sanity check to make sure that the network setup works for spoofing",
				"start": "2020-04-22T17:12:13.018490141Z",
				"end": "2020-04-22T17:12:17.169151639Z",
				"summaryResult": {
					"pass": true,
					"details": "text\n"
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

	// Set title info.
	showSuiteName(data.name);
	$("#testsuite_desc").html(html.urls_to_links(html.encode(data.description)));

	// Set client versions.
	if (data.clientVersions) {
		// Remove empty version strings.
		for (let key in data.clientVersions) {
			if (!data.clientVersions[key]) {
				delete data.clientVersions[key];
			}
		}
		$("#testsuite_clients").html(html.make_definition_list(data.clientVersions));
	}

	// Convert test cases to list.
	let cases = [];
	for (var k in data.testCases) {
		let tc = data.testCases[k];
		tc['testIndex'] = k;
		tc['duration'] = testCaseDuration(tc);
		cases.push(tc);
	}
	console.log("got " + cases.length + " testcases");

	// Fill info box.
	let suiteTimes = testSuiteTimes(cases);
	$("#testsuite_start").html("🕒 " + suiteTimes.start.toLocaleString());
	$("#testsuite_duration").html("⌛️ " + format.duration(suiteTimes.duration));
	let logfile = app.resultsRoot + data.simLog;
	let url = app.route.logFileInViewer(suiteID, suiteName, logfile);
	$("#sim-log-link").attr("href", url);
	$("#sim-log-link").text("simulator log");
	$("#testsuite_info").show();

	// Initialize the DataTable.
	let table = $('#execresults').DataTable({
		data: cases,
		pageLength: 100,
		autoWidth: false,
		responsive: {
			// Turn off display of hidden columns because it conflicts with our own use of
			// child rows. This should be OK since the only column that will be ever be
			// hidden is 'duration'.
			details: {
				type: 'none',
				display: function (row, update, render) {},
			},
		},
		order: [[1, 'desc']],
		columns: [
			{
				title: "Test",
				data: "name",
				className: "test-name-column",
				width: "65%",
				responsivePriority: 0,
			},
			// Status: pass or not.
			{
				title: "Status",
				data: "summaryResult",
				className: "test-status-column",
				width: "80px",
				responsivePriority: 0,
				render: function(summaryResult) {
					if (summaryResult.pass) {
						return "&#x2713"
					};
					let s = summaryResult.timeout ? "Timeout" : "Fail";
					return "&#x2715; <b>" + s + "</b>";
				},
			},
			// Test duration.
			{
				title: "⌛️",
				data: "duration",
				className: "test-duration-column",
				width: "6em",
				responsivePriority: 2,
				type: "num",
				render: function (v, type, row) {
					if (type === 'display' || type === 'filter') {
						return format.duration(v);
					}
					return v;
				},
			},
			// The logs for clients related to the test.
			{
				title: "Logs",
				data: "clientInfo",
				width: "20%",
				responsivePriority: 1,
				render: function(clientInfo) {
					let logs = []
					for (let instanceID in clientInfo) {
						let instanceInfo = clientInfo[instanceID]
						let logfile = app.resultsRoot + instanceInfo.logFile;
						let url = app.route.logFileInViewer(suiteID, suiteName, logfile);
						let link = html.get_link(url, instanceInfo.name)
						logs.push(link.outerHTML);
					}
					return logs.join(", ")
				},
			},
		],
		rowCallback: function(row, data, displayNum, displayIndex, dataIndex) {
			if (!cases[dataIndex].summaryResult.pass) {
				row.classList.add("failed");
			}
		},
	});

	// This sets up the expanded info on click.
	// https://www.datatables.net/examples/api/row_details.html
	$('#execresults tbody').on('click', 'td.test-name-column', function() {
		let tr = $(this).closest('tr');
		toggleTestDetails(data, table, tr);
	});
}

// testSuiteTimes computes start/end/duration of a test suite.
// The duration is returned in milliseconds.
function testSuiteTimes(cases) {
	if (cases.length == 0) {
		return 0;
	}
	var start = cases[0].start;
	var end = cases[0].end;
	for (var i = 1; i < cases.length; i++) {
		let test = cases[i];
		if (test.start < start) {
			start = test.start;
		}
		if (test.end > end) {
			end = test.end;
		}
	}
	return {
		start: new Date(start),
		end: new Date(end),
		duration: Date.parse(end) - Date.parse(start),
	}
}

// testCaseDuration computes the duration of a single test case in milliseconds.
function testCaseDuration(test) {
	return Date.parse(test.end) - Date.parse(test.start);
}

// scrollToTest scrolls to the given test row index.
function scrollToTest(suiteData, testIndex) {
	let table = $('#execresults').dataTable().api();
	let row = findRowByTestIndex(table, testIndex);
	if (row) {
		if (row.page() != table.page()) {
			table.page(row.page()).draw(false);
		}
		row.node().scrollIntoView();
		toggleTestDetails(suiteData, table, row.node());
	} else {
		console.error("invalid row in scrollToTest:", testIndex);
	}
}

// findRowByTestIndex finds the dataTables row corresponding to a testIndex.
function findRowByTestIndex(table, testIndex) {
	let tests = table.data();
	for (var i = 0; i < tests.length; i++) {
		if (tests[i].testIndex == testIndex) {
			return table.row(i);
		}
	}
	return null;
}

// toggleTestDetails shows/hides the test details panel.
function toggleTestDetails(suiteData, table, tr) {
	let row = table.row(tr);
	if (row.child.isShown()) {
		if (!$(row.node()).hasClass('highlighted')) {
			// When clicking a test that is expanded, but not selected,
			// the click only changes selection.
			selectTest(table, row);
		} else {
			// This test is the selected one, clicking deselects and closes it.
			deselectTest(row, true);
		}
	} else {
		let details = formatTestDetails(suiteData, row.data());
		row.child(details).show();
		$(tr).addClass('shown');
		selectTest(table, row);
	}
}

function selectTest(table, row) {
	let selected = $('#execresults tr.dt-hasChild.highlighted');
	if (selected) {
		let selectedRow = table.row(selected[0]);
		deselectTest(selectedRow, false);
	}
	console.log('select:', row.data().testIndex);
	$(row.node()).addClass('highlighted');
	$(row.child()).addClass('highlighted');
	history.replaceState(null, null, '#test-' + row.data().testIndex);
}

function deselectTest(row, closeDetails) {
	if (closeDetails) {
		row.child.hide();
		$(row.node()).removeClass('shown');
	}
	$(row.node()).removeClass('highlighted');
	$(row.child()).removeClass('highlighted');
	history.replaceState(null, null, '#');
}
