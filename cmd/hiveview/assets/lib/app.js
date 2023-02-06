import { $ } from '../extlib/jquery.module.js'

export const resultsRoot = "/results/"

// This object has constructor function for various app-internal URLs.
export let route = {
	simulatorLog: function(suiteID, suiteName, file) {
		let params = new URLSearchParams({
			"suiteid": suiteID,
			"suitename": suiteName,
			"file": file,
		});
		return "/viewer.html?" + params.toString();
	},

	testLog: function(suiteID, suiteName, testIndex) {
		let params = new URLSearchParams({
			"suiteid": suiteID,
			"suitename": suiteName,
			"testid": testIndex,
			"showtestlog": "1",
		});
		return "/viewer.html?" + params.toString();
	},

	clientLog: function(suiteID, suiteName, testIndex, file) {
		let params = new URLSearchParams({
			"suiteid": suiteID,
			"suitename": suiteName,
			"testid": testIndex,
			"file": file,
		});
		return "/viewer.html?" + params.toString();
	},

	suite: function(suiteID, suiteName) {
		let params = new URLSearchParams({"suiteid": suiteID, "suitename": suiteName});
		return "/suite.html?" + params.toString();
	},

	testInSuite: function(suiteID, suiteName, testIndex) {
		return route.suite(suiteID, suiteName) + "#test-" + escape(testIndex);
	},
}

export function init() {
	// Update the header.
	$.ajax({
		type: 'GET',
		url: resultsRoot + "hive.json",
		dataType: 'json',
		success: function(data) {
			console.log("hive.json:", data);
			$("#hive-instance-info").html(hiveInfoHTML(data));
		},
		error: function(xhr, status, error) {
			console.log("error fetching hive.json:", error);
		},
	});
}

function hiveInfoHTML(data) {
	var txt = "";
	if (data.buildDate) {
		let date = new Date(data.buildDate).toLocaleString();
		txt += '<span>built: ' + date + '</span>';
	}
	if (data.sourceCommit) {
		let url = "https://github.com/ethereum/hive/commit/" + escape(data.sourceCommit);
		let link = '<a href="' + url + '">' + data.sourceCommit.substring(0, 8) + '</a>';
		txt += '<span>commit: ' + link + '</span>';
	}
	return txt;
}
