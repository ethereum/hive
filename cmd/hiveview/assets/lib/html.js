// encode does HTML-encoding/escaping.
export function encode(str) {
    let d = document.createElement('textarea');
    d.innerText = str;
    return d.innerHTML;
}

// urlsToLinks replaces URLs in input with HTML links.
export function urlsToLinks(str) {
    // Thanks, http://urlregex.com/
    let re = /(((?:http[s]?:\/\/)(?:[\w;:&=+$,-]+@)?[A-Za-z0-9.-]+|(?:www\.|[\w;:&=+$,]+@)[A-Za-z0-9.-]+)((?:\/[\w+~%/._-]*)?\??(?:[\w+=&;%@._-]*)#?(?:[\w.!/\\]*))?)/;
    return String(str).replace(re, function (match) {
        return makeLink(match, match).outerHTML;
    });
}

// makeLink creates an anchor-element from 'untrusted' link data.
export function makeLink(url, text) {
    let a = document.createElement('a');
    a.setAttribute('href', url);
    a.text = text;
    return a;
}

// makeButton creates a button-shaped link element.
export function makeButton(url, text) {
    let a = makeLink(url, text);
    a.setAttribute('class', 'btn btn-primary btn-sm');
    return a;
}

// Takes { "a": "1", ... }
// Returns <dl><dt>a</dt><dd>1</dd>...
export function makeDefinitionList(data) {
    let list = document.createElement('dl');
    for (let key in data) {
        let dt = document.createElement('dt');
        dt.textContent = key;
        list.appendChild(dt);
        let dd = document.createElement('dd');
        dd.textContent = data[key];
        list.appendChild(dd);
    }
    return list;
}
