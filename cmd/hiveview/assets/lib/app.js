import { $ } from '../extlib/jquery.module.js'

export const resultsRoot = "/results/"

// This object has constructor function for various app-internal URLs.
export let route = {
	logFileInViewer: function(suiteID, suiteName, file) {
		let params = new URLSearchParams();
		params.set("suiteid", suiteID);
		params.set("suitename", suiteName);
		params.set("file", file);
		return "/viewer.html?" + params.toString();
	},

	testLogInViewer: function(suiteID, suiteName, testIndex) {
		let params = new URLSearchParams();
		params.set("suiteid", suiteID);
		params.set("suitename", suiteName);
		params.set("testid", testIndex);
		params.set("showtestlog", "1");
		return "/viewer.html?" + params.toString();
	},

	suite: function(suiteID, suiteName) {
		let params = new URLSearchParams();
		params.set("suiteid", suiteID);
		params.set("suitename", suiteName);
		return "/suite.html?" + params.toString();
	},

	testInSuite: function(suiteID, suiteName, testIndex) {
		return appRoutes.suite(suiteID, suiteName) + "#test-" + escape(testIndex);
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
	if (data.sourceCommit) {
		let url = "https://github.com/ethereum/hive/commit/" + escape(data.sourceCommit);
		let link = '<a href="' + url + '">' + data.sourceCommit.substring(0, 8) + '</a>';
		txt += '<span>commit: ' + link + '</span>';
	}
	if (data.buildDate) {
		txt += '<span>built: ' + data.buildDate + '</span>';
	}
	return txt;
}
