import 'bootstrap';
import $ from 'jquery';

import * as routes from './routes.js';
import { makeLink } from './html.js';

// Initialize theme
const storedTheme = localStorage.getItem('theme') || (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
document.documentElement.classList.add(storedTheme);
document.documentElement.setAttribute('data-bs-theme', storedTheme);

// Theme toggle handler
$(document).ready(function() {
    $('.theme-toggle').text(storedTheme === 'dark' ? '☀️' : '🌙');
    $('.theme-toggle').on('click', function() {
        const currentTheme = document.documentElement.classList.contains('dark') ? 'dark' : 'light';
        const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
        document.documentElement.classList.toggle('dark', newTheme === 'dark');
        document.documentElement.setAttribute('data-bs-theme', newTheme);
        localStorage.setItem('theme', newTheme);
        $('.theme-toggle').text(newTheme === 'dark' ? '☀️' : '🌙');
    });
});

// updateHeader populates the page header with version information from hive.json.
export function updateHeader() {
    $.ajax({
        type: 'GET',
        url: routes.resultsRoot + 'hive.json',
        dataType: 'json',
        cache: false,
        success: function(data) {
            console.log('hive.json:', data);
            $('#hive-instance-info').html(hiveInfoHTML(data));
        },
        error: function(xhr, status, error) {
            console.log('error fetching hive.json:', error);
        },
    });
}

function hiveInfoHTML(data) {
    var txt = '';
    if (data.buildDate) {
        let date = new Date(data.buildDate).toLocaleString();
        txt += '<span>hiveview (UI) built: ' + date + '</span>';
    }
    if (data.sourceCommit) {
        let url = 'https://github.com/ethereum/hive/commits/' + escape(data.sourceCommit);
        let link = makeLink(url, data.sourceCommit.substring(0, 8));
        txt += '<span>commit: ' + link.outerHTML + '</span>';
    }
    return txt;
}

// newXhrWithProgressBar creates an XMLHttpRequest and shows its progress
// in the 'load-progress-bar-container' element.
export function newXhrWithProgressBar() {
    let xhr = new window.XMLHttpRequest();
    xhr.addEventListener('progress', function(evt) {
        if (evt.lengthComputable) {
            showLoadProgress(evt.loaded / evt.total);
        } else {
            showLoadProgress(true);
        }
    });
    xhr.addEventListener('loadend', function(evt) {
        showLoadProgress(false);
    });
    return xhr;
}

export function showLoadProgress(loadState, element) {
    if (!element) {
        element = $('#load-progress-bar-container');
    } else {
        element = $(element);
    }

    if (!loadState) {
        console.log('load finished');
        element.hide();
        return;
    }

    var animated = false;
    if (typeof loadState == 'boolean') {
        loadState = 1.0;
        animated = true;
    }
    let percent = Math.floor(loadState * 100);
    console.log('loading: ' + percent);

    element.show();
    let bar = $('.progress-bar', element);
    bar.toggleClass('progress-bar-animated', animated);
    bar.toggleClass('progress-bar-striped', animated);
    bar.attr('aria-valuenow', '' + percent);
    bar.width('' + percent + '%');
}
