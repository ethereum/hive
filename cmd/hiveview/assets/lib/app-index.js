import 'datatables.net';
import 'datatables.net-bs5';
import 'datatables.net-responsive';
import 'datatables.net-responsive-bs5';
import $ from 'jquery';
import * as bootstrap from 'bootstrap';

import * as common from './app-common.js';
import * as routes from './routes.js';
import { makeButton } from './html.js';
import { formatBytes, escapeRegExp } from './utils.js';

function timeSince(date) {
    const seconds = Math.floor((new Date() - date) / 1000);

    const intervals = {
        year: 31536000,
        month: 2592000,
        week: 604800,
        day: 86400,
        hour: 3600,
        minute: 60
    };

    for (const [unit, secondsInUnit] of Object.entries(intervals)) {
        const interval = Math.floor(seconds / secondsInUnit);
        if (interval >= 1) {
            return interval === 1 ? `1 ${unit}` : `${interval} ${unit}s`;
        }
    }

    return 'just now';
}

window.sortAllClients = function(sortBy) {
    $('.suite-box').each(function() {
        const clientBoxes = $(this).find('.client-box').get();

        clientBoxes.sort((a, b) => {
            const boxA = $(a);
            const boxB = $(b);

            switch(sortBy) {
                case 'name':
                    return boxA.data('client').localeCompare(boxB.data('client'));
                case 'coverage':
                    const coverageA = parseInt(boxA.find('.coverage-percent').text());
                    const coverageB = parseInt(boxB.find('.coverage-percent').text());
                    return coverageB - coverageA; // Higher coverage first
                case 'time':
                    const timeA = new Date(boxA.data('time'));
                    const timeB = new Date(boxB.data('time'));
                    return timeB - timeA; // Most recent first
                default:
                    return 0;
            }
        });

        const clientResults = $(this).find('.client-results');
        clientResults.empty();
        clientBoxes.forEach(box => clientResults.append(box));
    });

    // Update URL hash with sort parameter while preserving other parameters
    const urlParams = new URLSearchParams(window.location.hash.substring(1));
    urlParams.set('summary-sort', sortBy);
    const newHash = urlParams.toString();
    if (newHash) {
        window.history.replaceState(null, '', '#' + newHash);
    }

    // Update dropdown button text
    const sortText = {
        'name': 'Name',
        'coverage': 'Coverage',
        'time': 'Time'
    }[sortBy];
    $('.summary-controls .current-sort').text(sortText);
};

