import { $ } from '../extlib/jquery.module.js'
import { html, nav, format, loader } from './utils.js'
import * as app from './app.js'

function navigate() {
    app.init();

    // Check for line number in hash.
    var line = null;
    if (window.location.hash.substr(1, 1) == "L") {
        line = parseInt(window.location.hash.substr(2));
    }

    // Get suite context.
    let suiteFile = nav.load("suiteid");
    let suiteName = nav.load("suitename");
    let testIndex = nav.load("testid");
    if (suiteFile) {
        showLinkBack(suiteFile, suiteName, testIndex);
    }

    // Check if we're supposed to show a test log.
    let showTestLog = nav.load("showtestlog");
    if (showTestLog === "1") {
        if (!suiteFile || !testIndex) {
            showError("Invalid parameters! Missing 'suitefile' or 'testid' in URL.");
            return;
        }
        fetchTestLog(app.resultsRoot + suiteFile, testIndex, line);
        return;
    }

    // Check for file name.
    let file = nav.load("file");
    if (file) {
        $("#fileload").val(file);
        showText("Loading file...");
        fetchFile(file, line);
        return;
    }

    // Show default text because nothing was loaded.
    showText(document.getElementById("exampletext").innerHTML);
}

$(document).ready(navigate);

// setHL sets the highlight on a line number.
function setHL(num, scroll) {
    // out with the old
    $(".highlighted").removeClass("highlighted");
    if (!num) {
        return;
    }

    let contentArea = $('#file-content');
    let gutter = $('#gutter');
    let numElem = gutter.children().eq(num - 1);
    if (!numElem) {
        console.error("invalid line number:", num);
        return;
    }
    // in with the new
    let lineElem = contentArea.children().eq(num - 1);
    numElem.addClass("highlighted");
    lineElem.addClass("highlighted");
    if (scroll) {
        numElem[0].scrollIntoView();
    }
}

// showLinkBack displays the link to the test viewer.
function showLinkBack(suiteID, suiteName, testID) {
    let linkText = "Back to test suite: " + suiteName;
    var linkURL;
    if (testID) {
        linkURL = app.route.testInSuite(suiteID, suiteName, testID);
    } else {
        linkURL = app.route.suite(suiteID, suiteName);
    }
    $('#link-back').html(html.get_link(linkURL, linkText));
}

function showTitle(type, title) {
    document.title = title + " - hive";
    if (type) {
        title = type + ' ' + title;
    }
    $("#file-title").text(title);
}

function showError(text) {
    $("#file-title").text("Error");
    showText("Error:\n\n" + text);
}

// showFileContent shows a file + fileinfo.
// This is called by the loader, after a successful fetch.
function showFileContent(text, filename) {
    showText(text);
    let raw = $("#raw-url");
    raw.attr("href", filename);
    raw.show();
}

// showText sets the content of the viewer.
function showText(text) {
    let contentArea = document.getElementById("file-content");
    let gutter = document.getElementById("gutter");

    // Clear content.
    contentArea.innerHTML = "";
    gutter.innerHTML = "";

    // Add the lines.
    let lines = text.split("\n")
    for (let i = 0; i < lines.length; i++) {
        appendLine(contentArea, gutter, i + 1, lines[i]);
    }

    // Text showing done, now let's wire up the gutter-clicking
    // so if a line number is clicked,
    // 1. Previous highlight is removed
    // 2. The line is highlighted,
    // 3. The id is added to the URL hash
    $(".num").on('click', function(obj) {
        setHL($(this).attr("line"), false);
        history.replaceState(null, null, "#" + $(this).attr("id"));
    });

    // Set meta-info.
    let meta = $("#meta");
    meta.text(lines.length + " Lines, " + format.units(text.length));

    // Ensure viewer is visible.
    $('#viewer-header').show();
    $('#viewer').show();
}

function appendLine(contentArea, gutter, number, text) {
    let num = document.createElement("span");
    num.setAttribute("id", "L" + number);
    num.setAttribute("class", "num");
    num.setAttribute("line", number.toString());
    gutter.appendChild(num);

    let line = document.createElement("pre")
    line.innerText = text + "\n";
    contentArea.appendChild(line);
}

// fetchFile loads up a new file to view
function fetchFile(url, line /* optional jump to line */ ) {
    let resultsRE = new RegExp("^" + app.resultsRoot);
    $.ajax({
        xhr: loader.newXhrWithProgressBar,
        url: url,
        dataType: "text",
        success: function(data) {
            let title = url.replace(resultsRE, '');
            showTitle(null, title);
            showFileContent(data, url);
            setHL(line, true);
        },
        error: function(jq, status, error) {
            alert("Failed to load " + url + "\nstatus:" + status + "\nerror:" + error);
        },
    });
}

// fetchTestLog loads the suite file and displays the output of a test.
function fetchTestLog(suiteFile, testIndex, line) {
    $.ajax({
        xhr: loader.newXhrWithProgressBar,
        url: suiteFile,
        dataType: "json",
        success: function(data) {
            if (!data['testCases'] || !data['testCases'][testIndex]) {
                let errtext = "Invalid test data returned by server: " + JSON.stringify(data, null, 2);
                showError(errtext);
                return
            }

            let test = data.testCases[testIndex];
            let name = test.name;
            let logtext = test.summaryResult.details;
            showTitle('Test:', name);
            showText(logtext);
            setHL(line, true);
        },
        error: function(jq, status, error) {
            alert("Failed to load " + suiteFile + "\nstatus:" + status + "\nerror:" + error);
        },
    });
}
