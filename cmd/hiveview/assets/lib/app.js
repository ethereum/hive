// Pull in dependencies.
import 'datatables.net'
import 'datatables.net-bs5'
import 'datatables.net-responsive'
import 'datatables.net-responsive-bs5'
import 'bootstrap'
import { $ } from 'jquery'

// Pull in app files.
import * as routes from './routes.js'
import { default as index } from './app-index.js'
import { default as suite } from './app-suite.js'
import { default as viewer } from './app-viewer.js'

$(document).ready(function() {
    // Kick off the page main function.
    let pages = { index, suite, viewer };
    let name = $('script[type=module]').attr('data-main');
    pages[name]();

    // Update the header with version info from hive.json.
    $.ajax({
        type: 'GET',
        url: routes.resultsRoot + "hive.json",
        dataType: 'json',
        success: function(data) {
            console.log("hive.json:", data);
            $("#hive-instance-info").html(hiveInfoHTML(data));
        },
        error: function(xhr, status, error) {
            console.log("error fetching hive.json:", error);
        },
    });
})

function hiveInfoHTML(data) {
    var txt = "";
    if (data.buildDate) {
        let date = new Date(data.buildDate).toLocaleString();
        txt += '<span>built: ' + date + '</span>';
    }
    if (data.sourceCommit) {
        let url = "https://github.com/ethereum/hive/commit/" + escape(data.sourceCommit);
        let link = '<a href="' + url + '">' + data.sourceCommit.substring(0, 8) + '</a>';
        txt += '<span>commit: ' + link + '</span>';
    }
    return txt;
}