$(document).ready(function () {
    // Initialize popovers
    const popoverTriggerList = document.querySelectorAll('[data-bs-toggle="popover"]');
    [...popoverTriggerList].map(el => new bootstrap.Popover(el));

    common.updateHeader();

    $('#loading-container').addClass('show');
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
            $('#loading-container').removeClass('show');
        },
    });

    // Add keyboard navigation
    document.addEventListener('keydown', (e) => {
        // Don't handle keyboard events if user is typing in an input
        if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') {
            return;
        }

        switch (e.key) {
            case 'Escape':
                // Clear filters and selections
                $('#filters-clear').click();
                break;

            case 'ArrowLeft':
            case 'ArrowRight':
                // Navigate between client boxes
                const boxes = $('.client-box');
                const selected = $('.client-box.selected');
                if (!selected.length) {
                    // If no box is selected, select the first one on right arrow
                    if (e.key === 'ArrowRight') {
                        boxes.first().click();
                    }
                    return;
                }

                const currentIndex = boxes.index(selected);
                const nextIndex = currentIndex + (e.key === 'ArrowLeft' ? -1 : 1);

                if (nextIndex >= 0 && nextIndex < boxes.length) {
                    const nextBox = boxes.eq(nextIndex);
                    nextBox.click();
                    nextBox[0].scrollIntoView({ behavior: 'smooth', block: 'nearest' });
                }
                break;

            case 'ArrowUp':
            case 'ArrowDown':
                // Navigate between suite boxes
                const suites = $('.suite-box');
                const selectedSuite = $('.suite-box.selected');
                if (!selectedSuite.length) {
                    // If no suite is selected, select the first one on down arrow
                    if (e.key === 'ArrowDown') {
                        suites.first().find('.title').click();
                    }
                    return;
                }

                const currentSuiteIndex = suites.index(selectedSuite);
                const nextSuiteIndex = currentSuiteIndex + (e.key === 'ArrowUp' ? -1 : 1);

                if (nextSuiteIndex >= 0 && nextSuiteIndex < suites.length) {
                    const nextSuite = suites.eq(nextSuiteIndex);
                    nextSuite.find('.title').click();
                    nextSuite[0].scrollIntoView({ behavior: 'smooth', block: 'nearest' });
                }
                break;

            case 'Enter':
                // If a client box is selected, load its results
                const selectedBox = $('.client-box.selected');
                if (selectedBox.length) {
                    selectedBox.find('.btn-load-results').click();
                }
                break;

            case 'g':
                // Group by toggle
                if (e.ctrlKey || e.metaKey) {
                    e.preventDefault();
                    const currentGrouping = $('.summary-controls button[data-group-by].active').data('group-by');
                    const newGrouping = currentGrouping === 'suite' ? 'client' : 'suite';
                    window.toggleGrouping(newGrouping);
                }
                break;

            case '/':
                // Focus search
                e.preventDefault();
                $('.dataTables_filter input').focus();
                break;
        }
    });

    // Add keyboard shortcut hints to UI elements
    $('.dataTables_filter input').attr('placeholder', 'Search... (Press /)');
    $('.summary-controls button[data-group-by]').attr('title', 'Toggle grouping (Ctrl/⌘ + G)');
});

