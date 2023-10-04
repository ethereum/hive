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

    let theTable = $('#filetable').DataTable({
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

    const filters = new ColumnFilterSet(theTable);
    filters.build();
    $('#filters-clear').click(function () {
        filters.clear();
        return false;
    });
}

// ColumnFilterSet manages the column filters.
class ColumnFilterSet {
    table = null; // holds the DataTable

    constructor(table) {
        this.table = table;
        this._filters = [
            new DateFilter(this, 0),
            new SuiteFilter(this, 1),
            new ClientFilter(this, 2),
            new StatusFilter(this, 3),
        ];
    }

    // build creates the filters in the table.
    build() {
        // Build header row.
        const ncol = this.table.columns().nodes().length;
        const th = '<th></th>';
        const thead = $('thead', this.table.table);
        $('<tr class="filters">' + th.repeat(ncol) + '</tr>').appendTo(thead);

        // Create select boxes.
        this._selects = {};
        this._filters.forEach(function (f) {
            const sel = f.build();
            this._selects[f.key()] = sel;
        }.bind(this))

        // Apply filters from the URL hash segment.
        const p = new URLSearchParams(window.location.hash.substring(1));
        p.forEach(function (value, key) {
            const f = this.byKey(key);
            if (!f) {
                console.log(`unknown filter ${key} in URL!`);
                return;
            }
            f.apply(value);
            this._selects[key].val(value);
        }.bind(this));
    }

    // clear unsets all filters.
    clear() {
        this._filters.forEach(function (f) {
            f.apply('');
            this._selects[f.key()].val('');
        }.bind(this));
    }

    // filterChanged is called by the filter when their value has changed.
    filterChanged(f) {
        const any = this._filters.some((f) => f.isActive());
        $('#filters-notice').toggle(any);
    }

    // byKey finds a filter by its key.
    byKey(key) {
        return this._filters.find((f) => f.key() == key);
    }
}

// ColumnFilter is an abstract column filter.
class ColumnFilter {
    constructor(controller, columnIndex) {
        this._controller = controller;
        this._columnIndex = columnIndex;
    }

    build() { throw new Error('build() not implemented'); }
    key() { throw new Error('key() not implemented'); }

    // apply filters the table.
    apply(value) {
        const api = this._controller.table;
        if (value !== '') {
            let re = this.valueToRegExp(value);
            console.log(`searching column ${this._columnIndex} with regexp ${re}`);
            api.column(this._columnIndex).search(re, true, false);
        } else {
            // Empty query clears search.
            api.column(this._columnIndex).search('');
        }
        api.draw();

        // Update URL hash segment.
        this.storeToURL(value);

        // Notify controller.
        const changed = value !== this.value;
        this._value = value;
        if (changed) {
            this._controller.filterChanged(this);
        }
    }

    // isActive reports whether the filter is set.
    isActive() {
        return this._value && this._value.length > 0;
    }

    // storeToURL saves the filter value to the URL.
    storeToURL(value) {
        const p = new URLSearchParams(window.location.hash.substring(1))
        if (value !== '') {
            p.set(this.key(), '' + value);
        } else {
            p.delete(this.key());
        }
        window.history.replaceState(null, '', '#' + p.toString());
    }

    // valueToRegExp turns the filter value in to a regular expression for searching.
    valueToRegExp(value) {
        return escapeRegExp(value);
    }

    // buildSelect creates an empty <select> element and adds it to the table header.
    buildSelect() {
        const header = $('.filters th', this._controller.table.table);
        const cell = header.eq(this._columnIndex);

        // Create the select list and search operation
        const _this = this;
        const callback = function () { _this.apply($(this).val()) };
        const select = $('<select />');
        select.on('change', callback);
        select.appendTo(cell);
        select.append($('<option value="">Show all</option>'));
        return select;
    }

    // buildSelectWithColumnValues creates the <select> with <option> values
    // for all values in the filter's table column.
    buildSelectWithOptions() {
        const api = this._controller.table;
        const select = this.buildSelect();
        let options = new Set();

        // Get the search data for the first column and add to the select list
        api.column(this._columnIndex)
           .cache('search')
           .unique()
           .each(function (d) {
               d.split(',').forEach(function (d) {
                   d = d.trim();
                   if (d.length > 0) {
                       options.add(d);
                   }
               });
           });
        Array.from(options.values()).sort().forEach(function (d) {
            select.append($('<option value="'+d+'">'+d+'</option>'));
        });
        return select;
    }
}

// DateFilter is for the date column.
class DateFilter extends ColumnFilter {
    key() { return "daysago"; }

    build() {
        const select = this.buildSelect();
        select.append($('<option value="0">Today</option>'));
        select.append($('<option value="1">Yesterday</option>'));
        select.append($('<option value="2">2 days ago</option>'));
        select.append($('<option value="3">3 days ago</option>'));
        select.append($('<option value="4">4 days ago</option>'));
        select.append($('<option value="5">5 days ago</option>'));
        select.append($('<option value="6">6 days ago</option>'));
        select.append($('<option value="7">7 days ago</option>'));
        return select;
    }

    minusXdays(x) {
        const date = new Date(new Date().setDate(new Date().getDate() - x));
        return date.toLocaleDateString();
    }

    valueToRegExp(x) {
        const date = this.minusXdays(0 + x);
        return escapeRegExp(date);
    }
}

// ClientFilter is for the clients column.
class ClientFilter extends ColumnFilter {
    key() { return "client"; }

    build() {
        return this.buildSelectWithOptions();
    }

    valueToRegExp(value) {
        return '\\b' + escapeRegExp(value) + '\\b'; // anchor match to words
    }
}

// SuiteFilter is for the suite name column.
class SuiteFilter extends ColumnFilter {
    key() { return "suite"; }

    build() {
        return this.buildSelectWithOptions();
    }

    valueToRegExp(value) {
        return '^' + escapeRegExp(value) + '$'; // anchor match to whole field
    }
}

// StatusFilter is for the suite status column.
class StatusFilter extends ColumnFilter {
    key() { return "status"; }

    build() {
        const select = this.buildSelect();
        select.append($('<option value="SUCCESS">SUCCESS</option>'));
        select.append($('<option value="FAIL">FAIL</option>'));
        select.append($('<option value="TIMEOUT">TIMEOUT</option>'));
        return select;
    }

    valueToRegExp(value) {
        if (value === 'SUCCESS') {
            return '✓';
        }
        return escapeRegExp(value);
    }
}
