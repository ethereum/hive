import $ from 'jquery';

import * as routes from './routes.js';
import { formatDuration } from './utils.js';

const listingURL = 'listing.jsonl?limit=1000';
const preferredSuiteOrder = ['client-interop', 'rpc-compat', 'sync', 'validation', 'gossip', 'reqresp'];
const trendPointLimitOptions = [5, 10, 25, 50];
const trendAllClientsValue = '__all__';
const trendColorHashSeed = 13663;
const trendClientPalette = [
    '#0072B2', '#D55E00', '#009E73', '#CC79A7',
    '#E69F00', '#56B4E9', '#F0E442', '#6A3D9A',
    '#B15928', '#E7298A', '#1B9E77', '#7570B3',
    '#E6AB02', '#A6761D', '#66A61E', '#E41A1C',
    '#377EB8', '#4DAF4A', '#984EA3', '#FF7F00',
    '#A65628', '#F781BF', '#999999', '#17BECF',
    '#BCBD22', '#8C564B', '#9467BD', '#2CA02C',
    '#DB2777', '#0891B2', '#F97316', '#7C3AED',
];
const leanLatestState = {
    entries: [],
    devnets: [],
    selectedDevnet: '',
    suiteRuns: new Map(),
    selectedRunIndexes: new Map(),
    simLogRuns: [],
    selectedSimLog: '',
    collapsedSuites: new Set(),
    suiteDataCache: new Map(),
    trendClient: '',
    trendSuite: '',
    trendPointLimit: trendPointLimitOptions[0],
    displayPreviousRunsForBlankGrids: false,
    trendClientOptions: [],
    trendRenderID: 0,
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
    leanLatestState.selectedRunIndexes = new Map();
    leanLatestState.selectedSimLog = '';
    leanLatestState.trendClient = '';
    leanLatestState.trendSuite = '';
    if (updateURL) {
        updateDevnetURL(leanLatestState.selectedDevnet);
    }
    renderDevnetControls(leanLatestState.devnets, leanLatestState.selectedDevnet);

    const entries = entriesForDevnet(leanLatestState.entries, leanLatestState.selectedDevnet);
    leanLatestState.suiteRuns = suiteRunEntries(entries);
    leanLatestState.simLogRuns = simLogRunEntries(entries);
    if (leanLatestState.suiteRuns.size === 0) {
        const suffix = leanLatestState.selectedDevnet ? ` for ${leanLatestState.selectedDevnet}` : '';
        renderEmptyState(`No lean suite runs found${suffix}.`);
        return;
    }

    applySimLogSelection(leanLatestState.simLogRuns[0]?.simLog || '');
    await renderSelectedRuns();
}

async function renderSelectedRuns() {
    const matrixGroups = await Promise.all(Array.from(leanLatestState.suiteRuns.entries()).map(([suiteName, runs]) => {
        return leanLatestState.displayPreviousRunsForBlankGrids
            ? loadDisplayMatricesForSuite(suiteName, runs)
            : loadSelectedMatricesForSuite(suiteName, runs);
    }));
    await renderLeanLatest(matrixGroups.flat().filter(Boolean), leanLatestState.selectedDevnet);
}

async function loadSelectedMatricesForSuite(suiteName, runs) {
    const runIndex = selectedRunIndex(suiteName);
    if (runIndex >= runs.length) {
        return [blankSuiteMatrix(suiteName, runs, runIndex)];
    }
    return loadSuiteMatricesForRun(runs[runIndex], runs, runIndex);
}

async function loadDisplayMatricesForSuite(suiteName, runs) {
    const fallbackRuns = scoreFallbackRuns(suiteName, runs);
    if (fallbackRuns.length === 0) {
        return [blankSuiteMatrix(suiteName, runs, selectedRunIndex(suiteName))];
    }

    const entry = fallbackRuns[0];
    const runIndex = runs.findIndex(run => run.fileName === entry.fileName);
    return loadSuiteMatricesForRun(entry, runs, runIndex === -1 ? selectedRunIndex(suiteName) : runIndex);
}

async function loadSuiteMatricesForRun(entry, runHistory, runIndex) {
    const matrices = await loadSuiteMatrices(entry);
    return matrices.map(matrix => ({
        ...matrix,
        runHistory,
        runIndex,
        runSuiteName: entry.name || matrix.linkSuiteName || matrix.suiteName,
    }));
}

function blankSuiteMatrix(suiteName, runHistory, runIndex) {
    const devnet = leanLatestState.selectedDevnet ? ` for ${leanLatestState.selectedDevnet}` : '';
    const simLogRun = selectedSimLogRun();
    const missingRunLabel = simLogRun ? 'Not run in selected simulator run' : `Run ${runIndex + 1} unavailable`;
    const emptyMessage = simLogRun
        ? `No ${suiteName} run found in simulator run ${simLogRunDateLabel(simLogRun)}.`
        : `No run ${runIndex + 1} found${devnet}.`;
    return {
        entry: null,
        suiteData: null,
        suiteName,
        linkSuiteName: suiteName,
        clients: [],
        cases: [],
        rowHeaderLabel: 'Test',
        rows: [],
        stats: {
            total: 0,
            passed: 0,
            failed: 0,
            timeouts: 0,
            start: null,
            end: null,
            duration: null,
        },
        runHistory,
        runIndex,
        runSuiteName: suiteName,
        missingRun: true,
        missingRunLabel,
        emptyMessage,
    };
}

