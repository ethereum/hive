import $ from 'jquery';

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
        fetchFile(file, line);
        return;
    }

    // Show default text because nothing was loaded.
    showText(document.getElementById('exampletext').innerHTML);
});

// setHL sets the highlight on a line number.
function setHL(num, scroll) {
    // out with the old
    $('.highlighted').removeClass('highlighted');
    if (!num) {
        return;
    }

    let contentArea = document.getElementById('file-content');
    let gutter = document.getElementById('gutter');
    let numElem = gutter.children[num - 1];
    if (!numElem) {
        console.error('invalid line number:', num);
        return;
    }
    // in with the new
    let lineElem = contentArea.children[num - 1];
    $(numElem).addClass('highlighted');
    $(lineElem).addClass('highlighted');
    if (scroll) {
        numElem.scrollIntoView();
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
    line.innerText = text + '\n';
    contentArea.appendChild(line);
}

function lineNumberClicked() {
    setHL($(this).attr('line'), false);
    history.replaceState(null, null, '#' + $(this).attr('id'));
}

// fetchFile loads up a new file to view
async function fetchFile(url, line /* optional jump to line */ ) {
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
    setHL(line, true);
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
