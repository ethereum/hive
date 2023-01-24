export let html = {
	// encode does HTML-encoding/escaping.
	encode: function(str) {
		//Let the DOM do it for us.
		var d = document.createElement('textarea');
		d.innerText = str;
		//Yes, I'm aware of
		// http://stackoverflow.com/questions/1219860/html-encoding-in-javascript-jquery
		// I just don't agree.
		return d.innerHTML;
	},

	// tag encapsulates data inside a tag
	tag: function(typ, str) {
		//Let the DOM do it for us.
		var d = document.createElement(typ);
			d.innerText = ("" + str);
		return d.outerHTML;
	},

	// HTML Attribute encoding
	attr_encode: function(str) {
		x = document.createElement("x");
		x.setAttribute("b", str);
		var all = x.outerHTML;
		return all.substring(6, all.length - 6);
	},

	get_link: function(url, text) {
		// get_link creates an anchor-element from 'untrusted' link data.
		var a = document.createElement('a');
		a.setAttribute("href", url);
		a.text = text;
		return a.outerHTML;
	},
	
	get_js_link: function(js, text) {
		var a = document.createElement('a');
		a.setAttribute("href", "javascript:" + js);
		a.text = text;
		return a.outerHTML;
	},

	// urls_to_links replaces URLs in input with HTML links.
	urls_to_links: function(str) {
		// Thanks, http://urlregex.com/
		let re = /(((?:http[s]?:\/\/)(?:[\-;:&=\+\$,\w]+@)?[A-Za-z0-9\.\-]+|(?:www\.|[\-;:&=\+\$,\w]+@)[A-Za-z0-9\.\-]+)((?:\/[\+~%\/\.\w\-_]*)?\??(?:[\-\+=&;%@\.\w_]*)#?(?:[\.\!\/\\\w]*))?)/;
		return String(str).replace(re, function (match) {
			return html.get_link(match, match);
		});
	},

	// get_button creates <button type="button" class="btn btn-default">Default</button>
	get_button: function(onclick, text) {
		var a = document.createElement('button');
		a.setAttribute("type", "button");
		a.setAttribute("class", "btn btn-primary btn-xs")
		a.textContent = text;
		a.setAttribute("onclick", onclick)
		return a.outerHTML;
	},

	// Takes { "a": "1", ... }
	// Returns <dl><dt>a</dt><dd>1</dd>...
	make_definition_list: function(data) {
		var list = document.createElement('dl');
		for (let key in data) {
			let dt = document.createElement('dt');
			dt.textContent = key;
			list.appendChild(dt);
			let dd = document.createElement('dd');
			dd.textContent = data[key];
			list.appendChild(dd);
		}
		return list;
	},
}

export let format = {
	// format_timespan gives the difference between times d1 and d2
	// in human readable time units.
	format_timespan: function(d1, d2) {
		var diff = d2 - d1;
		var _s = "";
		if (diff < 0) {
			_s = "-";
			diff = -diff;
		}
		var d = Math.floor(diff / 86400000);
		diff %= 86400000;
		var h = Math.floor(diff / 3600000);
		diff %= 3600000;
		var m = Math.floor(diff / 60000);
		diff %= 60000;
		var s = Math.floor(diff / 1000);

		var a = d ? (d + "d") : "";
		a += ((a || h) ? (h + "h") : "");
		a += ((a || m) ? (m + "m") : "") + s + "s";
		return _s + a;
	},

	// units returns human readable units for the given data size in bytes.
	units: function(loc) {
		if (loc < 1024) {
			return loc + "B"
		}
		loc = loc / 1024
		if (loc < 1024) {
			return loc.toFixed(2) + "KB";
		}
		loc = loc / 1024
		return loc.toFixed(2) + "MB";
	},
}

// nav is a little utility to store things in the url, so that people can link into stuff.
export let nav = {
	// get_hash_params loads parameters from the URL hash segment.
	get_hash_params: function() {
		var retval = {}
		var query = window.location.hash.substring(1);
		var vars = query.split('&');
		for (var i = 0; i < vars.length; i++) {
			var pair = vars[i].split('=');
			retval[decodeURIComponent(pair[0])] = decodeURIComponent(pair[1])
		}
		return retval;
	},
	
	// load returns the value of 'key' in the URL query.
	load: function(key) {
		if (!URLSearchParams) {
			console.error("Error: browser doesn't support URLSearchParams. IE or what?")
			return null
		}
		return new URLSearchParams(location.search).get(key);
	},

	// store stores the given keys and values in the URL query.
	store: function(keys) {
		let params = new URLSearchParams(location.search);
		for (let key in keys) {
			params.set(key, keys[key]);
		}
		let newsearch = "?" + params.toString();
		if (newsearch != location.search) {
			history.pushState(null, null, newsearch);
		}
	},
}