function selectedRunIndex(suiteName) {
    const index = leanLatestState.selectedRunIndexes.get(suiteName) || 0;
    if (!Number.isInteger(index) || index < 0) {
        return 0;
    }
    return index;
}

async function selectGlobalRun(simLog) {
    if (!simLog || !leanLatestState.simLogRuns.some(run => run.simLog === simLog)) {
        return;
    }

    applySimLogSelection(simLog);
    $('#loading-container').addClass('show');
    $('#lean-latest-error').hide();
    try {
        await renderSelectedRuns();
    } catch (err) {
        showError(`Unable to load simulator run ${simLog}: ${err.message || err}`);
    } finally {
        $('#loading-container').removeClass('show');
    }
}

function applySimLogSelection(simLog) {
    leanLatestState.selectedSimLog = simLog || '';
    Array.from(leanLatestState.suiteRuns.entries()).forEach(([suiteName, runs]) => {
        const runIndex = simLog ? runs.findIndex(entry => entry.simLog === simLog) : 0;
        leanLatestState.selectedRunIndexes.set(suiteName, runIndex === -1 ? runs.length : runIndex);
    });
}

function selectedSimLogRun() {
    return leanLatestState.simLogRuns.find(run => run.simLog === leanLatestState.selectedSimLog);
}

async function loadSuiteMatrices(entry) {
    const suiteData = await loadSuiteData(entry);
    return buildSuiteMatrices(entry, suiteData);
}

