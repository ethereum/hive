// formatDuration formats a duration value (given in ms).
export function formatDuration(dur) {
    var _s = '';
    if (dur < 0) {
        _s = '-';
        dur = -dur;
    }
    var d = Math.floor(dur / 86400000);
    dur %= 86400000;
    var h = Math.floor(dur / 3600000);
    dur %= 3600000;
    var m = Math.floor(dur / 60000);
    dur %= 60000;
    var s = Math.floor(dur / 1000);

    var a = d ? (' ' + d + 'd') : '';
    a += ((a || h) ? (' ' + h + 'h') : '');
    a += ((a || m) ? (' ' + m + 'min') : '');
    a += s + 's';
    return _s + a;
}

// formatBytes returns human readable units for the given data size in bytes.
export function formatBytes(loc) {
    if (loc < 1024) {
        return loc + 'B';
    }
    loc = loc / 1024;
    if (loc < 1024) {
        return loc.toFixed(2) + 'KB';
    }
    loc = loc / 1024;
    return loc.toFixed(2) + 'MB';
}

// queryParam returns the value of 'key' in the URL query.
export function queryParam(key) {
    return new URLSearchParams(document.location.search).get(key);
}
