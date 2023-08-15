import 'datatables.net';
import 'datatables.net-bs5';
import 'datatables.net-responsive';
import 'datatables.net-responsive-bs5';
import $ from 'jquery';

import * as common from './app-common.js';
import * as routes from './routes.js';
import { makeButton } from './html.js';
import { formatBytes, escapeRegExp } from './utils.js';

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
        initComplete: function () {
            const api = this.api();
            $('<tr class="filters"><th></th><th></th><th></th><th></th><th></th></tr>')
                .appendTo($('#filetable thead'));
            dateSelect(api, 0);
            selectWithOptions(api, 1, true);
            selectWithOptions(api, 2, false);
            statusSelect(api, 3);
        }
    });
}

function genericSelect(api, colIdx, anchoredMatch) {
    const table = $('#filetable').DataTable();

    const cell = $('.filters th').eq(
        $(api.column(colIdx).header()).index()
    );

    // Create the select list and search operation
    const select = $('<select />')
        .appendTo(cell)
        .on('change', function () {
            let re = escapeRegExp($(this).val());
            if (re !== '') {
                if (anchoredMatch) {
                    re = '^' + re + '$';
                } else {
                    re = '\\b' + re + '\\b';
                }
                console.log(`searching column ${colIdx} with regexp ${re}`);
                table.column(colIdx).search(re, true, false);
            } else {
                // Empty query clears search.
                table.column(colIdx).search('');
            }
            table.draw();
        });

    select.append($('<option value="">Show all</option>'));
    return select;
}

function selectWithOptions(api, colIdx, anchoredMatch) {
    const table = $('#filetable').DataTable();
    const select = genericSelect(api, colIdx, anchoredMatch);
    let added = {};

    // Get the search data for the first column and add to the select list
    table
        .column(colIdx)
        .cache('search')
        .sort()
        .unique()
        .each(function (d) {
            d.split(',').forEach(function (d) {
                d = d.trim();
                if (added[d]) {
                    return;
                }
                added[d] = true;
                select.append($('<option value="'+d+'">'+d+'</option>'));
            });
        });
}

function statusSelect(api, colIdx) {
    const select = genericSelect(api, colIdx);
    select.append($('<option value="✓">SUCCESS</option>'));
    select.append($('<option value="FAIL">FAIL</option>'));
    select.append($('<option value="TIMEOUT">TIMEOUT</option>'));
}

function minusXDaysDate(x) {
    const date = new Date(new Date().setDate(new Date().getDate() - x))
    return date.toLocaleDateString()
}

function dateSelect(api, colIdx) {
    const select = genericSelect(api, colIdx);
    const today = new Date().toLocaleDateString();
    select.append($('<option value="' + today + '">Today</option>'));
    select.append($('<option value="' + minusXDaysDate(1) + '">Yesterday</option>'));
    select.append($('<option value="' + minusXDaysDate(2) + '">2 days ago</option>'));
    select.append($('<option value="' + minusXDaysDate(3) + '">3 days ago</option>'));
    select.append($('<option value="' + minusXDaysDate(4) + '">4 days ago</option>'));
    select.append($('<option value="' + minusXDaysDate(5) + '">5 days ago</option>'));
    select.append($('<option value="' + minusXDaysDate(6) + '">6 days ago</option>'));
    select.append($('<option value="' + minusXDaysDate(7) + '">7 days ago</option>'));
}