async function loadSuiteData(entry) {
    const cacheKey = entry.fileName;
    if (!leanLatestState.suiteDataCache.has(cacheKey)) {
        leanLatestState.suiteDataCache.set(cacheKey, loadJSON(routes.resultsRoot + entry.fileName));
    }
    return leanLatestState.suiteDataCache.get(cacheKey);
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

function suiteRunEntries(entries) {
    const suites = new Map();
    entries.forEach(entry => {
        if (!entry.name) {
            return;
        }

        if (!suites.has(entry.name)) {
            suites.set(entry.name, []);
        }
        suites.get(entry.name).push(entry);
    });

    return new Map(Array.from(suites.entries())
        .map(([suiteName, runs]) => [
            suiteName,
            runs.sort((a, b) => compareDates(b.startDate, a.startDate) || (b.fileName || '').localeCompare(a.fileName || '')),
        ])
        .sort(([a], [b]) => compareSuiteNames(a, b)));
}

function simLogRunEntries(entries) {
    const groups = new Map();
    entries.forEach(entry => {
        const simLog = entry.simLog || entry.fileName;
        if (!simLog) {
            return;
        }

        if (!groups.has(simLog)) {
            groups.set(simLog, {
                simLog,
                date: simLogDate(simLog) || entry.startDate,
                entries: [],
            });
        }
        const group = groups.get(simLog);
        group.entries.push(entry);
        if (!group.date) {
            group.date = entry.startDate;
        }
    });

    return Array.from(groups.values()).sort((a, b) => {
        return compareDates(b.date, a.date) || b.simLog.localeCompare(a.simLog);
    });
}

function compareSuiteNames(a, b) {
    const ar = suiteRank(a);
    const br = suiteRank(b);
    if (ar !== br) {
        return ar - br;
    }
    return a.localeCompare(b);
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
            sourceFileName: entry.fileName,
            sourceSuiteName: suiteName,
            sourceLabel: scoreSourceLabel(entry),
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
            sourceFileName: entry.fileName,
            sourceSuiteName: suiteName,
            sourceLabel: scoreSourceLabel(entry),
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
    if (counts.size === 0) {
        return null;
    }
    if (counts.size === 1) {
        const [client] = counts.keys();
        return { majority: client, minority: client };
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

async function renderLeanLatest(matrices, devnet) {
    if (matrices.length === 0) {
        renderEmptyState('No lean suite runs found.');
        return;
    }

    const clientRanking = await renderClientScores(matrices);
    void renderClientTrend(clientRanking);
    applyClientRanking(matrices, clientRanking);

    const suiteNames = new Set(matrices.map(matrix => matrix.linkSuiteName || matrix.suiteName));
    const failingSuites = new Set(
        matrices
            .filter(matrix => matrix.stats.failed > 0)
            .map(matrix => matrix.linkSuiteName || matrix.suiteName)
    ).size;
    const devnetLabel = devnet ? `${devnet} ` : '';
    const latestSimLog = leanLatestState.simLogRuns[0]?.simLog || '';
    const runPrefix = leanLatestState.selectedSimLog
        ? (leanLatestState.selectedSimLog === latestSimLog ? 'Latest' : 'Selected')
        : (matrices.some(matrix => (matrix.runIndex || 0) > 0) ? 'Selected' : 'Latest');
    $('#lean-latest-subtitle').text(`${runPrefix} ${devnetLabel}runs for ${suiteNames.size} suites.`);
    $('#lean-latest-summary').empty()
        .append(renderGlobalRunSelector())
        .append(renderDisplayFallbackSwitch())
        .append(summaryPill('Suites', suiteNames.size))
        .append(summaryPill('Failing', failingSuites, failingSuites > 0 ? 'danger' : 'success'));

    const content = $('#lean-latest-content').empty();
    matrices.forEach(matrix => {
        content.append(renderSuiteSection(matrix));
    });
}

async function renderClientScores(matrices) {
    const section = $('#lean-client-score-section');
    const content = $('#lean-client-score-content').empty();
    if (!section.length) {
        return [];
    }

    const scores = await buildClientScores(matrices);
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

async function renderClientTrend(clientRanking = null) {
    const section = $('#lean-client-trend-section');
    const controls = $('#lean-client-trend-controls').empty();
    const content = $('#lean-client-trend-content').empty();
    if (!section.length) {
        return;
    }

    if (Array.isArray(clientRanking)) {
        leanLatestState.trendClientOptions = collectTrendClients(clientRanking);
    }
    const clients = leanLatestState.trendClientOptions;
    if (clients.length === 0 || leanLatestState.simLogRuns.length === 0) {
        section.hide();
        return;
    }

    if (!leanLatestState.trendClient || (leanLatestState.trendClient !== trendAllClientsValue && !clients.includes(leanLatestState.trendClient))) {
        leanLatestState.trendClient = trendAllClientsValue;
    }

    const suites = trendSuiteOptions();
    if (leanLatestState.trendSuite && !suites.includes(leanLatestState.trendSuite)) {
        leanLatestState.trendSuite = '';
    }

    section.show();
    renderTrendControls(controls, clients, suites);
    content.append($('<p />').addClass('text-secondary').text('Loading trend...'));

    const renderID = ++leanLatestState.trendRenderID;
    try {
        const trendData = await buildTrendData(leanLatestState.trendClient, leanLatestState.trendSuite, clients);
        if (renderID !== leanLatestState.trendRenderID) {
            return;
        }
        renderTrendChart(content.empty(), trendData, leanLatestState.trendClient, leanLatestState.trendSuite);
    } catch (err) {
        if (renderID !== leanLatestState.trendRenderID) {
            return;
        }
        content.empty().append($('<p />').addClass('text-danger').text(`Unable to load trend: ${err.message || err}`));
    }
}

function collectTrendClients(clientRanking) {
    const clients = [];
    const seen = new Set();
    const addClient = client => {
        if (!client || seen.has(client)) {
            return;
        }
        seen.add(client);
        clients.push(client);
    };

    clientRanking.forEach(addClient);
    entriesForDevnet(leanLatestState.entries, leanLatestState.selectedDevnet).forEach(entry => {
        (entry.clients || []).forEach(addClient);
        Object.keys(entry.versions || {}).forEach(addClient);
    });
    return clients;
}

function trendSuiteOptions() {
    return Array.from(leanLatestState.suiteRuns.keys());
}

function renderTrendControls(controls, clients, suites) {
    controls.append(renderTrendSelect('Client', 'Client trend client', leanLatestState.trendClient, [
        { value: trendAllClientsValue, label: 'all' },
        ...clients.map(client => ({
            value: client,
            label: client,
        })),
    ], value => {
        leanLatestState.trendClient = value;
        void renderClientTrend();
    }));

    controls.append(renderTrendSelect('Suite', 'Client trend score source', leanLatestState.trendSuite, [
        { value: '', label: 'Total' },
        ...suites.map(suiteName => ({ value: suiteName, label: suiteName })),
    ], value => {
        leanLatestState.trendSuite = value;
        void renderClientTrend();
    }));

    controls.append(renderTrendSelect('Runs', 'Client trend simlog point count', String(leanLatestState.trendPointLimit), trendPointLimitOptions.map(count => ({
        value: String(count),
        label: String(count),
    })), value => {
        const nextLimit = Number(value);
        leanLatestState.trendPointLimit = trendPointLimitOptions.includes(nextLimit)
            ? nextLimit
            : trendPointLimitOptions[0];
        void renderClientTrend();
    }));
}

function renderTrendSelect(labelText, ariaLabel, value, options, onChange) {
    const label = $('<label />').addClass('lean-trend-select');
    const select = $('<select />')
        .addClass('form-select form-select-sm')
        .attr('aria-label', ariaLabel);

    options.forEach(option => {
        select.append($('<option />').val(option.value).text(option.label));
    });
    select.val(value);
    select.on('change', function() {
        onChange($(this).val());
    });

    label.append($('<span />').text(labelText));
    label.append(select);
    return label;
}

async function buildTrendData(client, suiteName, clients) {
    if (client === trendAllClientsValue) {
        return buildAllClientTrendData(clients, suiteName);
    }

    const points = await buildTrendPoints(client, suiteName);
    const runs = points.map(point => point.run);
    return {
        runs,
        series: [{
            client,
            points: points.map((point, index) => ({
                ...point,
                runIndex: index,
            })),
        }],
    };
}

async function buildTrendPoints(client, suiteName) {
    const points = [];
    const pointLimit = leanLatestState.trendPointLimit || trendPointLimitOptions[0];
    for (const run of trendCandidateRuns(suiteName)) {
        const entries = suiteName
            ? run.entries.filter(entry => entry.name === suiteName)
            : run.entries;
        const score = await scoreClientForEntries(entries, client);
        if (score.total === 0) {
            continue;
        }

        points.push({ run, score });
        if (points.length >= pointLimit) {
            break;
        }
    }
    return points;
}

async function buildAllClientTrendData(clients, suiteName) {
    const runs = [];
    const seriesByClient = new Map(clients.map(client => [client, []]));
    const pointLimit = leanLatestState.trendPointLimit || trendPointLimitOptions[0];
    for (const run of trendCandidateRuns(suiteName)) {
        const entries = suiteName
            ? run.entries.filter(entry => entry.name === suiteName)
            : run.entries;
        const scores = await scoreClientsForEntries(entries, clients);
        const scoredClients = clients.filter(client => (scores.get(client)?.total || 0) > 0);
        if (scoredClients.length === 0) {
            continue;
        }

        const runIndex = runs.length;
        runs.push(run);
        scoredClients.forEach(client => {
            seriesByClient.get(client).push({
                run,
                runIndex,
                score: scores.get(client),
            });
        });
        if (runs.length >= pointLimit) {
            break;
        }
    }

    return {
        runs,
        series: clients.map(client => ({
            client,
            points: seriesByClient.get(client) || [],
        })).filter(series => series.points.length > 0),
    };
}

function trendCandidateRuns(suiteName) {
    return suiteName
        ? leanLatestState.simLogRuns.filter(run => run.entries.some(entry => entry.name === suiteName))
        : leanLatestState.simLogRuns;
}

async function scoreClientForEntries(entries, client) {
    const matrixGroups = await Promise.all(entries.map(loadSuiteMatrices));
    const score = emptyScore();
    matrixGroups.flat().forEach(matrix => {
        const suiteName = matrix.linkSuiteName || matrix.suiteName;
        matrix.cases.forEach(test => {
            if (isSuiteSetupTest(test, suiteName)) {
                return;
            }
            if (scoreClientNamesForTest(test).includes(client)) {
                incrementScore(score, test);
            }
        });
    });
    return score;
}

async function scoreClientsForEntries(entries, clients) {
    const clientSet = new Set(clients);
    const scores = new Map(clients.map(client => [client, emptyScore()]));
    const matrixGroups = await Promise.all(entries.map(loadSuiteMatrices));
    matrixGroups.flat().forEach(matrix => {
        const suiteName = matrix.linkSuiteName || matrix.suiteName;
        matrix.cases.forEach(test => {
            if (isSuiteSetupTest(test, suiteName)) {
                return;
            }
            scoreClientNamesForTest(test).forEach(client => {
                if (clientSet.has(client)) {
                    incrementScore(scores.get(client), test);
                }
            });
        });
    });
    return scores;
}

function renderTrendChart(content, trendData, selectedClient, suiteName) {
    if (trendData.runs.length === 0 || trendData.series.length === 0) {
        content.append($('<p />').addClass('text-secondary').text('No trend data available.'));
        return;
    }

    const source = suiteName || 'Total score';
    const clientLabel = selectedClient === trendAllClientsValue ? 'all clients' : selectedClient;
    content.append($('<p />')
        .addClass('lean-trend-summary')
        .text(`${clientLabel} - ${source} (Newest to Oldest)`));

    const chart = $('<div />').addClass('lean-trend-chart');
    const yAxis = $('<div />').addClass('lean-trend-y-axis');
    [100, 75, 50, 25, 0].forEach(percent => {
        yAxis.append($('<span />').text(`${percent}%`));
    });

    const axisWidth = trendAxisWidth(trendData.runs.length);
    const plot = $('<div />')
        .addClass('lean-trend-plot')
        .css('width', axisWidth);
    plot.append(renderTrendGridLines());
    plot.append(renderTrendLinePlot(trendData.series, trendData.runs, clientLabel));

    const xAxis = $('<div />')
        .addClass('lean-trend-x-axis')
        .css('width', axisWidth);
    const xAxisInner = $('<div />').addClass('lean-trend-x-axis-inner');
    trendData.runs.forEach((run, index) => {
        xAxisInner.append($('<span />')
            .addClass('lean-trend-x-label')
            .css('left', `${trendPointX(index, trendData.runs.length)}%`)
            .attr('title', simLogRunDateTitle(run))
            .text(simLogRunDateLabel(run)));
    });
    xAxis.append(xAxisInner);

    chart.append(yAxis, plot, $('<div />'), xAxis);
    content.append(chart);
    content.append(renderTrendLegend(trendData.series));
}

function trendAxisWidth(pointCount) {
    const labelWidthRem = 5.75;
    const labelGapRem = 0.5;
    return `max(100%, ${(pointCount * labelWidthRem) + (Math.max(pointCount - 1, 0) * labelGapRem)}rem)`;
}

function renderTrendGridLines() {
    const svg = $(svgElement('svg', {
        class: 'lean-trend-grid-svg',
        viewBox: '0 0 100 100',
        preserveAspectRatio: 'none',
        'aria-hidden': 'true',
        focusable: 'false',
    }));
    [25, 50, 75].forEach(y => {
        svg.append(svgElement('line', {
            class: 'lean-trend-grid-line',
            x1: '0',
            y1: String(y),
            x2: '100',
            y2: String(y),
        }));
    });
    return svg;
}

function renderTrendLinePlot(seriesList, runs, label) {
    const plot = $('<div />')
        .addClass('lean-trend-line-plot')
        .attr('role', 'img')
        .attr('aria-label', `${label} passing percentage trend`);
    const svg = $(svgElement('svg', {
        class: 'lean-trend-line-svg',
        viewBox: '0 0 100 100',
        preserveAspectRatio: 'none',
        'aria-hidden': 'true',
        focusable: 'false',
    }));
    const showValueLabels = seriesList.length === 1;
    const renderSeries = seriesList.map(series => ({
        ...series,
        points: series.points.map(point => {
            const percent = Math.round(scorePercent(point.score) * 100);
            return {
                ...point,
                percent,
                coords: trendPointCoordinates(point.runIndex, runs.length, percent),
            };
        }),
    }));
    const dotGroups = trendDotGroups(renderSeries);

    renderSeries.forEach(series => {
        const linePoints = [];
        const color = clientTrendColor(series.client);
        series.points.forEach(point => {
            const score = point.score;
            const percent = point.percent;
            const coords = point.coords;
            const labelAbove = coords.y >= 14;
            linePoints.push(coords);
            if (showValueLabels) {
                plot.append($('<span />')
                    .addClass('lean-trend-value')
                    .toggleClass('below', !labelAbove)
                    .css('left', `${coords.x}%`)
                    .css('top', `${coords.y}%`)
                    .text(`${percent}%`));
            }
            plot.append($('<span />')
                .addClass('lean-trend-dot')
                .toggleClass('has-failures', score.failed > 0)
                .css('--lean-trend-client-color', color)
                .css('left', `${coords.x}%`)
                .css('top', `${coords.y}%`)
                .attr('title', trendDotTitle(dotGroups.get(trendDotKey(point)))));
        });

        if (linePoints.length > 1) {
            const line = svgElement('polyline', {
                class: 'lean-trend-line',
                points: linePoints.map(point => `${point.x},${point.y}`).join(' '),
            });
            line.style.setProperty('--lean-trend-client-color', color);
            svg.append(line);
        }
    });
    plot.prepend(svg);
    return plot;
}

function trendDotGroups(seriesList) {
    const groups = new Map();
    seriesList.forEach(series => {
        series.points.forEach(point => {
            const key = trendDotKey(point);
            if (!groups.has(key)) {
                groups.set(key, []);
            }
            groups.get(key).push({
                client: series.client,
                percent: point.percent,
                score: point.score,
            });
        });
    });
    return groups;
}

function trendDotKey(point) {
    return `${point.runIndex}:${point.percent}`;
}

function trendDotTitle(group = []) {
    if (group.length === 0) {
        return '';
    }
    if (group.length === 1) {
        const point = group[0];
        return `${point.client}: ${point.percent}% - ${point.score.passed}/${point.score.total} tests passed`;
    }
    return [
        `${group[0].percent}%`,
        ...group.map(point => `${point.client}: ${point.score.passed}/${point.score.total} tests passed`),
    ].join('\n');
}

function renderTrendLegend(seriesList) {
    const legend = $('<div />')
        .addClass('lean-trend-legend')
        .attr('aria-label', 'Client color legend');
    seriesList.slice().sort(compareTrendSeriesByClient).forEach(series => {
        legend.append($('<span />')
            .addClass('lean-trend-legend-item')
            .css('--lean-trend-client-color', clientTrendColor(series.client))
            .append($('<span />').addClass('lean-trend-legend-swatch'))
            .append($('<span />').text(series.client)));
    });
    return legend;
}

function compareTrendSeriesByClient(a, b) {
    return a.client.localeCompare(b.client);
}

function clientTrendColor(client) {
    const hash = trendColorHash(clientBaseName(client));
    return trendClientPalette[hash % trendClientPalette.length];
}

function clientBaseName(client) {
    const name = (client || '').trim();
    const match = name.match(/^(.*?)(?:[_-]?devnet[0-9][a-z0-9_-]*)$/i);
    if (match && match[1]) {
        return match[1].replace(/[_-]+$/, '').toLowerCase();
    }
    return name.toLowerCase();
}

function hashString(value) {
    let hash = 2166136261;
    for (let index = 0; index < value.length; index++) {
        hash ^= value.charCodeAt(index);
        hash = Math.imul(hash, 16777619);
    }
    return hash >>> 0;
}

function trendColorHash(value) {
    let hash = (hashString(value) ^ trendColorHashSeed) >>> 0;
    hash ^= hash >>> 16;
    hash = Math.imul(hash, 0x85ebca6b);
    hash ^= hash >>> 13;
    hash = Math.imul(hash, 0xc2b2ae35);
    hash ^= hash >>> 16;
    return hash >>> 0;
}

function trendPointCoordinates(index, total, percent) {
    return {
        x: trendPointX(index, total),
        y: 100 - percent,
    };
}

function trendPointX(index, total) {
    return total <= 1 ? 50 : index * (100 / (total - 1));
}

function svgElement(name, attrs = {}) {
    const element = document.createElementNS('http://www.w3.org/2000/svg', name);
    Object.entries(attrs).forEach(([key, value]) => {
        element.setAttribute(key, value);
    });
    return element;
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
            return diff !== 0 ? diff : (b.label || b.name).localeCompare(a.label || a.name);
        });

        if (matrix.rowRoleLabel) {
            matrix.rows = matrix.rows.slice().sort((a, b) => {
                const diff = rankOf(a.name) - rankOf(b.name);
                return diff !== 0 ? diff : b.name.localeCompare(a.name);
            });
        }
    });
}

