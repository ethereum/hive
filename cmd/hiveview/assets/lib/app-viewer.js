import $ from 'jquery';
import Prism from '../extlib/prism-1.29.0.min.js';
import '../extlib/prism-bash-1.29.0.min.js';
import '../extlib/prism-json-1.29.0.min.js';

import * as common from './app-common.js';
import * as routes from './routes.js';
import * as testlog from './testlog.js';
import { makeLink } from './html.js';
import { formatBytes, queryParam } from './utils.js';

$(document).ready(function () {
    common.updateHeader();

    // Check for line number in hash.
    var line = null;
    if (window.location.hash.substr(1, 1) == 'L') {
        line = parseInt(window.location.hash.substr(2));
    }

    // Check for byte range parameters (for multi-test client log highlighting).
    let byteBegin = queryParam('begin');
    let byteEnd = queryParam('end');
    let byteRange = null;
    if (byteBegin !== null && byteEnd !== null) {
        let begin = parseInt(byteBegin, 10);
        let end = parseInt(byteEnd, 10);
        if (!isNaN(begin) && !isNaN(end)) {
            byteRange = { begin, end };
        }
    }

    // Get suite context.
    let suiteFile = queryParam('suiteid');
    let suiteName = queryParam('suitename');
    let testIndex = queryParam('testid');
    if (suiteFile) {
        showLinkBack(suiteFile, suiteName, testIndex);
    }

    // Check if we're supposed to show a test log.
    let showTestLog = queryParam('showtestlog');
    if (showTestLog === '1') {
        if (!suiteFile || !testIndex) {
            showError('Invalid parameters! Missing \'suitefile\' or \'testid\' in URL.');
            return;
        }
        fetchTestLog(routes.resultsRoot + suiteFile, testIndex, line);
        return;
    }

    // Check for file name.
    let file = queryParam('file');
    if (file) {
        $('#fileload').val(file);
        showText('Loading file...');
        fetchFile(file, line, byteRange);
        return;
    }

    // Show default text because nothing was loaded.
    showText(document.getElementById('exampletext').innerHTML);
});

// setHL sets the highlight on a line number or range.
function setHL(startNum, scroll, endNum) {
    // out with the old
    $('.highlighted').removeClass('highlighted');
    if (!startNum) {
        return;
    }

    let contentArea = document.getElementById('file-content');
    let gutter = document.getElementById('gutter');
    endNum = endNum || startNum;  // Single line if no end specified

    for (let num = startNum; num <= endNum; num++) {
        let numElem = gutter.children[num - 1];
        let lineElem = contentArea.children[num - 1];
        if (numElem) $(numElem).addClass('highlighted');
        if (lineElem) $(lineElem).addClass('highlighted');
    }

    if (scroll) {
        let contextLines = 5;  // Show a few lines before the highlight for context.
        let scrollTarget = Math.max(0, startNum - 1 - contextLines);
        if (gutter.children[scrollTarget]) {
            gutter.children[scrollTarget].scrollIntoView({ behavior: 'smooth', block: 'start' });
        }
    }
}

// showLinkBack displays the link to the test viewer.
function showLinkBack(suiteID, suiteName, testID) {
    var text, url;
    if (testID) {
        text = 'Back to test ' + testID + ' in suite ‘' + suiteName + '’';
        url = routes.testInSuite(suiteID, suiteName, testID);
    } else {
        text = 'Back to test suite ‘' + suiteName + '’';
        url = routes.suite(suiteID, suiteName);
    }
    $('#link-back').html(makeLink(url, text));
}

function showTitle(type, title) {
    document.title = title + ' - hive';
    if (type) {
        title = type + ' ' + title;
    }
    $('#file-title').text(title);
}

function showError(text, err) {
    let errtext = text;
    if (err instanceof Error) {
        errtext += `\n${err.name}: ${err.message}`;
    } else if (err) {
        if (err.status) {
            errtext += `\nstatus ${err.status}`;
        } else {
            errtext += `\n${err}`;
        }
    }

    $('#file-title').text('Error');
    showText('Error!\n' + errtext);
}

function showRawLink(url, text) {
    let raw = $('#raw-url');
    raw.attr('href', url);
    if (text) {
        raw.text(text);
    }
    raw.show();
}

// showText sets the content of the viewer.
function showText(text) {
    let contentArea = document.getElementById('file-content');
    let gutter = document.getElementById('gutter');

    // Clear content.
    contentArea.innerHTML = '';
    gutter.innerHTML = '';

    // Add the lines.
    let lines = text.split('\n');
    for (let i = 0; i < lines.length; i++) {
        // Avoid showing empty last line when there is a newline at the end.
        if (i === lines.length-1 && lines[i] == "") {
            break;
        }
        appendLine(contentArea, gutter, i + 1, lines[i]);
    }

    // Set meta-info.
    let meta = $('#meta');
    meta.text(lines.length + ' Lines, ' + formatBytes(text.length));

    // Ensure viewer is visible.
    $('#viewer-header').show();
    $('#viewer').show();
}

