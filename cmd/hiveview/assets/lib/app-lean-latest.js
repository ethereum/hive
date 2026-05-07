import $ from 'jquery';

import * as routes from './routes.js';
import { formatDuration } from './utils.js';

const listingURL = 'listing.jsonl?limit=1000';
const preferredSuiteOrder = ['client-interop', 'rpc-compat', 'sync', 'validation', 'gossip', 'reqresp'];
const leanLatestState = {
    entries: [],
    devnets: [],
    selectedDevnet: '',
};

export async function loadLeanLatest(options = {}) {
    const manageLoading = options.manageLoading !== false;
    if (manageLoading) {
        $('#loading-container').addClass('show');
    }
    $('#lean-latest-error').hide();

    try {
        const listingText = await loadText(listingURL);
        const entries = prepareEntries(parseListing(listingText));
        if (entries.length === 0) {
            renderEmptyState('No lean suite runs found.');
            return;
        }

        leanLatestState.entries = entries;
        leanLatestState.devnets = collectDevnets(entries);
        const selectedDevnet = selectedDevnetFromURL(leanLatestState.devnets) || defaultDevnet(leanLatestState.devnets);
        await renderSelectedDevnet(selectedDevnet, false);
    } catch (err) {
        showError(`Unable to load lean latest results: ${err.message || err}`);
    } finally {
        if (manageLoading) {
            $('#loading-container').removeClass('show');
        }
    }
}

async function renderSelectedDevnet(devnet, updateURL) {
    leanLatestState.selectedDevnet = devnet || '';
    if (updateURL) {
        updateDevnetURL(leanLatestState.selectedDevnet);
    }
    renderDevnetControls(leanLatestState.devnets, leanLatestState.selectedDevnet);

    const entries = entriesForDevnet(leanLatestState.entries, leanLatestState.selectedDevnet);
    const latestEntries = latestSuiteEntries(entries);
    if (latestEntries.length === 0) {
        const suffix = leanLatestState.selectedDevnet ? ` for ${leanLatestState.selectedDevnet}` : '';
        renderEmptyState(`No lean suite runs found${suffix}.`);
        return;
    }

    const matrices = await Promise.all(latestEntries.map(loadSuiteMatrices));
    renderLeanLatest(matrices.flat().filter(Boolean), leanLatestState.selectedDevnet);
}

async function loadSuiteMatrices(entry) {
    const suiteData = await loadJSON(routes.resultsRoot + entry.fileName);
    return buildSuiteMatrices(entry, suiteData);
}

async function loadText(url) {
    const response = await fetch(url, { cache: 'no-store' });
    if (!response.ok) {
        throw new Error(`${url} returned ${response.status}`);
    }
    return response.text();
}

async function loadJSON(url) {
    const response = await fetch(url, { cache: 'no-store' });
    if (!response.ok) {
        throw new Error(`${url} returned ${response.status}`);
    }
    return response.json();
}

function parseListing(data) {
    return data.split('\n').reduce((entries, line) => {
        line = line.trim();
        if (!line) {
            return entries;
        }

        try {
            const entry = JSON.parse(line);
            entry.startDate = parseDate(entry.start);
            entries.push(entry);
        } catch (err) {
            console.warn('Skipping invalid listing entry:', err);
        }
        return entries;
    }, []);
}

function prepareEntries(entries) {
    return entries.map(entry => ({
        ...entry,
        devnet: normalizeDevnet(entry.devnet) || devnetFromEntry(entry),
    }));
}

function collectDevnets(entries) {
    const devnets = new Set();
    entries.forEach(entry => {
        if (entry.devnet) {
            devnets.add(entry.devnet);
        }
    });
    return Array.from(devnets).sort(compareDevnets);
}

function entriesForDevnet(entries, devnet) {
    if (!devnet) {
        return entries;
    }
    return entries.filter(entry => entry.devnet === devnet);
}

function selectedDevnetFromURL(devnets) {
    const params = new URLSearchParams(window.location.search);
    const devnet = normalizeDevnet(params.get('devnet'));
    return devnets.includes(devnet) ? devnet : '';
}

function defaultDevnet(devnets) {
    return devnets[devnets.length - 1] || '';
}