async function buildClientScores(matrices) {
    const clients = [];
    const seenClients = new Set();
    const rows = [];
    const rowsBySuite = new Map();
    const totals = new Map();
    const matrixCache = scoreMatrixCache(matrices);
    const targetClients = collectScoreTargetClients(matrices);

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

    const suiteNames = Array.from(leanLatestState.suiteRuns.keys());
    if (suiteNames.length === 0) {
        matrices.forEach(matrix => {
            const suiteName = matrix.linkSuiteName || matrix.suiteName;
            if (!rowsBySuite.has(suiteName)) {
                const row = {
                    suiteName,
                    fileName: matrix.entry ? matrix.entry.fileName : '',
                    linkSuiteName: matrix.linkSuiteName || matrix.suiteName,
                    cells: new Map(),
                };
                rowsBySuite.set(suiteName, row);
                rows.push(row);
            }
            const row = rowsBySuite.get(suiteName);

            addMatrixScoresToRow(row, [matrix], addClient, scoreFor, totals, matrix.entry, targetClients);
        });
        clients.sort((a, b) => compareClientScores(totals, a, b));
        return { clients, rows, totals };
    }

    for (const suiteName of suiteNames) {
        if (!rowsBySuite.has(suiteName)) {
            const row = {
                suiteName,
                fileName: '',
                linkSuiteName: suiteName,
                cells: new Map(),
            };
            rowsBySuite.set(suiteName, row);
            rows.push(row);
        }
        const row = rowsBySuite.get(suiteName);
        const runs = leanLatestState.suiteRuns.get(suiteName) || [];

        for (const entry of scoreFallbackRuns(suiteName, runs)) {
            const scoreMatrices = await loadScoreMatrices(entry, matrixCache);
            addMatrixScoresToRow(row, scoreMatrices, addClient, scoreFor, totals, entry, targetClients);
            if (scoreRowComplete(row, targetClients)) {
                break;
            }
        }
    }

    clients.sort((a, b) => compareClientScores(totals, a, b));
    return { clients, rows, totals };
}

