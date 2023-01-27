import { $ } from '../extlib/jquery.module.js'
import { html, nav, format, loader, appRoutes } from './utils.js'

const resultsRoot = "/results/"

function navigate() {
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
        fetchTestLog(resultsRoot + suiteFile, testIndex, line);
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

/*
  makeLine creates an element like the template here:
  <tr>
  <td id="L{{ line }}" class="num" line="{{ line }} "></td>
  <td>{{ content }}</td>
  </tr>
*/
function makeLine(number, text) {
    let tr = document.createElement("tr")
    let td1 = document.createElement("td")
    td1.setAttribute("id", "L" + parseInt(number))
    td1.setAttribute("class", "num")
    td1.setAttribute("line", parseInt(number))
    let td2 = document.createElement("td")
    td2.innerText = text
    tr.appendChild(td1);
    tr.appendChild(td2);
    return tr
}

// setHL sets the highlight
function setHL(num, scroll) {
    $(".highlighted").removeClass("highlighted"); // out with the old
    if (num) {
        let el = $("#L" + num);
        if (!el) {
            console.error("invalid line number:", num);
            return;
        }
        el.parent().addClass("highlighted"); // in with the new
        if (scroll) {
            el[0].scrollIntoView();
        }
    }
}

// showLinkBack displays the link to the test viewer.
function showLinkBack(suiteID, suiteName, testID) {
    let linkText = "Back to test suite: " + suiteName;
    var linkURL;
    if (testID) {
        linkURL = appRoutes.testInSuite(suiteID, suiteName, testID);
    } else {
        linkURL = appRoutes.suite(suiteID, suiteName);
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
    let raw = $("raw-url");
    raw.attr("href", filename);
    raw.show();
}

// showText sets the content of the viewer.
function showText(text) {
    let container = document.getElementById("viewer");

    // Clear content.
    container.innerHTML = "";

    // Add the lines.
    let lines = text.split("\n")
    for (let i = 0; i < lines.length; i++) {
        let elem = makeLine(i + 1, lines[i]);
        container.appendChild(elem);
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
    let meta = lines.length + " Lines, " + format.units(text.length);
    document.getElementById("meta").innerText = meta;
    return lines.length
}

// fetchFile loads up a new file to view
function fetchFile(url, line /* optional jump to line */ ) {
    $.ajax({
        xhr: loader.newXhrWithProgressBar,
        url: url,
        dataType: "text",
        success: function(data) {
            let title = url.replace(/\/results\//, '');
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
