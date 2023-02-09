import 'bootstrap'
import { $ } from 'jquery'

import * as routes from './routes.js'

export function updateHeader() {
    // Update the header with version info from hive.json.
    $.ajax({
        type: 'GET',
        url: routes.resultsRoot + "hive.json",
        dataType: 'json',
        cache: false,
        success: function(data) {
            console.log("hive.json:", data);
            $("#hive-instance-info").html(hiveInfoHTML(data));
        },
        error: function(xhr, status, error) {
            console.log("error fetching hive.json:", error);
        },
    });
}

function hiveInfoHTML(data) {
    var txt = "";
    if (data.buildDate) {
        let date = new Date(data.buildDate).toLocaleString();
        txt += '<span>built: ' + date + '</span>';
    }
    if (data.sourceCommit) {
        let url = "https://github.com/ethereum/hive/commits/" + escape(data.sourceCommit);
        let link = '<a href="' + url + '">' + data.sourceCommit.substring(0, 8) + '</a>';
        txt += '<span>commit: ' + link + '</span>';
    }
    return txt;
}