function collectScoreTargetClients(matrices) {
    const clients = new Set();
    entriesForDevnet(leanLatestState.entries, leanLatestState.selectedDevnet).forEach(entry => {
        (entry.clients || []).forEach(client => clients.add(client));
        Object.keys(entry.versions || {}).forEach(client => clients.add(client));
    });
    matrices.forEach(matrix => {
        matrix.clients.forEach(client => clients.add(client.name));
        matrix.cases.forEach(test => {
            scoreClientNamesForTest(test).forEach(client => clients.add(client));
        });
    });
    return clients;
}

function scoreRowComplete(row, targetClients) {
    if (targetClients.size === 0) {
        return false;
    }
    return Array.from(targetClients).every(client => {
        const score = row.cells.get(client);
        return score && score.total > 0;
    });
}

function scoreMatrixCache(matrices) {
    const cache = new Map();
    matrices.forEach(matrix => {
        if (!matrix.entry || !matrix.entry.fileName) {
            return;
        }
        if (!cache.has(matrix.entry.fileName)) {
            cache.set(matrix.entry.fileName, []);
        }
        cache.get(matrix.entry.fileName).push(matrix);
    });
    return cache;
}

async function loadScoreMatrices(entry, matrixCache) {
    if (matrixCache.has(entry.fileName)) {
        return matrixCache.get(entry.fileName);
    }
    const matrices = await loadSuiteMatrices(entry);
    matrixCache.set(entry.fileName, matrices);
    return matrices;
}