function updateDevnetURL(devnet) {
    const url = new URL(window.location.href);
    if (devnet) {
        url.searchParams.set('devnet', devnet);
    } else {
        url.searchParams.delete('devnet');
    }
    window.history.replaceState(null, '', url);
}

function devnetFromEntry(entry) {
    for (const client of entry.clients || []) {
        const devnet = normalizeDevnet(client);
        if (devnet) {
            return devnet;
        }
    }
    for (const client of Object.keys(entry.versions || {})) {
        const devnet = normalizeDevnet(client);
        if (devnet) {
            return devnet;
        }
    }
    return '';
}

function normalizeDevnet(value) {
    if (!value || typeof value !== 'string') {
        return '';
    }
    const match = value.match(/(?:^|[^a-z0-9])(devnet[0-9][a-z0-9_-]*)/i);
    return match ? match[1].toLowerCase() : '';
}

function compareDevnets(a, b) {
    const left = devnetSortParts(a);
    const right = devnetSortParts(b);
    if (left.number !== right.number) {
        return left.number - right.number;
    }
    return left.suffix.localeCompare(right.suffix) || a.localeCompare(b);
}

function devnetSortParts(devnet) {
    const match = devnet.match(/^devnet([0-9]+)(.*)$/i);
    if (!match) {
        return { number: Number.MAX_SAFE_INTEGER, suffix: devnet };
    }
    return {
        number: Number.parseInt(match[1], 10),
        suffix: match[2] || '',
    };
}

function latestSuiteEntries(entries) {
    const latest = new Map();
    entries.forEach(entry => {
        if (!entry.name) {
            return;
        }

        const previous = latest.get(entry.name);
        if (!previous || compareDates(entry.startDate, previous.startDate) > 0) {
            latest.set(entry.name, entry);
        }
    });
    return Array.from(latest.values()).sort(compareSuites);
}

function compareSuites(a, b) {
    const ar = suiteRank(a.name);
    const br = suiteRank(b.name);
    if (ar !== br) {
        return ar - br;
    }
    return a.name.localeCompare(b.name);
}

function suiteRank(name) {
    const index = preferredSuiteOrder.indexOf(name);
    return index === -1 ? preferredSuiteOrder.length : index;
}

function buildSuiteMatrices(entry, suiteData) {
    if ((suiteData.name || entry.name) === 'client-interop') {
        return buildClientInteropMatrices(entry, suiteData);
    }
    return [buildSuiteMatrix(entry, suiteData)];
}

function buildSuiteMatrix(entry, suiteData) {
    const suiteName = suiteData.name || entry.name;
    const cases = Object.entries(suiteData.testCases || {})
        .map(([testIndex, test]) => ({
            ...test,
            testIndex,
            numericIndex: numericTestIndex(testIndex),
        }))
        .sort((a, b) => a.numericIndex - b.numericIndex || testName(a).localeCompare(testName(b)));

    const rowMap = new Map();

    cases.forEach(test => {
        const clients = clientNamesForTest(test);
        if (clients.length === 0) {
            return;
        }

        const name = testName(test);
        if (!rowMap.has(name)) {
            rowMap.set(name, {
                name,
                order: test.numericIndex,
                cells: new Map(),
            });
        }

        const row = rowMap.get(name);
        row.order = Math.min(row.order, test.numericIndex);

        clients.forEach(clientName => {
            if (!row.cells.has(clientName)) {
                row.cells.set(clientName, []);
            }
            row.cells.get(clientName).push(test);
        });
    });

    const clients = collectClients(entry, suiteData, cases);

    return {
        entry,
        suiteData,
        suiteName,
        clients,
        cases,
        rowHeaderLabel: 'Test',
        rows: Array.from(rowMap.values()).sort((a, b) => a.order - b.order || a.name.localeCompare(b.name)),
        stats: suiteStats(cases, suiteName),
    };
}

