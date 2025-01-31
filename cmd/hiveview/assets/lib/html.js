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
export function makeButton(href, text, classes = "", attributes = "") {
    const button = document.createElement('a');
    button.href = href;
    button.className = 'btn ' + classes;
    button.innerHTML = text;
    if (attributes) {
        // Match attributes while preserving quoted values
        const attrRegex = /(\w+)=(['"])(.*?)\2/g;
        let match;
        while ((match = attrRegex.exec(attributes)) !== null) {
            const [_, name, quote, value] = match;
            button.setAttribute(name, value);
        }
    }
    return button;
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

// sanitizeHtml safely cleans HTML content by removing unsafe elements and attributes
export function sanitizeHtml(unsafeHtml, allowList) {
    if (!unsafeHtml || !unsafeHtml.length) {
        return unsafeHtml;
    }

    const parser = new DOMParser();
    const doc = parser.parseFromString(unsafeHtml, 'text/html');
    const elements = doc.body.querySelectorAll('*');

    elements.forEach(element => {
        const elementName = element.nodeName.toLowerCase();
        if (!Object.keys(allowList).includes(elementName)) {
            element.remove();
            return;
        }

        const allowedAttributes = [].concat(allowList['*'] || [], allowList[elementName] || []);
        Array.from(element.attributes).forEach(attr => {
            if (!allowedAttributes.includes(attr.name)) {
                element.removeAttribute(attr.name);
            }
        });
    });

    return doc.body.innerHTML;
}