function showFileListing(data) {
    console.log('Got file list');

    // Process summary data
    const lines = data.trim().split('\n');
    const suiteGroups = new Map();

    // First, collect all runs for each suite+client combination
    lines.forEach(line => {
        const entry = JSON.parse(line);
        if (!suiteGroups.has(entry.name)) {
            suiteGroups.set(entry.name, new Map());
        }

        const clientGroup = suiteGroups.get(entry.name);
        const clientKey = entry.clients.join(',');

        if (!clientGroup.has(clientKey)) {
            clientGroup.set(clientKey, []);
        }
        clientGroup.get(clientKey).push(entry);
    });

    // Sort runs and keep only last 5 for each client
    for (const clientGroup of suiteGroups.values()) {
        for (const [clientKey, runs] of clientGroup.entries()) {
            runs.sort((a, b) => new Date(b.start) - new Date(a.start));
            clientGroup.set(clientKey, runs.slice(0, 5));
        }
    }

    // Display summary boxes
    const summaryDiv = $('#suite-summary');

    // Add global sort controls
    const sortControls = $(`
        <div class="summary-controls d-flex gap-2">
            <div class="btn-group">
                <button class="btn btn-sm btn-secondary active" data-group-by="suite" onclick="window.toggleGrouping('suite')">Group by Suite</button>
                <button class="btn btn-sm btn-secondary" data-group-by="client" onclick="window.toggleGrouping('client')">Group by Client</button>
            </div>
            <div class="dropdown">
                <button class="btn btn-sm btn-secondary dropdown-toggle" type="button" data-bs-toggle="dropdown">
                    Sort runs by: <span class="current-sort">Name</span>
                </button>
                <ul class="dropdown-menu dropdown-menu-end">
                    <li><a class="dropdown-item" href="#" onclick="event.preventDefault(); window.sortAllClients('name')">Name</a></li>
                    <li><a class="dropdown-item" href="#" onclick="event.preventDefault(); window.sortAllClients('coverage')">Coverage</a></li>
                    <li><a class="dropdown-item" href="#" onclick="event.preventDefault(); window.sortAllClients('time')">Time</a></li>
                </ul>
            </div>
            <button class="btn btn-sm btn-secondary" data-bs-toggle="modal" data-bs-target="#keyboardShortcutsModal">
                <i class="bi bi-question-circle"></i>
            </button>
        </div>
    `);
    summaryDiv.prepend(sortControls);

    // Add keyboard shortcuts modal
    const shortcutsModal = $(`
        <div class="modal fade" id="keyboardShortcutsModal" tabindex="-1">
            <div class="modal-dialog modal-dialog-centered">
                <div class="modal-content">
                    <div class="modal-header">
                        <h5 class="modal-title">Keyboard Shortcuts</h5>
                        <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                    </div>
                    <div class="modal-body">
                        <table class="table table-sm">
                            <tbody>
                                <tr>
                                    <td><kbd>/</kbd></td>
                                    <td>Focus search input</td>
                                </tr>
                                <tr>
                                    <td><kbd>←</kbd> <kbd>→</kbd></td>
                                    <td>Navigate between client boxes</td>
                                </tr>
                                <tr>
                                    <td><kbd>↑</kbd> <kbd>↓</kbd></td>
                                    <td>Navigate between suite boxes</td>
                                </tr>
                                <tr>
                                    <td><kbd>Enter</kbd></td>
                                    <td>Load results for selected client</td>
                                </tr>
                                <tr>
                                    <td><kbd>Esc</kbd></td>
                                    <td>Clear filters and selections</td>
                                </tr>
                                <tr>
                                    <td><kbd>Ctrl</kbd>/<kbd>⌘</kbd> + <kbd>G</kbd></td>
                                    <td>Toggle between suite/client grouping</td>
                                </tr>
                            </tbody>
                        </table>
                    </div>
                </div>
            </div>
        </div>
    `);
    $('body').append(shortcutsModal);

    // Add keyboard shortcut to open modal
    document.addEventListener('keydown', (e) => {
        if (e.key === '?' && !e.target.closest('input, textarea')) {
            e.preventDefault();
            new bootstrap.Modal('#keyboardShortcutsModal').show();
        }
    });

    // Add floating filters notice
    const filtersNotice = $(`
        <div id="filters-notice" class="btn-group" role="group" style="display: none">
            <button type="button" class="btn btn-light" disabled>Filters active</button>
            <button type="button" class="btn btn-primary" id="filters-clear">Clear Filters</button>
        </div>
    `);
    $('main').prepend(filtersNotice);

    let suites = [];
    data.split('\n').forEach(function(elem) {
        if (!elem) {
            return;
        }
        let suite = JSON.parse(elem);
        suite.start = new Date(suite.start);
        suites.push(suite);
    });

    // Store the processed data globally for reuse
    window.processedData = {
        suiteGroups,
        clientGroups: processClientGroups(suites)
    };

    // Initial display based on URL hash
    const urlParams = new URLSearchParams(window.location.hash.substring(1));
    const groupBy = urlParams.get('group-by') || 'suite';

    // Set initial button state
    $('.summary-controls button[data-group-by]').removeClass('active');
    $(`.summary-controls button[data-group-by="${groupBy}"]`).addClass('active');

    displayGroups(groupBy);

    // Apply initial sort from URL hash if present
    const initialSort = urlParams.get('summary-sort') || 'name';
    window.sortAllClients(initialSort);

    // Handle suite and client selection from URL hash
    const hashSuite = urlParams.get('suite');
    const hashClient = urlParams.get('client');
    if (hashSuite) {
        // Find and highlight the clicked suite box
        $(`.suite-box:has(.title:contains('${hashSuite}'))`).filter(function() {
            return $(this).find('.title').text() === hashSuite;
        }).addClass('selected');

        if (hashClient) {
            // Find and highlight the clicked client box
            $(`.client-box[data-suite="${hashSuite}"][data-client="${hashClient}"]`).addClass('selected');
        }
    }

    let theTable = $('#filetable').DataTable({
        data: suites,
        pageLength: 50,
        autoWidth: false,
        dom: '<"row"<"col-sm-6"l><"col-sm-6"f>>' +
             '<"row"<"col-sm-12"tr>>' +
             '<"row"<"col-sm-5"i><"col-sm-7"p>>',
        language: {
            lengthMenu: "Show _MENU_",
            search: "",
            searchPlaceholder: "Search...",
            info: "Showing _START_ to _END_ of _TOTAL_ results",
            infoEmpty: "No results found",
            paginate: {
                first: "«",
                last: "»",
                next: "›",
                previous: "‹"
            }
        },
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
                render: function(data, type, row) {
                    // For searching and ordering, return raw data
                    if (type === 'filter' || type === 'sort') {
                        return data.join(',');  // Return comma-separated list for searching
                    }
                    // For display, return the HTML
                    const clients = data.map(client => {
                        const version = row.versions ? row.versions[client] || '' : '';
                        return `<div class="client-entry">
                            <span class="client-name">${client}</span>
                            ${version ? `<span class="client-version"><code>${version}</code></span>` : ''}
                        </div>`;
                    }).join('');
                    return `<div class="client-list" data-suite-id="${row.fileName}" data-suite-name="${row.name}">${clients}</div>`;
                },
            },
            {
                title: 'Status',
                data: null,
                width: '5.5em',
                className: 'suite-status-column',
                render: function(data, type, row) {
                    if (data.fails > 0) {
                        let prefix = data.timeout ? 'Timeout' : 'Fail';
                        return `<span><span class="pass-count">✓ ${data.passes}</span> <span class="fail-count">✗ ${data.fails}</span> <span class="badge bg-danger ms-1">${prefix}</span></span>`;
                    }
                    return `<span class="pass-count">✓ ${data.passes} <span class="badge bg-success ms-1">Pass</span></span>`;
                },
            },
            {
                title: 'Diff',
                data: null,
                width: '2em',
                className: 'suite-diff-column',
                orderable: false,
                render: function(data, type, row) {
                    // Find previous run with same suite and clients
                    const prevRun = suites.find(s =>
                        s.name === data.name &&
                        s.clients.join(',') === data.clients.join(',') &&
                        new Date(s.start) < new Date(data.start)
                    );

                    if (!prevRun || prevRun.passes === data.passes) {
                        return '';
                    }

                    const passDiff = prevRun.passes - data.passes;
                    const sign = passDiff > 0 ? '-' : '+';
                    const absValue = Math.abs(passDiff);
                    return `<span class="${passDiff > 0 ? 'fail-diff' : 'pass-diff'}" title="Change in passing tests compared to previous run">${sign}${absValue}</span>`;
                },
            },
            {
                title: '',
                data: null,
                width: '11em',
                className: 'action-buttons-column',
                orderable: false,
                render: function(data) {
                    let url = routes.suite(data.fileName, data.name);
                    let loadText = 'Load (' + formatBytes(data.size) + ')';
                    const clientKey = data.clients.join(',');

                    return `<div class="btn-group w-100">
                        ${makeButton('#', '<i class="bi bi-funnel"></i>', `btn-outline-secondary btn-sm`, `onclick="filterSuiteAndClient('${data.name}', '${clientKey}'); return false;" title="Filter by this suite and client"`).outerHTML}
                        ${makeButton(url, loadText, "btn-secondary btn-sm flex-grow-1").outerHTML}
                    </div>`;
                },
            },
        ],
    });

    const filters = new ColumnFilterSet(theTable);
    filters.build();
    $('#filters-clear').click(function () {
        filters.clear();
        $('.suite-box').removeClass('selected');
        $('.client-box').removeClass('selected');
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
        const api = this._controller.table;
        const select = this.buildSelect();
        let options = new Set();

        // Get unique client names and combinations from all rows
        api.column(this._columnIndex)
           .data()
           .each(function(clients) {
               if (clients) {
                   // Add individual clients
                   clients.forEach(client => options.add(client));
                   // Add exact combination if multiple clients
                   if (clients.length > 1) {
                       options.add(clients.join(','));
                   }
               }
           });

        // Add options sorted alphabetically
        Array.from(options.values())
            .sort()
            .forEach(function(d) {
                select.append($('<option value="'+d+'">'+d+'</option>'));
            });

        return select;
    }

    apply(value) {
        const api = this._controller.table;
        if (value !== '') {
            // Custom search function
            $.fn.dataTable.ext.search.push(
                function(settings, searchData, index, rowData, counter) {
                    // For single client, match if it exists in the row
                    if (!value.includes(',')) {
                        return searchData[2].split(',').includes(value);
                    }
                    // For client combinations, match exact string
                    return searchData[2] === value;
                }
            );

            // Trigger search
            api.draw();

            // Remove the custom search function
            $.fn.dataTable.ext.search.pop();
        } else {
            api.draw();
        }

        // Update URL hash segment
        this.storeToURL(value);

        // Notify controller
        const changed = value !== this.value;
        this._value = value;
        if (changed) {
            this._controller.filterChanged(this);
        }
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
        select.append($('<option value="PASS">PASS</option>'));
        select.append($('<option value="FAIL">FAIL</option>'));
        select.append($('<option value="TIMEOUT">TIMEOUT</option>'));
        return select;
    }

    valueToRegExp(value) {
        return escapeRegExp(value);
    }
}