function buildClientInteropMatrices(entry, suiteData) {
    const suiteName = suiteData.name || entry.name;
    const cases = Object.entries(suiteData.testCases || {})
        .map(([testIndex, test]) => ({
            ...test,
            testIndex,
            numericIndex: numericTestIndex(testIndex),
            topology: clientInteropTopology(test),
        }))
        .sort((a, b) => a.numericIndex - b.numericIndex || testName(a).localeCompare(testName(b)));

    const topologyCases = cases
        .map(test => ({
            ...test,
            roles: clientInteropRoles(test.topology),
        }))
        .filter(test => test.roles);

    const clients = collectClients(entry, suiteData, topologyCases);
    const clientOrder = clientOrderMap(clients);
    const rows = buildClientInteropRows(topologyCases, 'majority', 'minority', clientOrder, '2 nodes');

    return [{
        entry,
        suiteData,
        suiteName,
        linkSuiteName: suiteName,
        clients: roleLabelClients(clients, '1 node'),
        cases,
        rowHeaderLabel: '',
        rowRoleLabel: 'majority',
        columnRoleLabel: 'minority',
        rows,
        stats: suiteStats(cases, suiteName),
        emptyMessage: 'No client-interop topology tests with client results.',
    }];
}

function buildClientInteropRows(cases, rowRole, columnRole, clientOrder, rowRoleLabel) {
    const rowMap = new Map();

    cases.forEach(test => {
        const rowName = test.roles[rowRole];
        const columnName = test.roles[columnRole];
        if (!rowMap.has(rowName)) {
            rowMap.set(rowName, {
                name: rowName,
                label: roleLabel(rowName, rowRoleLabel),
                order: test.numericIndex,
                cells: new Map(),
            });
        }

        const row = rowMap.get(rowName);
        row.order = Math.min(row.order, test.numericIndex);
        if (!row.cells.has(columnName)) {
            row.cells.set(columnName, []);
        }
        row.cells.get(columnName).push(test);
    });

    return Array.from(rowMap.values()).sort((a, b) => {
        const orderDiff = clientOrderRank(clientOrder, a.name) - clientOrderRank(clientOrder, b.name);
        if (orderDiff !== 0) {
            return orderDiff;
        }
        return a.order - b.order || a.name.localeCompare(b.name);
    });
}

function roleLabelClients(clients, role) {
    return clients.map(client => ({
        ...client,
        label: roleLabel(client.label, role),
    }));
}

function roleLabel(label, role) {
    return `${label} (${role})`;
}

function clientOrderMap(clients) {
    const order = new Map();
    clients.forEach((client, index) => {
        order.set(client.name, index);
    });
    return order;
}

function clientOrderRank(order, name) {
    return order.has(name) ? order.get(name) : Number.MAX_SAFE_INTEGER;
}

function clientInteropTopology(test) {
    const marker = ' / ';
    const name = testName(test);
    const index = name.lastIndexOf(marker);
    if (index !== -1) {
        const topology = name.slice(index + marker.length).split(',').map(client => client.trim()).filter(Boolean);
        if (topology.length > 0) {
            return topology;
        }
    }

    const match = (test.description || '').match(/^Starts\s+(.+?)\s+with a shared genesis/);
    if (!match) {
        return [];
    }
    return match[1].split(',').map(client => client.trim()).filter(Boolean);
}

function clientInteropRoles(topology) {
    if (topology.length === 0) {
        return null;
    }

    const counts = new Map();
    topology.forEach(client => counts.set(client, (counts.get(client) || 0) + 1));
    if (counts.size < 2) {
        return null;
    }

    const entries = Array.from(counts.entries()).sort((a, b) => b[1] - a[1]);
    const [majority, majorityCount] = entries[0];
    const [minority, minorityCount] = entries[entries.length - 1];
    if (majorityCount === minorityCount) {
        return null;
    }
    return { majority, minority };
}

function collectClients(entry, suiteData, cases) {
    const versions = suiteData.clientVersions || entry.versions || {};
    const names = [];
    const seen = new Set();

    const addClient = name => {
        if (!name || seen.has(name)) {
            return;
        }
        seen.add(name);
        names.push(name);
    };

    cases.forEach(test => {
        if (test.topology && test.topology.length > 0) {
            test.topology.forEach(addClient);
            return;
        }
        clientNamesForTest(test).forEach(addClient);
    });

    Object.keys(versions).sort().forEach(addClient);
    if (names.length === 0) {
        (entry.clients || []).slice().sort().forEach(addClient);
    }

    return names.map(name => ({
        name,
        label: name,
        version: versions[name] || '',
    }));
}