function scoreFallbackRuns(suiteName, runs) {
    if (runs.length === 0) {
        return [];
    }

    const runIndex = selectedRunIndex(suiteName);
    if (runIndex < runs.length) {
        return runs.slice(runIndex);
    }

    const selectedRun = selectedSimLogRun();
    if (!selectedRun || !selectedRun.date) {
        return runs.slice(0);
    }

    const fallbackIndex = runs.findIndex(entry => compareDates(scoreEntryDate(entry), selectedRun.date) <= 0);
    return fallbackIndex === -1 ? [] : runs.slice(fallbackIndex);
}

function scoreEntryDate(entry) {
    return simLogDate(entry.simLog) || entry.startDate;
}

function addMatrixScoresToRow(row, matrices, addClient, scoreFor, totals, entry, targetClients) {
    const sourceScores = clientScoresForMatrices(matrices);
    sourceScores.forEach((sourceScore, client) => {
        if (targetClients.size > 0 && !targetClients.has(client)) {
            return;
        }
        addClient(client);
        const cell = scoreFor(row.cells, client);
        if (cell.total > 0 || sourceScore.total === 0) {
            return;
        }

        const matrix = sourceScore.matrix;
        cell.passed = sourceScore.passed;
        cell.failed = sourceScore.failed;
        cell.total = sourceScore.total;
        cell.fileName = entry ? entry.fileName : (matrix.entry ? matrix.entry.fileName : '');
        cell.linkSuiteName = matrix.linkSuiteName || matrix.suiteName || row.linkSuiteName;
        cell.sourceLabel = scoreSourceLabel(entry || matrix.entry);
        incrementScoreBy(totals.get(client), sourceScore);
        if (!row.fileName) {
            row.fileName = cell.fileName;
        }
    });
}