// Add this function at the global scope
window.filterSuite = function(suiteName) {
    // Remove all selections
    $('.suite-box').removeClass('selected');
    $('.client-box').removeClass('selected');

    // Find and highlight the clicked suite - match exact title
    $(`.suite-box:has(.title:contains('${suiteName}'))`).filter(function() {
        return $(this).find('.title').text() === suiteName;
    }).addClass('selected');

    const filters = new ColumnFilterSet($('#filetable').DataTable());

    // Apply suite filter
    const suiteFilter = filters.byKey('suite');
    if (suiteFilter) {
        suiteFilter.apply(suiteName);
        $('select', $('.filters th').eq(1)).val(suiteName);
    }

    // Clear client filter
    const clientFilter = filters.byKey('client');
    if (clientFilter) {
        clientFilter.apply('');
        $('select', $('.filters th').eq(2)).val('');
    }

    // Scroll to the table
    $('#filetable').get(0).scrollIntoView({ behavior: 'smooth', block: 'start' });
};

// Update the existing filterSuiteAndClient function to also handle suite box selection
window.filterSuiteAndClient = function(suiteName, clientKey) {
    // Remove all selections
    $('.suite-box').removeClass('selected');
    $('.client-box').removeClass('selected');

    // Find and highlight the clicked box and its suite - match exact title
    $(`.suite-box:has(.title:contains('${suiteName}'))`).filter(function() {
        return $(this).find('.title').text() === suiteName;
    }).addClass('selected');
    $(`.client-box[data-suite="${suiteName}"][data-client="${clientKey}"]`).addClass('selected');

    const filters = new ColumnFilterSet($('#filetable').DataTable());

    // Apply suite filter
    const suiteFilter = filters.byKey('suite');
    if (suiteFilter) {
        suiteFilter.apply(suiteName);
        $('select', $('.filters th').eq(1)).val(suiteName);
    }

    // Apply client filter
    const clientFilter = filters.byKey('client');
    if (clientFilter) {
        clientFilter.apply(clientKey);
        $('select', $('.filters th').eq(2)).val(clientKey);
    }

    // Scroll to the table
    $('#filetable').get(0).scrollIntoView({ behavior: 'smooth', block: 'start' });
};

