import $ from 'jquery';
import Prism from '../extlib/prism-1.29.0.min.js';
import '../extlib/prism-bash-1.29.0.min.js';
import '../extlib/prism-json-1.29.0.min.js';

import * as common from './app-common.js';
import * as routes from './routes.js';
import * as testlog from './testlog.js';
import { makeLink } from './html.js';
import { formatBytes, queryParam } from './utils.js';

// Virtual scrolling configuration.
const LINE_HEIGHT = 20;  // Must match CSS
const BUFFER_LINES = 50; // Extra lines to render above/below viewport

// Virtual scrolling state.
let viewerState = {
    lines: [],           // All lines of text
    totalLines: 0,
    highlightStart: 0,   // Highlighted line range (0 = none)
    highlightEnd: 0,
    renderedStart: -1,   // Currently rendered line range
    renderedEnd: -1,
    viewer: null,        // The viewer element for position calculations
};

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
        byteRange = { begin: parseInt(byteBegin), end: parseInt(byteEnd) };
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
    // Update state.
    viewerState.highlightStart = startNum || 0;
    viewerState.highlightEnd = endNum || startNum || 0;

    // Scroll to the highlighted line first (so renderVisibleLines renders the right area).
    if (scroll && startNum && viewerState.viewer) {
        let viewerTop = viewerState.viewer.getBoundingClientRect().top + window.scrollY;
        let contextLines = 5;  // Show a few lines before the highlight for context.
        let targetScroll = viewerTop + Math.max(0, startNum - 1 - contextLines) * LINE_HEIGHT;
        window.scrollTo({ top: targetScroll, behavior: 'smooth' });
    }

    // Re-render to apply highlighting.
    renderVisibleLines();
}