function clientScoresForMatrices(matrices) {
    const scores = new Map();
    matrices.forEach(matrix => {
        const suiteName = matrix.linkSuiteName || matrix.suiteName;
        clientScoresForMatrix(matrix, suiteName).forEach((sourceScore, client) => {
            if (!scores.has(client)) {
                scores.set(client, {
                    ...emptyScore(),
                    matrix,
                });
            }
            incrementScoreBy(scores.get(client), sourceScore);
        });
    });
    return scores;
}

function clientScoresForMatrix(matrix, suiteName) {
    const scores = new Map();
    const scoreForClient = client => {
        if (!scores.has(client)) {
            scores.set(client, emptyScore());
        }
        return scores.get(client);
    };

    matrix.cases.forEach(test => {
        if (isSuiteSetupTest(test, suiteName)) {
            return;
        }

        scoreClientNamesForTest(test).forEach(client => {
            incrementScore(scoreForClient(client), test);
        });
    });
    return scores;
}

function incrementScoreBy(score, sourceScore) {
    score.passed += sourceScore.passed;
    score.failed += sourceScore.failed;
    score.total += sourceScore.total;
}

function scoreSourceLabel(entry) {
    if (!entry) {
        return '';
    }
    return formatDateLabel(scoreEntryDate(entry), entry.simLog || entry.start || entry.fileName);
}

