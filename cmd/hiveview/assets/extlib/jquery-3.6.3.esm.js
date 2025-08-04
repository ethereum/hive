// This is an ES module wrapper for jQuery.

import * as jq from './jquery-3.6.3.js';

// Importing jQuery as an ES module works a bit differently in browsers and esbuild. This
// is because esbuild recognizes jquery.js as a CommonJS module, and creates a named
// export called 'default' instead of the expected default export. In browsers, jQuery
// registers itself on window.
let $ = jq.default || window.jQuery;
export default $.noConflict();
