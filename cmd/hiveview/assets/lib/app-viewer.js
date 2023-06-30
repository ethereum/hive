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

function showError(text) {
    $('#file-title').text('Error');
    showText('Error:\n\n' + text);
}

// showFileContent shows a file + fileinfo.
// This is called by the loader, after a successful fetch.
function showFileContent(text, filename) {
    showText(text);
    let raw = $('#raw-url');
    raw.attr('href', filename);
    raw.show();
}

// showText sets the content of the viewer.
function showText(text) {
    let viewer = new TextViewer();
    viewer.setText(text);
}

class TextViewer {
    number = 1;

    constructor(container) {
        this.header = $('#viewer-header');
        this.container = $('#viewer');
        this.contentArea = $('.file-content', this.container);
        this.gutter = $('gutter', this.container);
    }

    // clear removes all text from the view.
    clear() {
        this.contentArea.innerHTML = '';
        this.gutter.innerHTML = '';
        this.number = 1;
    }

    // show ensures the viewer is visible.
    show() {
        this.header.show();
        this.container.show();
    }

    // setText displays the given text.
    setText(text) {
        this.clear();
        let lines = text.split('\n');
        for (let i = 0; i < lines.length; i++) {
            this.appendLine(lines[i]);
        }
        let meta = $('#meta');
        meta.text(lines.length + ' Lines, ' + formatBytes(text.length));
    }

    // highlight sets the highlight on a line number.
    highlight(lineNumber, scroll) {
        // out with the old
        $('.highlighted', this.container).removeClass('highlighted');
        if (!num) {
            return;
        }

        let numElem = this.gutter.children[num - 1];
        if (!numElem) {
            console.error('invalid line number:', num);
            return;
        }
        // in with the new
        let lineElem = this.contentArea.children[num - 1];
        $(numElem).addClass('highlighted');
        $(lineElem).addClass('highlighted');
        if (scroll) {
            numElem.scrollIntoView();
        }
    }

    lineNumberClicked() {
        setHL($(this).attr('line'), false);
        history.replaceState(null, null, '#' + $(this).attr('id'));
    }
    
    appendLine(text) {
        let num = document.createElement('span');
        num.setAttribute('id', 'L' + this.number);
        num.setAttribute('class', 'num');
        num.setAttribute('line', this.number.toString());
        num.addEventListener('click', lineNumberClicked);
        this.gutter.appendChild(num);
        this.number += 1;

        let line = document.createElement('pre');
        line.innerText = text + '\n';
        this.contentArea.appendChild(line);
    }
}

// fetchFile loads up a new file to view
function fetchFile(url, line /* optional jump to line */ ) {
    let resultsRE = new RegExp('^' + routes.resultsRoot);
    $.ajax({
        xhr: common.newXhrWithProgressBar,
        url: url,
        dataType: 'text',
        success: function(data) {
            let title = url.replace(resultsRE, '');
            showTitle(null, title);
            showFileContent(data, url);
            setHL(line, true);
        },
        error: function(jq, status, error) {
            alert('Failed to load ' + url + '\nstatus:' + status + '\nerror:' + error);
        },
    });
}

// fetchTestLog loads the suite file and displays the output of a test.
function fetchTestLog(suiteFile, testIndex, lineHighlight) {
    $.ajax({
        xhr: common.newXhrWithProgressBar,
        url: suiteFile,
        dataType: 'json',
        success: function(data) {
            if (!data['testCases'] || !data['testCases'][testIndex]) {
                let errtext = 'Invalid test data returned by server: ' + JSON.stringify(data, null, 2);
                showError(errtext);
                return;
            }
            let test = data.testCases[testIndex];
            streamTestLogLines(suiteFile, test, lineHighlight);
        },
        error: function(jq, status, error) {
            alert('Failed to load ' + suiteFile + '\nstatus:' + status + '\nerror:' + error);
        },
    });
}

function streamTestLogLines(suiteFile, testCase, lineToHighlight) {
    let name = testCase.name;
    showTitle('Test:', name);

    if (!testCase.logOffsets) {
        let logtext = testCase.summaryResult.details;
        showText(logtext);
        setHL(lineToHighlight, true);
        return;
    }

    // Log is stored in a separate file.
    let logFile = suiteFile += "-testlog.txt";
    let loader = new testlog.LogLoader(logFile, testCase.logOffsets);
    let viewer = new TextViewer();
    viewer.clear();
    viewer.show();
    loader.iterLines(function (line, offsets) {
        viewer.appendLine(line);
        return true;
    });
}
