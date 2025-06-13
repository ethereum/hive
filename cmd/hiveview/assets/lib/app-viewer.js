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

    // Check for line number or line range in hash.
    var startLine = null;
    var endLine = null;
    var hash = window.location.hash.substr(1);
    if (hash.startsWith('L')) {
        var range = hash.substr(1).split('-');
        startLine = parseInt(range[0]);
        if (range.length > 1) {
            endLine = parseInt(range[1]);
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
        fetchTestLog(routes.resultsRoot + suiteFile, testIndex, startLine, endLine);
        return;
    }

    // Check for file name.
    let file = queryParam('file');
    if (file) {
        $('#fileload').val(file);
        showText('Loading file...');
        fetchFile(file, startLine, endLine);
        return;
    }

    // Show default text because nothing was loaded.
    showText(document.getElementById('exampletext').innerHTML);
});

// setHL sets the highlight on a line number or range of lines.
function setHL(startLine, endLine, scroll) {
    // out with the old
    $('.highlighted').removeClass('highlighted');
    if (!startLine) {
        return;
    }

    let contentArea = document.getElementById('file-content');
    let gutter = document.getElementById('gutter');
    
    // Calculate the end line if not provided
    if (!endLine) {
        endLine = startLine;
    }
    
    // Calculate max available lines and adjust range if needed
    const maxLines = gutter.children.length;
    
    // Check if the requested range is beyond the file size
    if (startLine > maxLines) {
        startLine = 1;
    }
    
    if (endLine > maxLines) {
        endLine = maxLines;
    }
    
    // Highlight all lines in the adjusted range
    for (let num = startLine; num <= endLine; num++) {
        let numElem = gutter.children[num - 1];
        if (!numElem) {
            // Skip invalid line numbers
            continue;
        }
        
        let lineElem = contentArea.children[num - 1];
        $(numElem).addClass('highlighted');
        $(lineElem).addClass('highlighted');
    }
    
    // Scroll to the start of the highlighted range
    if (scroll) {
        let firstNumElem = gutter.children[startLine - 1];
        if (firstNumElem) {
            firstNumElem.scrollIntoView();
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
    setHL(parseInt($(this).attr('line')), null, false);
    history.replaceState(null, null, '#' + $(this).attr('id'));
}

// fetchFile loads up a new file to view
async function fetchFile(url, startLine, endLine) {
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
    setHL(startLine, endLine, true);
}

// fetchTestLog loads the suite file and displays the output of a test.
async function fetchTestLog(suiteFile, testIndex, startLine, endLine) {
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
    setHL(startLine, endLine, true);
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