function appendLine(contentArea, gutter, number, text) {
    let num = document.createElement('span');
    num.setAttribute('id', 'L' + number);
    num.setAttribute('class', 'num');
    num.setAttribute('line', number.toString());
    num.addEventListener('click', lineNumberClicked);
    gutter.appendChild(num);

    let line = document.createElement('pre');
    line.className = 'language-log';
    line.innerHTML = Prism.highlight(text + '\n', Prism.languages.log || Prism.languages.plaintext, 'log');
    contentArea.appendChild(line);
}

function lineNumberClicked() {
    setHL($(this).attr('line'), false);
    history.replaceState(null, null, '#' + $(this).attr('id'));
}

// fetchFile loads up a new file to view
async function fetchFile(url, line, byteRange) {
    let resultsRE = new RegExp('^' + routes.resultsRoot);
    let text;
    try {
        showRawLink(url);
        text = await load(url, 'text');
    } catch (err) {
        showError(`Failed to load ${url}`, err);
        return;
    }
    let title = url.replace(resultsRE, '');
    showTitle(null, title);
    showText(text);

    // Highlight byte range if provided, otherwise use line number
    if (byteRange) {
        let lineRange = byteRangeToLineNumbers(text, byteRange);
        setHL(lineRange.start, true, lineRange.end);
    } else {
        setHL(line, true);
    }
}

// byteRangeToLineNumbers converts byte offsets to line numbers.
// Encodes text to UTF-8 bytes once, then counts newlines (0x0A) directly.
// The range [begin, end) is exclusive on end, so a trailing newline at
// position end-1 does not extend the highlight into the next line.
function byteRangeToLineNumbers(text, byteRange) {
    const encoder = new TextEncoder();
    const bytes = encoder.encode(text);

    let startLine = 1, endLine = 1, foundStart = false;
    const newlineByte = 0x0A;  // '\n' in UTF-8

    for (let bytePos = 0; bytePos < bytes.length && bytePos < byteRange.end; bytePos++) {
        if (!foundStart && bytePos >= byteRange.begin) {
            startLine = endLine;
            foundStart = true;
        }
        if (bytes[bytePos] === newlineByte) {
            endLine++;
        }
    }
    // If end falls right after a newline, the line counter has already been
    // bumped to the next line which belongs to the following test. Back up.
    if (byteRange.end > 0 && byteRange.end <= bytes.length && bytes[byteRange.end - 1] === newlineByte) {
        endLine = Math.max(startLine, endLine - 1);
    }
    return { start: startLine, end: endLine };
}

// fetchTestLog loads the suite file and displays the output of a test.
async function fetchTestLog(suiteFile, testIndex, line) {
    let data;
    try {
        data = await load(suiteFile, 'json');
    } catch(err) {
        showError(`Can't load suite file: ${suiteFile}`, err);
        return;
    }
    if (!data['testCases'] || !data['testCases'][testIndex]) {
        showError('Invalid test data returned by server: ' + JSON.stringify(data, null, 2));
        return;
    }

    let test = data.testCases[testIndex];
    let name = test.name;
    let logtext;
    if (test.summaryResult.details) {
        logtext = test.summaryResult.details;
    } else if (test.summaryResult.log) {
        try {
            let url = routes.resultsRoot + data.testDetailsLog;
            let loader = new testlog.Loader(url, test.summaryResult.log);
            showRawLink(url, 'raw suite output');
            logtext = await loader.text(function (received, length) {
                common.showLoadProgress(received/length);
            });
            common.showLoadProgress(false);
        } catch(err) {
            showError('Loading test log failed.', err);
            return;
        }
    } else {
        showError('test has no details/log');
    }
    showTitle('Test:', name);
    showText(logtext);
    setHL(line, true);
}

async function load(url, dataType) {
    return $.ajax({url, dataType, xhr: common.newXhrWithProgressBar});
}

// Add log language definition for Prism
Prism.languages.log = {
    'info': /(?:^\[INFO\]|\bINFO\b).*/m,
    'warn': /(?:^\[WARN\]|\bWARN\b).*/m,
    'error': /(?:^\[ERROR\]|\bERROR\b).*/m,
    'debug': /(?:^\[DEBUG\]|\bDEBUG\b).*/m,
    'timestamp': /\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z?/,
    'number': /\b\d+\b/,
    'string': /"[^"]*"/,
    'path': /(?:\/[\w.-]+)+/,
    'function': /\b\w+(?=\()/,
    'hexcode': /0x[a-fA-F0-9]+/,
    'boolean': /\b(?:true|false)\b/,
    'ip': /\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b/,
    'important': /\[[A-Z]+\]/
};