function processClientGroups(suites) {
    const clientGroups = new Map();

    suites.forEach(entry => {
        entry.clients.forEach(client => {
            if (!clientGroups.has(client)) {
                clientGroups.set(client, new Map());
            }

            const suiteGroup = clientGroups.get(client);
            const suiteKey = entry.name;

            if (!suiteGroup.has(suiteKey)) {
                suiteGroup.set(suiteKey, []);
            }
            suiteGroup.get(suiteKey).push(entry);
        });
    });

    // Sort and limit runs
    for (const suiteGroup of clientGroups.values()) {
        for (const [suiteKey, runs] of suiteGroup.entries()) {
            runs.sort((a, b) => new Date(b.start) - new Date(a.start));
            suiteGroup.set(suiteKey, runs.slice(0, 5));
        }
    }

    return clientGroups;
}

window.toggleGrouping = function(groupBy) {
    // Update button states
    $('.summary-controls button[data-group-by]').removeClass('active');
    $(`.summary-controls button[data-group-by="${groupBy}"]`).addClass('active');

    // Update URL hash
    const urlParams = new URLSearchParams(window.location.hash.substring(1));
    urlParams.set('group-by', groupBy);
    window.history.replaceState(null, '', '#' + urlParams.toString());

    // Display the groups
    displayGroups(groupBy);
};