function clientNamesForTest(test) {
    const names = [];
    const seen = new Set();
    clientInfoEntries(test).forEach(info => {
        if (info && info.name && !seen.has(info.name)) {
            seen.add(info.name);
            names.push(info.name);
        }
    });
    return names;
}

function clientInfoEntries(test) {
    return Object.values(test.clientInfo || {}).sort((a, b) => {
        const aTime = parseDate(a.instantiatedAt);
        const bTime = parseDate(b.instantiatedAt);
        const timeDiff = compareDates(aTime, bTime);
        if (timeDiff !== 0) {
            return timeDiff;
        }

        const nameDiff = (a.name || '').localeCompare(b.name || '');
        if (nameDiff !== 0) {
            return nameDiff;
        }
        return (a.id || '').localeCompare(b.id || '');
    });
}

function suiteStats(cases, suiteName) {
    let passed = 0;
    let failed = 0;
    let timeouts = 0;
    let total = 0;
    let start = null;
    let end = null;

    cases.forEach(test => {
        if (!isSuiteSetupTest(test, suiteName)) {
            total++;
            if (test.summaryResult && test.summaryResult.pass) {
                passed++;
            } else {
                failed++;
                if (test.summaryResult && test.summaryResult.timeout) {
                    timeouts++;
                }
            }
        }

        const testStart = parseDate(test.start);
        const testEnd = parseDate(test.end);
        if (testStart && (!start || testStart < start)) {
            start = testStart;
        }
        if (testEnd && (!end || testEnd > end)) {
            end = testEnd;
        }
    });

    return {
        total,
        passed,
        failed,
        timeouts,
        start,
        end,
        duration: start && end ? end - start : null,
    };
}

function isSuiteSetupTest(test, suiteName) {
    const name = testName(test).trim().toLowerCase();
    const normalizedSuite = (suiteName || '').trim().toLowerCase();
    if (normalizedSuite && (
        name === `${normalizedSuite}: client launch` ||
        name === `${normalizedSuite}: matrix`
    )) {
        return true;
    }
    return /:\s*client launch$/i.test(name);
}

function renderLeanLatest(matrices, devnet) {
    if (matrices.length === 0) {
        renderEmptyState('No lean suite runs found.');
        return;
    }

    const clientRanking = renderClientScores(matrices);
    applyClientRanking(matrices, clientRanking);

    const suiteNames = new Set(matrices.map(matrix => matrix.linkSuiteName || matrix.suiteName));
    const failingSuites = new Set(
        matrices
            .filter(matrix => matrix.stats.failed > 0)
            .map(matrix => matrix.linkSuiteName || matrix.suiteName)
    ).size;
    const devnetLabel = devnet ? `${devnet} ` : '';
    $('#lean-latest-subtitle').text(`Latest ${devnetLabel}runs for ${suiteNames.size} suites.`);
    $('#lean-latest-summary').empty()
        .append(summaryPill('Suites', suiteNames.size))
        .append(summaryPill('Failing', failingSuites, failingSuites > 0 ? 'danger' : 'success'));

    const content = $('#lean-latest-content').empty();
    matrices.forEach(matrix => {
        content.append(renderSuiteSection(matrix));
    });
}

function renderClientScores(matrices) {
    const section = $('#lean-client-score-section');
    const content = $('#lean-client-score-content').empty();
    if (!section.length) {
        return [];
    }

    const scores = buildClientScores(matrices);
    if (scores.clients.length === 0 || scores.rows.length === 0) {
        section.hide();
        return [];
    }

    section.show();
    const scroll = $('<div />').addClass('lean-grid-scroll');
    const table = $('<table />').addClass('lean-latest-grid lean-score-grid');
    const thead = $('<thead />');
    const headerRow = $('<tr />');

    headerRow.append($('<th />').addClass('lean-test-column').text('Suite'));
    scores.clients.forEach(client => {
        headerRow.append($('<th />')
            .addClass('lean-result-column')
            .attr('title', client)
            .append($('<span />').addClass('lean-client-heading').text(client)));
    });
    thead.append(headerRow);
    table.append(thead);

    const tbody = $('<tbody />');
    scores.rows.forEach(row => {
        const tr = $('<tr />');
        tr.append($('<th />').addClass('lean-test-column').attr('scope', 'row').text(row.suiteName));
        scores.clients.forEach(client => {
            tr.append(renderScoreCell(row.cells.get(client), {
                href: suiteClientURL(row, client),
                client,
                suiteName: row.suiteName,
            }));
        });
        tbody.append(tr);
    });
    table.append(tbody);

    const tfoot = $('<tfoot />');
    const totalRow = $('<tr />').addClass('lean-score-total-row');
    totalRow.append($('<th />').addClass('lean-test-column').attr('scope', 'row').text('Total'));
    scores.clients.forEach(client => {
        totalRow.append(renderScoreCell(scores.totals.get(client), { isTotal: true }));
    });
    tfoot.append(totalRow);
    table.append(tfoot);

    scroll.append(table);
    content.append(scroll);
    return scores.clients.slice();
}