// showLinkBack displays the link to the test viewer.
function showLinkBack(suiteID, suiteName, testID) {
    var text, url;
    if (testID) {
        text = 'Back to test ' + testID + ' in suite \u2018' + suiteName + '\u2019';
        url = routes.testInSuite(suiteID, suiteName, testID);
    } else {
        text = 'Back to test suite \u2018' + suiteName + '\u2019';
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


// showText sets the content of the viewer using virtual scrolling.
function showText(text) {
    let contentArea = document.getElementById('file-content');
    let gutter = document.getElementById('gutter');
    let viewer = document.getElementById('viewer');

    // Parse lines and store in state.
    let lines = text.split('\n');
    // Remove empty last line if file ends with newline.
    if (lines.length > 0 && lines[lines.length - 1] === '') {
        lines.pop();
    }

    viewerState.lines = lines;
    viewerState.totalLines = lines.length;
    viewerState.highlightStart = 0;
    viewerState.highlightEnd = 0;
    viewerState.renderedStart = -1;  // Force initial render
    viewerState.renderedEnd = -1;
    viewerState.viewer = viewer;

    // Clear content.
    contentArea.innerHTML = '';
    gutter.innerHTML = '';

    // Set up virtual scrolling container heights.
    let totalHeight = lines.length * LINE_HEIGHT;
    contentArea.style.height = totalHeight + 'px';
    contentArea.style.position = 'relative';
    contentArea.style.width = '100%';
    contentArea.style.display = 'block';
    gutter.style.height = totalHeight + 'px';
    gutter.style.position = 'relative';

    // Set meta-info.
    let meta = $('#meta');
    meta.text(lines.length + ' Lines, ' + formatBytes(text.length));

    // Ensure viewer is visible.
    $('#viewer-header').show();
    $('#viewer').show();

    // Set up scroll handler on window (page scrolls, not container).
    window.removeEventListener('scroll', onViewerScroll);
    window.addEventListener('scroll', onViewerScroll);

    // Initial render.
    renderVisibleLines();
}

// onViewerScroll handles scroll events to render visible lines.
function onViewerScroll() {
    renderVisibleLines();
}

// renderVisibleLines renders only the lines currently visible in the viewport.
function renderVisibleLines() {
    let viewer = viewerState.viewer;
    if (!viewer || viewerState.totalLines === 0) return;

    let contentArea = document.getElementById('file-content');
    let gutter = document.getElementById('gutter');

    // Get viewer position relative to viewport.
    let viewerRect = viewer.getBoundingClientRect();
    let viewportHeight = window.innerHeight;

    // Calculate which lines are visible.
    let scrollIntoViewer = -viewerRect.top;
    let firstVisible = Math.floor(Math.max(0, scrollIntoViewer) / LINE_HEIGHT);
    let lastVisible = Math.ceil(Math.max(0, scrollIntoViewer + viewportHeight) / LINE_HEIGHT);

    let renderStart = Math.max(0, firstVisible - BUFFER_LINES);
    let renderEnd = Math.min(viewerState.totalLines, lastVisible + BUFFER_LINES);

    // Skip if already rendered this range.
    if (renderStart === viewerState.renderedStart && renderEnd === viewerState.renderedEnd) {
        return;
    }

    // Clear and re-render.
    contentArea.innerHTML = '';
    gutter.innerHTML = '';

    for (let i = renderStart; i < renderEnd; i++) {
        let lineNum = i + 1;
        let isHighlighted = lineNum >= viewerState.highlightStart && lineNum <= viewerState.highlightEnd;
        appendLine(contentArea, gutter, lineNum, viewerState.lines[i], isHighlighted);
    }

    viewerState.renderedStart = renderStart;
    viewerState.renderedEnd = renderEnd;
}

function appendLine(contentArea, gutter, number, text, isHighlighted) {
    let top = (number - 1) * LINE_HEIGHT;

    let num = document.createElement('span');
    num.setAttribute('id', 'L' + number);
    num.setAttribute('class', 'num' + (isHighlighted ? ' highlighted' : ''));
    num.setAttribute('line', number.toString());
    num.style.position = 'absolute';
    num.style.top = top + 'px';
    num.style.right = '0';
    num.style.width = '100%';
    num.addEventListener('click', lineNumberClicked);
    gutter.appendChild(num);

    let line = document.createElement('pre');
    line.className = 'language-log' + (isHighlighted ? ' highlighted' : '');
    line.style.position = 'absolute';
    line.style.top = top + 'px';
    line.style.left = '0';
    line.style.whiteSpace = 'pre';
    line.style.overflow = 'visible';
    line.innerHTML = Prism.highlight(text + '\n', Prism.languages.log || Prism.languages.plaintext, 'log');
    contentArea.appendChild(line);
}

function lineNumberClicked() {
    let lineNum = parseInt($(this).attr('line'));
    setHL(lineNum, false);
    history.replaceState(null, null, '#L' + lineNum);
}

// fetchFile loads up a new file to view.
// When a byteRange is given, only the relevant portion is fetched via HTTP Range
// request (plus a small context buffer), making it fast even for multi-MB log files.
async function fetchFile(url, line, byteRange) {
    let resultsRE = new RegExp('^' + routes.resultsRoot);
    let title = url.replace(resultsRE, '');

    try {
        showRawLink(url);
        if (byteRange) {
            await fetchFileByteRange(url, title, byteRange);
        } else {
            let text = await load(url, 'text');
            showTitle(null, title);
            showText(text);
            setHL(line, true);
        }
    } catch (err) {
        showError(`Failed to load ${url}`, err);
    }
}

// CONTEXT_BYTES is the number of extra bytes to fetch before/after the byte range
// to provide surrounding context lines in the viewer.
const CONTEXT_BYTES = 8192;

// fetchFileByteRange fetches only the relevant portion of a log file using an
// HTTP Range request, then highlights the test's lines within the fetched text.
async function fetchFileByteRange(url, title, byteRange) {
    // Fetch with padding for context lines.
    let fetchBegin = Math.max(0, byteRange.begin - CONTEXT_BYTES);
    let fetchEnd = byteRange.end + CONTEXT_BYTES;  // Server clamps to file size.
    let range = `bytes=${fetchBegin}-${fetchEnd}`;

    let response = await fetch(url, { headers: { 'Range': range } });
    if (!response.ok && response.status !== 206) {
        throw new Error(`HTTP ${response.status} ${response.statusText}`);
    }

    let text = await response.text();

    // Compute highlight line range within the fetched text.
    // The fetched text starts at fetchBegin, so adjust offsets.
    let localBegin = byteRange.begin - fetchBegin;
    let localEnd = byteRange.end - fetchBegin;
    let lineRange = byteOffsetToLineRange(text, localBegin, localEnd);

    showTitle(null, title);
    showText(text);
    setHL(lineRange.start, true, lineRange.end);
}

// byteOffsetToLineRange converts byte offsets (relative to the text) to line numbers.
// The range [begin, end) is treated as exclusive on end.
function byteOffsetToLineRange(text, begin, end) {
    let currentLine = 1, startLine = 1, endLine = 1;
    let foundStart = false;

    for (let i = 0; i < text.length; i++) {
        if (!foundStart && i >= begin) {
            startLine = currentLine;
            foundStart = true;
        }
        if (i >= end) {
            endLine = currentLine > 1 ? currentLine - 1 : 1;
            break;
        }
        if (text.charCodeAt(i) === 0x0A) {
            currentLine++;
        }
    }
    if (!foundStart) {
        return { start: currentLine, end: currentLine };
    }
    if (endLine < startLine) {
        endLine = currentLine;
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
