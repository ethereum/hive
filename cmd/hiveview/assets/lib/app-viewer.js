import { $ } from '../extlib/jquery.module.js'
import { format } from './utils.js'

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
function setHL(num) {
    $(".highlighted").removeClass("highlighted"); // out with the old
    $("#L" + num).parent().addClass("highlighted"); // in with the new
}

// showText displays a bunch of text on the identified element
function showText(domId, bunchaText) {
    let codez = document.getElementById(domId)
    let lines = bunchaText.split("\n")
    for (let i = 0; i < lines.length; i++) {
        let elem = makeLine(i + 1, lines[i])
        codez.appendChild(elem)
    }
    // Text showing done, now let's wire up the gutter-clicking
    // so if a line number is clicked,
    // 1. Previous highlight is removed
    // 2. The line is highlighted,
    // 3. The id is added to the URL hash
    $(".num").on('click', function(obj) {
        setHL($(this).attr("line"))
        history.pushState(null, null, "#" + $(this).attr("id"));
    });

    // Set meta-info.
    let meta = lines.length + " Lines, " + format.units(bunchaText.length);
    document.getElementById("meta").innerText = meta;
    return lines.length
}

function showSpinner(spin) {
    let spinner = $("#main .loader");
    let spinClasses = "spinner-border spinner-border-sm";
    if (spin) {
        spinner.addClass(spinClasses);
    } else {
        spinner.removeClass(spinClasses);
    }
}

// setContent shows a file + fileinfo
// should be called by the loader, after successfull fetch
function setContent(text, filename) {
    document.getElementById("viewer").innerHTML = "";
    showText("viewer", text);
    document.getElementById("raw-url").setAttribute("href", filename);
}

// fetchFile loads up a new file to view
function fetchFile(line /* optional jump to line */ ) {
    let url = $("#fileload").val()
    showSpinner(true);
    $.ajax({
        url: url,
        dataType: "text",
        success: function(data) {
            showSpinner(false);
            let newsearch = "?file=" + url;
            if (window.location.search != newsearch) {
                history.pushState(null, null, newsearch);
            }
            document.title = url;
            setContent(data, url)
            setHL(line)
            if (line) {
                window.location.hash = "L" + line;
            }
        },
        error: function(jq, status, error) {
            showSpinner(false);
            alert("Failed to load " + url + "\nstatus:" + status + "\nerror:" + error);
        },
    });
}

function navigate() {
    // Check for line number in hash.
    var num = null;
    if (window.location.hash.substr(1, 1) == "L") {
        num = parseInt(window.location.hash.substr(2));
    }
    // Check for file name.
    let params = new URLSearchParams(location.search);
    if (params) {
        let f = params.get("file");
        if (f) {
            $("#fileload").val(f)
            showText("viewer", "Loading file...");
            fetchFile(num);
            return true;
        }
    }
    return false;
}

$(document).ready(function() {
    if (!navigate()) {
        // Show default text because nothing was loaded.
        showText("viewer", document.getElementById("exampletext").innerHTML);
    }
    window.addEventListener('popstate', function() { navigate() });
    $('#fetchFileForm').submit(function () { fetchFile(); return false; });
});