function suiteClientURL(row, client) {
    const cell = row.cells.get(client);
    const fileName = cell?.fileName || row.fileName;
    const suiteName = cell?.linkSuiteName || row.linkSuiteName;
    if (!fileName || !suiteName) {
        return '';
    }
    return `${routes.suite(fileName, suiteName)}&client=${encodeURIComponent(client)}`;
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
    return b.localeCompare(a);
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
    const source = score.sourceLabel ? ` from ${score.sourceLabel}` : '';
    value.addClass(statusClass)
        .attr('title', `${score.passed} passed, ${score.failed} failed${source}`)
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
    const suiteKey = suiteCollapseKey(matrix);
    const hasGrid = !matrix.missingRun && matrix.rows.length > 0 && matrix.clients.length > 0;
    const collapsed = hasGrid && leanLatestState.collapsedSuites.has(suiteKey);

    title.append($('<h2 />').text(matrix.suiteName));
    title.append(renderSuiteMeta(matrix));

    const statusClass = matrix.stats.failed > 0 ? 'bg-danger' : 'bg-success';
    const statusText = matrix.stats.failed > 0 ? 'Fail' : 'Pass';
    if (hasGrid) {
        actions.append(renderCollapseButton(suiteKey, collapsed));
    }
    if (!matrix.missingRun) {
        actions.append($('<span />').addClass(`badge ${statusClass}`).text(statusText));
        actions.append($('<a />')
            .addClass('btn btn-sm btn-secondary')
            .attr('href', routes.suite(matrix.entry.fileName, matrix.linkSuiteName || matrix.suiteName))
            .text('Open suite'));
    }

    header.append(title, actions);
    section.append(header);
    section.toggleClass('is-collapsed', collapsed);

    const body = $('<div />').addClass('lean-suite-body').prop('hidden', collapsed);
    section.append(body);

    if (matrix.missingRun || matrix.rows.length === 0 || matrix.clients.length === 0) {
        body.append($('<p />').addClass('text-secondary').text(matrix.emptyMessage || 'No test cases with client results.'));
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
    body.append(scroll);

    return section;
}

function suiteCollapseKey(matrix) {
    return matrix.linkSuiteName || matrix.suiteName;
}

function renderCollapseButton(suiteKey, collapsed) {
    const button = $('<button />')
        .attr('type', 'button')
        .addClass('btn btn-sm btn-secondary lean-collapse-button');
    updateCollapseButton(button, collapsed);

    button.on('click', function() {
        const nextCollapsed = !leanLatestState.collapsedSuites.has(suiteKey);
        if (nextCollapsed) {
            leanLatestState.collapsedSuites.add(suiteKey);
        } else {
            leanLatestState.collapsedSuites.delete(suiteKey);
        }

        const section = $(this).closest('.lean-suite-section');
        section.toggleClass('is-collapsed', nextCollapsed);
        section.children('.lean-suite-body').prop('hidden', nextCollapsed);
        updateCollapseButton($(this), nextCollapsed);
    });

    return button;
}

function updateCollapseButton(button, collapsed) {
    button
        .attr('aria-expanded', collapsed ? 'false' : 'true')
        .attr('aria-label', collapsed ? 'Expand grid' : 'Collapse grid')
        .attr('title', collapsed ? 'Expand grid' : 'Collapse grid')
        .empty()
        .append($('<i />').addClass(collapsed ? 'bi bi-chevron-down' : 'bi bi-chevron-up'));
}

function renderGlobalRunSelector() {
    const simLogRuns = leanLatestState.simLogRuns;
    if (simLogRuns.length === 0) {
        return $();
    }

    const label = $('<label />').addClass('lean-run-selector lean-global-run-selector');
    const select = $('<select />')
        .addClass('form-select form-select-sm')
        .attr('aria-label', 'Set simulator run for all suites');
    const selectedRun = selectedSimLogRun();

    if (!selectedRun) {
        select.append($('<option />').val('').text('Mixed').prop('disabled', true));
    }
    simLogRuns.forEach(run => {
        select.append($('<option />')
            .val(run.simLog)
            .text(simLogRunDateLabel(run))
            .attr('title', simLogRunDateTitle(run)));
    });

    select.val(selectedRun ? selectedRun.simLog : '');
    select.attr('title', selectedRun ? simLogRunDateTitle(selectedRun) : 'Suite run selections differ');
    select.on('change', function() {
        selectGlobalRun($(this).val());
    });

    label.append($('<span />').text('Run'));
    label.append(select);
    return label;
}

function renderDisplayFallbackSwitch() {
    const labelText = 'display previous simulator runs for blank grids';
    const wrapper = $('<label />').addClass('form-check form-switch lean-display-fallback-switch');
    const input = $('<input />')
        .addClass('form-check-input')
        .attr('type', 'checkbox')
        .attr('role', 'switch')
        .attr('aria-label', labelText)
        .prop('checked', leanLatestState.displayPreviousRunsForBlankGrids)
        .on('change', function() {
            selectDisplayFallback($(this).prop('checked'));
        });

    wrapper.append(input);
    wrapper.append($('<span />').addClass('form-check-label').text(labelText));
    return wrapper;
}

async function selectDisplayFallback(enabled) {
    if (leanLatestState.displayPreviousRunsForBlankGrids === enabled) {
        return;
    }

    leanLatestState.displayPreviousRunsForBlankGrids = enabled;
    $('#loading-container').addClass('show');
    $('#lean-latest-error').hide();
    try {
        await renderSelectedRuns();
    } catch (err) {
        showError(`Unable to update grid display: ${err.message || err}`);
    } finally {
        $('#loading-container').removeClass('show');
    }
}

function simLogRunDateTitle(run) {
    const label = simLogRunDateLabel(run);
    return run.simLog ? `Simulator run: ${label} - ${run.simLog}` : label;
}

function simLogRunDateLabel(run) {
    if (!run) {
        return 'Unavailable';
    }
    return formatDateLabel(run.date, run.simLog || 'Unavailable');
}

function simLogDate(simLog) {
    const match = (simLog || '').match(/^([0-9]+)-simulator-/);
    if (!match) {
        return null;
    }
    const seconds = Number.parseInt(match[1], 10);
    if (!Number.isFinite(seconds)) {
        return null;
    }
    return new Date(seconds * 1000);
}

function formatDateLabel(start, fallback) {
    if (!start) {
        return fallback || 'Unavailable';
    }

    const pad = value => String(value).padStart(2, '0');
    return [
        start.getFullYear(),
        pad(start.getMonth() + 1),
        pad(start.getDate()),
    ].join('/') + ` ${pad(start.getHours())}:${pad(start.getMinutes())}:${pad(start.getSeconds())}`;
}

function renderSuiteMeta(matrix) {
    const meta = $('<div />').addClass('lean-suite-meta');
    if (matrix.missingRun) {
        meta.append($('<span />').text(matrix.missingRunLabel || `Run ${matrix.runIndex + 1} unavailable`));
        return meta;
    }

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
    const url = routes.testInSuite(
        linkedTest.sourceFileName || matrix.entry.fileName,
        linkedTest.sourceSuiteName || matrix.linkSuiteName || matrix.suiteName,
        linkedTest.testIndex
    );
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
    const source = resultSourceSuffix(matrix, tests);
    if (matrix.columnRoleLabel && row.name === client.name) {
        return `${row.name} (3 nodes): ${label}${source}`;
    }
    if (matrix.columnRoleLabel) {
        const rowRole = matrix.rowRoleLabel || matrix.rowHeaderLabel;
        return `${rowRole} ${rowLabel(row)}, ${matrix.columnRoleLabel} ${client.label}: ${label}${source}`;
    }
    return `${client.label}: ${label} - ${rowLabel(row)}${source}`;
}

function resultSourceSuffix(matrix, tests) {
    const linkedTest = preferredLinkedTest(tests);
    if (!linkedTest || !linkedTest.sourceLabel) {
        return '';
    }
    const matrixSource = scoreSourceLabel(matrix.entry);
    return linkedTest.sourceLabel === matrixSource ? '' : ` from ${linkedTest.sourceLabel}`;
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
    $('#lean-client-trend-section').hide();
    $('#lean-client-trend-controls').empty();
    $('#lean-client-trend-content').empty();
    leanLatestState.trendRenderID++;
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
