export const resultsRoot = "/results/"

// This object has constructor function for various app-internal URLs.
export function simulatorLog(suiteID, suiteName, file) {
	let params = new URLSearchParams({
		"suiteid": suiteID,
		"suitename": suiteName,
		"file": file,
	});
	return "/viewer.html?" + params.toString();
}

export function testLog(suiteID, suiteName, testIndex) {
	let params = new URLSearchParams({
		"suiteid": suiteID,
		"suitename": suiteName,
		"testid": testIndex,
		"showtestlog": "1",
	});
	return "/viewer.html?" + params.toString();
}

export function clientLog(suiteID, suiteName, testIndex, file) {
	let params = new URLSearchParams({
		"suiteid": suiteID,
		"suitename": suiteName,
		"testid": testIndex,
		"file": file,
	});
	return "/viewer.html?" + params.toString();
}

export function suite(suiteID, suiteName) {
	let params = new URLSearchParams({"suiteid": suiteID, "suitename": suiteName});
	return "/suite.html?" + params.toString();
}

export function testInSuite(suiteID, suiteName, testIndex) {
	return suite(suiteID, suiteName) + "#test-" + escape(testIndex);
}