function applyClientRanking(matrices, ranking) {
    if (!ranking || ranking.length === 0) {
        return;
    }
    const rank = new Map();
    ranking.forEach((name, index) => rank.set(name, index));
    const rankOf = name => (rank.has(name) ? rank.get(name) : Number.MAX_SAFE_INTEGER);

    matrices.forEach(matrix => {
        matrix.clients = matrix.clients.slice().sort((a, b) => {
            const diff = rankOf(a.name) - rankOf(b.name);
            return diff !== 0 ? diff : (a.label || a.name).localeCompare(b.label || b.name);
        });

        if (matrix.rowRoleLabel) {
            matrix.rows = matrix.rows.slice().sort((a, b) => {
                const diff = rankOf(a.name) - rankOf(b.name);
                return diff !== 0 ? diff : a.name.localeCompare(b.name);
            });
        }
    });
}

function buildClientScores(matrices) {
    const clients = [];
    const seenClients = new Set();
    const rows = [];
    const rowsBySuite = new Map();
    const totals = new Map();

    const addClient = name => {
        if (!name || seenClients.has(name)) {
            return;
        }
        seenClients.add(name);
        clients.push(name);
        totals.set(name, emptyScore());
    };

    const scoreFor = (cells, client) => {
        if (!cells.has(client)) {
            cells.set(client, emptyScore());
        }
        return cells.get(client);
    };

    matrices.forEach(matrix => {
        const suiteName = matrix.linkSuiteName || matrix.suiteName;
        if (!rowsBySuite.has(suiteName)) {
            const row = {
                suiteName,
                fileName: matrix.entry.fileName,
                linkSuiteName: matrix.linkSuiteName || matrix.suiteName,
                cells: new Map(),
            };
            rowsBySuite.set(suiteName, row);
            rows.push(row);
        }
        const row = rowsBySuite.get(suiteName);

        matrix.clients.forEach(client => addClient(client.name));
        matrix.cases.forEach(test => {
            if (isSuiteSetupTest(test, suiteName)) {
                return;
            }

            scoreClientNamesForTest(test).forEach(client => {
                addClient(client);
                incrementScore(scoreFor(row.cells, client), test);
                incrementScore(totals.get(client), test);
            });
        });
    });

    clients.sort((a, b) => compareClientScores(totals, a, b));
    return { clients, rows, totals };
}

function suiteClientURL(row, client) {
    return `${routes.suite(row.fileName, row.linkSuiteName)}&client=${encodeURIComponent(client)}`;
}

function compareClientScores(totals, a, b) {
    const left = scorePercent(totals.get(a));
    const right = scorePercent(totals.get(b));
    if (left !== right) {
        return right - left;
    }

    const leftTotal = totals.get(a)?.total || 0;
    const rightTotal = totals.get(b)?.total || 0;
    if (leftTotal !== rightTotal) {
        return rightTotal - leftTotal;
    }
    return a.localeCompare(b);
}

function scorePercent(score) {
    return score && score.total > 0 ? score.passed / score.total : -1;
}

function emptyScore() {
    return { passed: 0, failed: 0, total: 0 };
}

function incrementScore(score, test) {
    score.total++;
    if (test.summaryResult && test.summaryResult.pass) {
        score.passed++;
    } else {
        score.failed++;
    }
}

function scoreClientNamesForTest(test) {
    const names = test.topology && test.topology.length > 0 ? test.topology : clientNamesForTest(test);
    const seen = new Set();
    return names.filter(name => {
        if (!name || seen.has(name)) {
            return false;
        }
        seen.add(name);
        return true;
    });
}

