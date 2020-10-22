// The app, more or less. No frameworks neede other than a splash of jquery
var hacks = {
    /*
    makeLine creates an element like the template here:
       <tr>
         <td id="L{{ line }}" class="num" line="{{ line }} "></td>
         <td>{{ content }}</td>
       </tr>
    */
    makeLine: function(number, text) {
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
    },

    // setHL sets the highlight
    setHL: function(num) {
        $(".highlighted").removeClass("highlighted"); // out with the old
        $("#L" + num).parent().addClass("highlighted"); // in with the new
    },

    // human readable units
    units: function(loc) {
        let unit = "B"
        if (loc > 1024) {
            loc = loc / 1024
            unit = " KB"
        }
        if (loc > 1024) {
            loc = loc / 1024
            unit = "MB"
        }
        return loc.toFixed(2) + unit;
    },

    // showText displays a bunch of text on the identified element
    showText: function(domId, bunchaText) {
        let codez = document.getElementById(domId)
        let lines = bunchaText.split("\n")
        for (let i = 0; i < lines.length; i++) {
            let elem = hacks.makeLine(i + 1, lines[i])
            codez.appendChild(elem)
        }
        // Text showing done, now let's wire up the gutter-clicking
        // so if a line number is clicked,
        // 1. Previous highlight is removed
        // 2. The line is highlighted,
        // 3. The id is added to the URL hash
        $(".num").on('click', function(obj) {
            hacks.setHL($(this).attr("line"))
            history.pushState(null, null, "#" + $(this).attr("id"));
        });
        // return LOC
        return lines.length
    },

    // fetchFile loads up a new file to view
    fetchFile: function(line /* optional jump to line */ ) {
        let url = $("#fileload").val()
        $.ajax({
            url: url,
            success: function(data) {
                history.pushState(null, null, "?file=" + url)
                hacks.setContent(data, url)
                hacks.setHL(line)
                if (line) {
                    window.location.hash = "L" + line;
                }
            },
            dataType: "text",
            error: function(jq, status, error) {
                alert("Failed to load " + url + "\nstatus:" + status + "\nerror:" + error)
            },
        });
    },

    // setContent shows a file + fileinfo
    // should be called by the loader, after successfull fetch
    setContent: function(text, filename) {
        document.getElementById("viewer").innerHTML = ""
        nLines = hacks.showText("viewer", text)
        // Set the raw dest
        document.getElementById("raw-url").setAttribute("href", filename)
        // Set meta-info
        let meta = nLines + " Lines, " + hacks.units(text.length)
        document.getElementById("meta").innerText = meta

    },
}

$.when($.ready).then(function() {
    // default text
    hacks.showText("viewer", document.getElementById("exampletext").innerHTML)

    // Check the hash
    let h = window.location.hash
    let num = null
    if (window.location.hash.substr(1, 1) == "L") {
        num = parseInt(window.location.hash.substr(2))
    }

    // Check the query
    let params = new URLSearchParams(location.search);
    if (params) {
        let f = params.get("file");
        if (f) {
            $("#fileload").val(f)
            hacks.fetchFile(num)
            return
        }
    }
});
