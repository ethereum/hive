import '../extlib/bootstrap.module.js'
import '../extlib/jquery.dataTables.module.js'
import { $ } from '../extlib/jquery.module.js'
import { html, nav, loader, appRoutes } from './utils.js'

const resultsRoot = "/results/"

$(document).ready(function () {
	let name = nav.load("suitename");
	if (name) {
		showSuiteName(name);
	}
	let filename = nav.load("suiteid");
	if (!filename) {
		showError("no suite ID in URL");
		return;
	}
	if (window.location.hash.match(/^#test-/)) {
		var testid = parseInt(window.location.hash.replace(/^#test-/, ''));
	}

	console.log("Loading:", filename, "name:", name);
	$.ajax({
		xhr: loader.newXhrWithProgressBar,
		type: 'GET',
		url: resultsRoot + filename,
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
		let descP = document.createElement("p")
		let txt = "<b>Description</b><br/>" + html.urls_to_links(html.encode(d.description));
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
	const maxLines = 30;

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
			let content = html.urls_to_links(html.encode(line));
			if (inPrefix) {
				prefixOutput += content + "\n";
			} else {
				suffixOutput += content + "\n";
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
		let linkText = "... " + hiddenLines + " lines hidden, click to see full output...";
		let linkURL = appRoutes.testLogInViewer(suiteData.suiteID, suiteData.name, test.testIndex);
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

// showSuiteName displays the suite title.
function showSuiteName(name) {
	$("#testsuite_name").text(name);
	document.title = name + " - hive";
}

// showSuiteData displays the suite and its tests in the table.
// This is called after loading the suite.
function showSuiteData(data, filename) {
	let suiteID = filename;
	let suiteName = data.name;
	data['suiteID'] = filename;

	// data structure of suite data:
	/*
	data = {
		"id": 0,
		"name": "Devp2p discovery v4 test suite",
		"description": "This suite of tests checks...",
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
		cases.push(tc);
	}
	console.log("got " + cases.length + " testcases");

	// Initialize the DataTable.
	let table = $('#execresults').DataTable({
		data: cases,
		pageLength: 100,
		autoWidth: false,
		order: [[1, 'desc']],
		columns: [
			// The test name.
			{
				title: "Test",
				data: "name",
				className: "test-name-column",
				width: "79%",
			},
			// Status: pass or not
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
			// The logs for clients related to the test.
			{
				title: "Logs",
				data: "clientInfo",
				render: function(clientInfo) {
					let logs = []
					for (let instanceID in clientInfo) {
						let instanceInfo = clientInfo[instanceID]
						let logfile = resultsRoot + instanceInfo.logFile;
						let url = appRoutes.logFileInViewer(suiteID, suiteName, logfile);
						let link = html.get_link(url, instanceInfo.name)
						logs.push(link.outerHTML);
					}
					return logs.join(", ")
				},
				width: "19%",
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

// toggleTestDetails shows/hides the test details panel.
function toggleTestDetails(suiteData, table, tr) {
	let row = table.row(tr);
	if (row.child.isShown()) {
		row.child.hide();
		$(tr).removeClass('shown');
	} else {
		let details = formatTestDetails(suiteData, row.data());
		row.child(details).show();
		$(tr).addClass('shown');

		// Set test reference in URL.
		history.replaceState(null, null, '#test-' + row.data().testIndex);
	}
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