function renderScoreCell(score, options = {}) {
    const td = $('<td />').addClass('lean-result-column');
    const shouldLink = score && score.total > 0 && options.href;
    const value = $(shouldLink ? '<a />' : '<span />').addClass('lean-score-value');
    if (options.isTotal) {
        value.addClass('total');
    }

    if (!score || score.total === 0) {
        value.addClass('empty').text('-');
        td.append(value);
        return td;
    }

    const statusClass = score.failed > 0 ? 'has-failures' : 'all-passed';
    value.addClass(statusClass)
        .attr('title', `${score.passed} passed, ${score.failed} failed`)
        .text(`${score.passed}/${score.total}`);
    if (shouldLink) {
        value.addClass('clickable')
            .attr('href', options.href)
            .attr('aria-label', `Open ${options.suiteName} filtered to ${options.client}`);
    }
    td.append(value);
    return td;
}

function renderDevnetControls(devnets, selectedDevnet) {
    const controls = $('#lean-latest-devnets').empty();
    if (!controls.length || devnets.length === 0) {
        return;
    }

    devnets.forEach(devnet => {
        const active = devnet === selectedDevnet;
        const button = $('<button />')
            .attr('type', 'button')
            .addClass('btn btn-sm btn-secondary lean-devnet-button')
            .toggleClass('active', active)
            .attr('aria-pressed', active ? 'true' : 'false')
            .text(devnet)
            .on('click', async function() {
                if (leanLatestState.selectedDevnet === devnet) {
                    return;
                }

                $('#loading-container').addClass('show');
                $('#lean-latest-error').hide();
                try {
                    await renderSelectedDevnet(devnet, true);
                } catch (err) {
                    showError(`Unable to load ${devnet} results: ${err.message || err}`);
                } finally {
                    $('#loading-container').removeClass('show');
                }
            });
        controls.append(button);
    });
}

function renderSuiteSection(matrix) {
    const section = $('<section />').addClass('lean-suite-section');
    const header = $('<div />').addClass('lean-suite-header');
    const title = $('<div />').addClass('lean-suite-title');
    const actions = $('<div />').addClass('lean-suite-actions');

    title.append($('<h2 />').text(matrix.suiteName));
    title.append(renderSuiteMeta(matrix));

    const statusClass = matrix.stats.failed > 0 ? 'bg-danger' : 'bg-success';
    const statusText = matrix.stats.failed > 0 ? 'Fail' : 'Pass';
    actions.append($('<span />').addClass(`badge ${statusClass}`).text(statusText));
    actions.append($('<a />')
        .addClass('btn btn-sm btn-secondary')
        .attr('href', routes.suite(matrix.entry.fileName, matrix.linkSuiteName || matrix.suiteName))
        .text('Open suite'));

    header.append(title, actions);
    section.append(header);

    if (matrix.rows.length === 0 || matrix.clients.length === 0) {
        section.append($('<p />').addClass('text-secondary').text(matrix.emptyMessage || 'No test cases with client results.'));
        return section;
    }

    const scroll = $('<div />').addClass('lean-grid-scroll');
    const table = $('<table />').addClass('lean-latest-grid');
    const thead = $('<thead />');
    const headerRow = $('<tr />');

    headerRow.append($('<th />').addClass('lean-test-column').text(matrix.rowHeaderLabel ?? 'Test'));
    matrix.clients.forEach(client => {
        const heading = $('<span />').addClass('lean-client-heading').text(client.label);
        const th = $('<th />')
            .addClass('lean-result-column')
            .attr('title', client.version || client.label)
            .append(heading);
        headerRow.append(th);
    });

    thead.append(headerRow);
    table.append(thead);

    const tbody = $('<tbody />');
    matrix.rows.forEach(row => {
        const tr = $('<tr />');
        tr.append($('<th />').addClass('lean-test-column').attr('scope', 'row').text(rowLabel(row)));
        matrix.clients.forEach(client => {
            tr.append(renderResultCell(matrix, row, client));
        });
        tbody.append(tr);
    });
    table.append(tbody);
    scroll.append(table);
    section.append(scroll);

    return section;
}

