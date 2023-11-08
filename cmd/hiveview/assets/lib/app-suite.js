import 'datatables.net';
import 'datatables.net-bs5';
import 'datatables.net-responsive';
import 'datatables.net-responsive-bs5';
import $ from 'jquery';

import * as common from './app-common.js';
import * as routes from './routes.js';
import * as html from './html.js';
import * as testlog from './testlog.js';
import { formatDuration, queryParam } from './utils.js';

$(document).ready(function () {
    common.updateHeader();

    let name = queryParam('suitename');
    if (name) {
        showSuiteName(name);
    }
    let filename = queryParam('suiteid');
    if (!filename) {
        showError('no suite ID in URL');
        return;
    }
    var testid = null;
    if (window.location.hash.match(/^#test-/)) {
        testid = parseInt(window.location.hash.replace(/^#test-/, ''));
    }

    console.log('Loading:', filename, 'name:', name);
    $.ajax({
        xhr: common.newXhrWithProgressBar,
        type: 'GET',
        url: routes.resultsRoot + filename,
        dataType: 'json',
        success: function(suiteData) {
            showSuiteData(suiteData, filename);
            if (testid) {
                scrollToTest(suiteData, testid);
            }
        },
        error: function(xhr, status, error) {
            showError('error fetching ' + filename + ': ' + error);
        },
    });
});

// showSuiteName displays the suite title.
function showSuiteName(name) {
    $('#testsuite_name').text(name);
    document.title = name + ' - hive';
}

function showError(message) {
    console.error(message);
    $('#testsuite_desc').text('Error: ' + message);
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
    $('#testsuite_desc').html(html.urlsToLinks(html.encode(data.description)));

    // Set client versions.
    if (data.clientVersions) {
        // Remove empty version strings.
        for (let key in data.clientVersions) {
            if (!data.clientVersions[key]) {
                delete data.clientVersions[key];
            }
        }
        $('#testsuite_clients').html(html.makeDefinitionList(data.clientVersions));
    }

    // Convert test cases to list.
    let cases = [];
    for (var k in data.testCases) {
        let tc = data.testCases[k];
        tc['testIndex'] = k;
        tc['duration'] = testCaseDuration(tc);
        cases.push(tc);
    }
    console.log('got ' + cases.length + ' testcases');

    // Fill info box.
    let suiteTimes = testSuiteTimes(cases);
    $('#testsuite_start').html('🕒 ' + suiteTimes.start.toLocaleString());
    $('#testsuite_duration').html('⌛️ ' + formatDuration(suiteTimes.duration));
    let logfile = routes.resultsRoot + data.simLog;
    let url = routes.simulatorLog(suiteID, suiteName, logfile);
    $('#sim-log-link').attr('href', url);
    $('#sim-log-link').text('simulator log');
    $('#testsuite_info').show();

    // Initialize the DataTable.
    let table = $('#execresults').DataTable({
        data: cases,
        pageLength: 100,
        autoWidth: false,
        responsive: {
            // Turn off display of hidden columns because it conflicts with our own use of
            // child rows. Display of hidden columns is handled in formatTestDetails.
            details: {
                type: 'none',
                display: function (row, update, render) {},
            },
        },
        order: [[1, 'desc']],
        columns: [
            {
                title: 'Test',
                data: 'name',
                name: 'name',
                className: 'test-name-column',
                width: '65%',
                responsivePriority: 0,
            },
            // Status: pass or not.
            {
                title: 'Status',
                data: 'summaryResult',
                className: 'test-status-column',
                name: 'status',
                width: '4em',
                responsivePriority: 0,
                render: formatTestStatus,
            },
            // Test duration.
            {
                title: '⌛️',
                data: 'duration',
                className: 'test-duration-column',
                name: 'duration',
                width: '6em',
                responsivePriority: 2,
                type: 'num',
                render: function (v, type, row) {
                    if (type === 'display' || type === 'filter') {
                        return formatDuration(v);
                    }
                    return v;
                },
            },
            // The logs for clients related to the test.
            {
                title: 'Logs',
                name: 'logs',
                data: 'clientInfo',
                width: '20%',
                responsivePriority: 1,
                render: function (clientInfo, type, row) {
                    return formatClientLogsList(data, row.testIndex, clientInfo);
                }
            },
        ],
        rowCallback: function(row, data, displayNum, displayIndex, dataIndex) {
            if (!cases[dataIndex].summaryResult.pass) {
                row.classList.add('failed');
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
    };
}

// testCaseDuration computes the duration of a single test case in milliseconds.
function testCaseDuration(test) {
    return Date.parse(test.end) - Date.parse(test.start);
}

// scrollToTest scrolls to the given test row index.
function scrollToTest(suiteData, testIndex) {
    let table = $('#execresults').dataTable().api();
    let row = findRowByTestIndex(table, testIndex);
    if (!row) {
        console.error('invalid row in scrollToTest:', testIndex);
        return;
    }
    if (row.page() != table.page()) {
        table.page(row.page()).draw(false);
    }
    row.node().scrollIntoView();
    toggleTestDetails(suiteData, table, row.node());
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
        let details = formatTestDetails(suiteData, row);
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

function testHasClients(testData) {
    return testData.clientInfo && Object.getOwnPropertyNames(testData.clientInfo).length > 0;
}

// formatClientLogsList turns the clientInfo part of a test into a list of links.
function formatClientLogsList(suiteData, testIndex, clientInfo) {
    let links = [];
    for (let instanceID in clientInfo) {
        let instanceInfo = clientInfo[instanceID];
        let logfile = routes.resultsRoot + instanceInfo.logFile;
        let url = routes.clientLog(suiteData.suiteID, suiteData.name, testIndex, logfile);
        let link = html.makeLink(url, instanceInfo.name);
        link.classList.add('log-link');
        links.push(link.outerHTML);
    }
    return links.join(', ');
}

function formatTestStatus(summaryResult) {
    if (summaryResult.pass) {
        return '&#x2713';
    }
    let s = summaryResult.timeout ? 'Timeout' : 'Fail';
    return '&#x2715; <b>' + s + '</b>';
}

// formatting function for the test 'details box' - this is called when a test is opened.
// `row` is the DataTables row.
function formatTestDetails(suiteData, row) {
    let d = row.data();

    let container = document.createElement('div');
    container.classList.add('details-box');

    // Display columns hidden by the Responsive addon.
    // Gotta do that here because they'll just be hidden otherwise.
    // Values shown here won't be un-displayed if the table width changes.
    // Note: responsiveHidden() returns false when the column is hidden!
    if (!row.column('status:name').responsiveHidden()) {
        let p = document.createElement('p');
        p.innerHTML = formatTestStatus(d.summaryResult);
        container.appendChild(p);
    }
    if (!row.column('logs:name').responsiveHidden() && testHasClients(d)) {
        let p = document.createElement('p');
        p.innerHTML = '<b>Clients:</b> ' + formatClientLogsList(suiteData, d.testIndex, d.clientInfo);
        container.appendChild(p);
    }
    if (!row.column('duration:name').responsiveHidden()) {
        let p = document.createElement('p');
        p.innerHTML = '<b>Duration:</b> ' + formatDuration(d.duration);
        container.appendChild(p);
    }

    if (d.description != '') {
        let p = document.createElement('p');
        let description = html.urlsToLinks(html.encode(d.description.trim()));
        let txt = '<b>Description:</b><br/>' + description;
        p.innerHTML = txt;
        container.appendChild(p);
    }

    if (d.summaryResult.details) {
        // Test output is contained directly in the test, so it can just be displayed.
        // In order to avoid freezing the browser with lots of output, we limit the display to
        // at most 25 lines from the head and tail.
        let log = testlog.splitHeadTail(d.summaryResult.details, 25);
        formatTestLog(suiteData, d.testIndex, log, container);
    } else if (d.summaryResult.log) {
        // Test output is stored in a separate file, so we need to load that here.
        // The .log field contains the offsets into that file, it's an object
        // like {begin: 732, end: 812}.
        let spinner = $('<div><div class="spinner-grow text-secondary" role="status"></div>');
        $(container).append(spinner);

        const testlogMaxLines = 25;
        const testlogMaxBytes = 2097152;

        let url = routes.resultsRoot + suiteData.testDetailsLog;
        let loader = new testlog.Loader(url, d.summaryResult.log);
        loader.headAndTailLines(testlogMaxLines, testlogMaxBytes).then(function (log) {
            spinner.remove();
            formatTestLog(suiteData, d.testIndex, log, container);
        }).catch(function (error) {
            console.error(error);
            spinner.remove();
            let p = document.createElement('p');
            p.innerHTML = highlightErrorsInTestOutput(error.toString());
            container.appendChild(p);
        });
    } else {
        $(container).append('<b>Details:</b> Test has no log output.');
    }

    return container;
}

// formatTestLog formats the test output.
// logData is an object like { head: "...", tail: "...", hiddenLines: 10 }.
function formatTestLog(suiteData, testIndex, logData, container) {
    let p = document.createElement('p');
    p.innerHTML = '<b>Details:</b>';
    container.appendChild(p);

    // Create the output sections.
    let output = document.createElement('div');
    output.classList.add('test-output');

    if (logData.head.length > 0) {
        // Add the beginning of text.
        let el = document.createElement('code');
        el.classList.add('output-prefix');
        if (logData.tail.length == 0) {
            el.classList.add('output-suffix');
        }
        el.innerHTML = formatTestDetailLines(logData.head);
        output.appendChild(el);
    }

    if (logData.tail.length > 0) {
        // Create the truncation marker.
        var linkText;
        if (logData.hiddenLines) {
            linkText = '' + logData.hiddenLines + ' lines hidden. Click here to see full log.';
        } else {
            linkText = 'Output truncated. Click here to see full log.';
        }
        let linkURL = routes.testLog(suiteData.suiteID, suiteData.name, testIndex);
        let trunc = html.makeLink(linkURL, linkText);
        trunc.classList.add('output-trunc');
        output.appendChild(trunc);

        // Add the remaining text.
        let el = document.createElement('code');
        el.classList.add('output-suffix');
        el.innerHTML = formatTestDetailLines(logData.tail);
        output.appendChild(el);
    }

    container.appendChild(output);
}

function formatTestDetailLines(lines) {
    return lines.reduce(function (o, line) {
        return o + highlightErrorsInTestOutput(html.encode(line));
    }, '');
}

function highlightErrorsInTestOutput(content) {
    let p = /\b(error:|fail(ed)?|can't launch node)\b/i;
    if (p.test(content)) {
        return '<span class="output-error">' + content + '</span>';
    }
    return content;
}