function displayGroups(groupBy) {
    const summaryDiv = $('#suite-summary');
    summaryDiv.find('.suite-box, .client-box-container').remove();

    if (groupBy === 'suite') {
        displaySuiteGroups(window.processedData.suiteGroups);
    } else {
        displayClientGroups(window.processedData.clientGroups);
    }

    // Initialize popovers after adding new content
    initPopovers();
}

function initPopovers() {
    // Destroy existing popovers
    document.querySelectorAll('[data-bs-toggle="popover"]').forEach(el => {
        const popover = bootstrap.Popover.getInstance(el);
        if (popover) {
            popover.dispose();
        }
    });
    // Initialize new popovers
    const popoverTriggerList = document.querySelectorAll('[data-bs-toggle="popover"]');
    [...popoverTriggerList].map(el => new bootstrap.Popover(el, {
        placement: 'top',
        html: true
    }));
}

function displaySuiteGroups(suiteGroups) {
    Array.from(suiteGroups.entries())
        .sort((a, b) => a[0].localeCompare(b[0]))
        .forEach(([suiteName, clientResults]) => {
            const clientBoxes = Array.from(clientResults.entries())
                .sort((a, b) => a[0].localeCompare(b[0]))
                .map(([clientKey, runs]) => {
                    const latest = runs[0];
                    const timeAgo = timeSince(new Date(latest.start));

                    // Generate history dots
                    const historyDots = runs.map((run, idx) => {
                        const prevRun = runs[idx + 1];
                        let trendClass = '';
                        if (prevRun) {
                            const prevRatio = prevRun.passes / (prevRun.passes + prevRun.fails);
                            const currRatio = run.passes / (run.passes + run.fails);
                            trendClass = currRatio > prevRatio ? 'trend-up' :
                                       currRatio < prevRatio ? 'trend-down' : 'trend-same';
                        }
                        const passRatio = (run.passes / (run.passes + run.fails)) * 100;
                        return `
                            <div class="history-dot ${trendClass}"
                                 data-bs-toggle="popover"
                                 data-bs-trigger="hover"
                                 data-bs-content="<div><span class='text-success'>✓ ${run.passes}</span>${run.fails > 0 ? `<span class='text-danger'> ✗ ${run.fails}</span>` : ''} passed (<span class='text-primary'>${passRatio.toFixed(2)}%</span>)</div><div class='text-secondary mt-1'>${timeSince(new Date(run.start))} ago</div>">
                                <div class="dot-fill" style="height: ${passRatio}%; --pass-percent: ${passRatio/100}"></div>
                            </div>
                        `;
                    }).reverse().join('');

                    return `
                        <div class="client-box ${latest.passes === 0 ? 'all-failed' : latest.fails === 0 ? 'all-passed' : 'has-failures'}"
                             data-suite="${suiteName}" data-client="${clientKey}" data-time="${latest.start}"
                             onclick="window.filterSuiteAndClient('${suiteName}', '${clientKey}')">
                            <div class="client-name">${clientKey}</div>
                            <div class="stats">
                                <span class="pass-count">✓ ${latest.passes}</span>
                                ${latest.fails > 0 ? `<span class="fail-count">✗ ${latest.fails}</span>` : ''}
                                <div class="history-dots">${historyDots}</div>
                            </div>
                            <div class="time">
                                <span>${timeAgo} ago</span>
                                <span class="coverage-percent">
                                    ${((latest.passes / (latest.passes + latest.fails)) * 100).toFixed(2)}%
                                </span>
                            </div>
                        </div>
                    `;
                }).join('');

            const box = $(`
                <div class="suite-box">
                    <div class="title" onclick="window.filterSuite('${suiteName}')">${suiteName}</div>
                    <div class="client-results">
                        ${clientBoxes}
                    </div>
                </div>
            `);
            $('#suite-summary').append(box);
        });
}

