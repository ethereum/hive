import 'datatables.net';
import 'datatables.net-bs5';
import 'datatables.net-responsive';
import 'datatables.net-responsive-bs5';
import $ from 'jquery';

import * as common from './app-common.js';
import * as routes from './routes.js';
import { makeButton } from './html.js';
import { formatBytes } from './utils.js';

$(document).ready(function () {
    common.updateHeader();

    $('#loading').show();
    console.log('Loading file list...');
    $.ajax({
        type: 'GET',
        url: 'listing.jsonl',
        cache: false,
        success: function(data) {
            $('#page-text').show();
            showFileListing(data);
        },
        failure: function(status, err) {
            alert(err);
        },
        complete: function () {
            $('#loading').hide();
        },
    });
});

function showFileListing(data) {
    console.log('Got file list');
    // the data is jsonlines
    /*
        {
            "fileName": "./1587325327-fa7ec3c7d09a8cfb754097f79df82118.json",
            "name": "Sync test suite",
            "start": "",
            "simLog": "1587325280-00befe48086b1ef74fbb19b9b7d43e4d-simulator.log",
            "passes": 0,
            "fails": 0,
            "size": 435,
            "clients": [],
            "description": "This suite of tests verifies that clients can sync from each...'\n",
            "ntests": 0
    }
    */

    let suites = [];
    data.split('\n').forEach(function(elem) {
        if (!elem) {
            return;
        }
        let suite = JSON.parse(elem);
        suite.start = new Date(suite.start);
        suites.push(suite);
    });

    $('#filetable').DataTable({
        data: suites,
        pageLength: 50,
        autoWidth: false,
        responsive: {
            details: {
                type: 'none',
                display: $.fn.dataTable.Responsive.display.childRowImmediate,
                renderer: function (table, rowIdx, columns) {
                    var output = '<div class="responsive-overflow">';
                    columns.forEach(function (col, i) {
                        if (col.hidden) {
                            output += '<span class="responsive-overflow-col">';
                            output += col.data;
                            output += '</span> ';
                        }
                    });
                    output += '</div>';
                    return output;
                },
            },
        },
        order: [[0, 'desc']],
        columns: [
            {
                title: '🕒',
                data: 'start',
                type: 'date',
                width: '10em',
                render: function(v, type) {
                    if (type === 'display' || type == 'filter') {
                        return v.toLocaleString();
                    }
                    return v.toISOString();
                },
            },
            {
                title: 'Suite',
                data: 'name',
                width: '14em',
            },
            {
                title: 'Clients',
                data: 'clients',
                width: 'auto',
                render: function(data) {
                    return data.join(', ');
                },
            },
            {
                title: 'Status',
                data: null,
                width: '5.5em',
                className: 'suite-status-column',
                render: function(data) {
                    if (data.fails > 0) {
                        let prefix = data.timeout ? 'Timeout' : 'Fail';
                        return '&#x2715; <b>' + prefix + ' (' + data.fails + ' / ' + (data.fails + data.passes) + ')</b>';
                    }
                    return '&#x2713 (' + data.passes + ')';
                },
            },
            {
                title: '',
                data: null,
                width: '8.5em',
                orderable: false,
                render: function(data) {
                    let url = routes.suite(data.fileName, data.name);
                    let loadText = 'Load (' + formatBytes(data.size) + ')';
                    return makeButton(url, loadText).outerHTML;
                },
            },
        ],
    });
}
