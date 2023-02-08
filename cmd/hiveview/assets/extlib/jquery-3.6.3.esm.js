// This is an ES module wrapper for jQuery.

import * as jq from './jquery-3.6.3.js';

export const $ = jq.ajax ? jq.default : (jq.$ || window.jQuery);
export const jQuery = $;
export default $;