function displayClientGroups(clientGroups) {
    Array.from(clientGroups.entries())
        .sort((a, b) => a[0].localeCompare(b[0]))
        .forEach(([clientName, suiteResults]) => {
            const suiteBoxes = Array.from(suiteResults.entries())
                .sort((a, b) => a[0].localeCompare(b[0]))
                .map(([suiteName, runs]) => {
                    const latest = runs[0];
                    const timeAgo = timeSince(new Date(latest.start));

                    // Generate history dots
                    const historyDots = runs.map((run, idx) => {
                        const prevRun = runs[idx + 1];
                        let trendClass = '';
                        if (prevRun) {
                            const prevRatio = prevRun.passes / (prevRun.passes + prevRun.fails);
                            const currRatio = run.passes / (run.passes + run.fails);
                            trendClass = currRatio > prevRatio ? 'trend-up' :
                                       currRatio < prevRatio ? 'trend-down' : 'trend-same';
                        }
                        const passRatio = (run.passes / (run.passes + run.fails)) * 100;
                        return `
                            <div class="history-dot ${trendClass}"
                                 data-bs-toggle="popover"
                                 data-bs-trigger="hover"
                                 data-bs-content="<div><span class='text-success'>✓ ${run.passes}</span>${run.fails > 0 ? `/<span class='text-danger'>✗ ${run.fails}</span>` : ''} passed (<span class='text-primary'>${passRatio.toFixed(2)}%</span>)</div><div class='text-secondary mt-1'>${timeSince(new Date(run.start))} ago</div>">
                                <div class="dot-fill" style="height: ${passRatio}%; --pass-percent: ${passRatio/100}"></div>
                            </div>
                        `;
                    }).reverse().join('');

                    return `
                        <div class="client-box ${latest.passes === 0 ? 'all-failed' : latest.fails === 0 ? 'all-passed' : 'has-failures'}"
                             data-suite="${suiteName}" data-client="${clientName}" data-time="${latest.start}"
                             onclick="window.filterSuiteAndClient('${suiteName}', '${clientName}')">
                            <div class="client-name">${suiteName}</div>
                            <div class="stats">
                                <span class="pass-count">✓ ${latest.passes}</span>
                                ${latest.fails > 0 ? `<span class="fail-count">✗ ${latest.fails}</span>` : ''}
                                <div class="history-dots">${historyDots}</div>
                            </div>
                            <div class="time">
                                <span>${timeAgo} ago</span>
                                <span class="coverage-percent">
                                    ${((latest.passes / (latest.passes + latest.fails)) * 100).toFixed(2)}%
                                </span>
                            </div>
                        </div>
                    `;
                }).join('');

            const box = $(`
                <div class="suite-box">
                    <div class="title" onclick="window.filterClient('${clientName}')">${clientName}</div>
                    <div class="client-results">
                        ${suiteBoxes}
                    </div>
                </div>
            `);
            $('#suite-summary').append(box);
        });
}

window.filterClient = function(clientName) {
    // Remove all selections
    $('.suite-box').removeClass('selected');
    $('.client-box').removeClass('selected');

    // Find and highlight the clicked client box
    $(`.suite-box:has(.title:contains('${clientName}'))`).filter(function() {
        return $(this).find('.title').text() === clientName;
    }).addClass('selected');

    const filters = new ColumnFilterSet($('#filetable').DataTable());

    // Apply client filter
    const clientFilter = filters.byKey('client');
    if (clientFilter) {
        clientFilter.apply(clientName);
        $('select', $('.filters th').eq(2)).val(clientName);
    }

    // Clear suite filter
    const suiteFilter = filters.byKey('suite');
    if (suiteFilter) {
        suiteFilter.apply('');
        $('select', $('.filters th').eq(1)).val('');
    }

    // Scroll to the table
    $('#filetable').get(0).scrollIntoView({ behavior: 'smooth', block: 'start' });
};