function renderSuiteMeta(matrix) {
    const meta = $('<div />').addClass('lean-suite-meta');
    const start = matrix.entry.startDate || matrix.stats.start;
    if (start) {
        meta.append($('<span />').text(start.toLocaleString()));
    }
    meta.append($('<span />').text(`${matrix.stats.total} tests`));
    meta.append($('<span />').text(`${matrix.clients.length} clients`));
    meta.append($('<span />').addClass('pass-count').text(`${matrix.stats.passed} passed`));
    if (matrix.stats.failed > 0) {
        meta.append($('<span />').addClass('fail-count').text(`${matrix.stats.failed} failed`));
    }
    if (matrix.stats.timeouts > 0) {
        meta.append($('<span />').addClass('text-warning').text(`${matrix.stats.timeouts} timeouts`));
    }
    if (matrix.stats.duration !== null) {
        meta.append($('<span />').text(formatDuration(matrix.stats.duration)));
    }
    return meta;
}

function renderResultCell(matrix, row, client) {
    const tests = row.cells.get(client.name) || [];
    const td = $('<td />').addClass('lean-result-column');

    if (tests.length === 0) {
        td.append($('<span />')
            .addClass('lean-result-box empty')
            .attr('aria-label', `${client.label} did not run ${rowLabel(row)}`));
        return td;
    }

    const status = resultStatus(tests);
    const linkedTest = preferredLinkedTest(tests);
    const url = routes.testInSuite(matrix.entry.fileName, matrix.linkSuiteName || matrix.suiteName, linkedTest.testIndex);
    const label = statusLabel(status);
    const link = $('<a />')
        .addClass(`lean-result-box ${status}`)
        .attr('href', url)
        .attr('title', resultTitle(matrix, row, client, tests, label))
        .attr('aria-label', `${client.label} ${label} ${rowLabel(row)}`)
        .append($('<i />').addClass(status === 'pass' ? 'bi bi-check-lg' : 'bi bi-x-lg'));

    if (tests.length > 1) {
        link.append($('<span />').addClass('lean-result-count').text(tests.length));
    }

    td.append(link);
    return td;
}

function resultStatus(tests) {
    const failed = tests.some(test => !test.summaryResult || !test.summaryResult.pass);
    if (!failed) {
        return 'pass';
    }
    return tests.some(test => test.summaryResult && test.summaryResult.timeout) ? 'timeout' : 'fail';
}

function preferredLinkedTest(tests) {
    return tests.find(test => !test.summaryResult || !test.summaryResult.pass) || tests[0];
}

function rowLabel(row) {
    return row.label || row.name;
}

function resultTitle(matrix, row, client, tests, label) {
    if (matrix.columnRoleLabel) {
        const rowRole = matrix.rowRoleLabel || matrix.rowHeaderLabel;
        return `${rowRole} ${rowLabel(row)}, ${matrix.columnRoleLabel} ${client.label}: ${label}`;
    }
    return `${client.label}: ${label} - ${rowLabel(row)}`;
}

function statusLabel(status) {
    switch (status) {
        case 'pass':
            return 'Pass';
        case 'timeout':
            return 'Timeout';
        default:
            return 'Fail';
    }
}

function summaryPill(label, value, tone) {
    const pill = $('<span />').addClass('lean-summary-pill');
    if (tone) {
        pill.addClass(`lean-summary-pill-${tone}`);
    }
    pill.append($('<span />').addClass('lean-summary-label').text(label));
    pill.append($('<span />').addClass('lean-summary-value').text(value));
    return pill;
}

function renderEmptyState(message) {
    $('#lean-client-score-section').hide();
    $('#lean-client-score-content').empty();
    $('#lean-latest-subtitle').text('');
    $('#lean-latest-summary').empty();
    $('#lean-latest-content').empty().append($('<p />').addClass('text-secondary').text(message));
}

function showError(message) {
    $('#lean-latest-error').text(message).show();
}

function testName(test) {
    return test.name || `test ${test.testIndex}`;
}

function numericTestIndex(value) {
    const parsed = Number.parseInt(value, 10);
    return Number.isNaN(parsed) ? Number.MAX_SAFE_INTEGER : parsed;
}

function parseDate(value) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? null : date;
}

function compareDates(a, b) {
    const left = a ? a.getTime() : 0;
    const right = b ? b.getTime() : 0;
    return left - right;
}
